package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/files"
	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/providers/curseforge"
	"github.com/packwiz-manager/packwiz-manager/service/internal/providers/modrinth"
	"github.com/packwiz-manager/packwiz-manager/service/internal/publishing"
	"github.com/packwiz-manager/packwiz-manager/service/internal/revisions"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

type API struct {
	Projects        *projects.Manager
	Publisher       *publishing.Publisher
	Blobs           files.Store
	DB              *sql.DB
	Modrinth        *modrinth.Client
	CurseForge      *curseforge.Client
	PublicBaseURL   string
	TmpRoot         string
	RemoteHTTP      *http.Client
	MaxArchiveBytes int64
	MaxArchiveFiles int
}
type rateWindow struct {
	at    time.Time
	count int
}

var mutationRates = struct {
	sync.Mutex
	values map[string]rateWindow
}{values: map[string]rateWindow{}}

func (a *API) Routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /projects", a.listProjects)
	m.HandleFunc("GET /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !permission(w, r, "packwiz.read") {
			return
		}
		v, err := a.Projects.Get(r.Context(), r.PathValue("id"))
		respond(w, v, err)
	})
	m.HandleFunc("POST /projects", a.createProject)
	m.HandleFunc("PATCH /projects/{id}", a.updateProject)
	m.HandleFunc("POST /projects/import", a.importProject)
	m.HandleFunc("POST /projects/{id}/custom-jars", a.uploadJAR)
	m.HandleFunc("POST /projects/{id}/files", a.uploadFile)
	m.HandleFunc("DELETE /projects/{id}/folders", a.removeFolder)
	m.HandleFunc("POST /projects/{id}/url-imports", a.importURL)
	m.HandleFunc("POST /projects/{id}/mods", a.addMod)
	m.HandleFunc("GET /projects/{id}/items", a.listItems)
	m.HandleFunc("PATCH /projects/{id}/items/{item}/side", a.updateItemSide)
	m.HandleFunc("DELETE /projects/{id}/items/{item}", a.removeItem)
	m.HandleFunc("POST /projects/{id}/publish", a.publish)
	m.HandleFunc("GET /projects/{id}/revisions", a.listRevisions)
	m.HandleFunc("GET /projects/{id}/revisions/diff", a.diffRevisions)
	m.HandleFunc("POST /projects/{id}/rollback/{revision}", a.rollback)
	m.HandleFunc("GET /projects/{id}/server-links/{server}", a.getServerLink)
	m.HandleFunc("PUT /projects/{id}/server-links/{server}", a.putServerLink)
	m.HandleFunc("GET /providers/modrinth/search", a.searchModrinth)
	m.HandleFunc("GET /providers/curseforge/search", a.searchCurseForge)
	return requestID(rateMutations(recoverJSON(m)))
}
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	v, err := a.Projects.List(r.Context())
	respond(w, v, err)
}
func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	var p projects.Project
	if err := decode(r, &p); err != nil {
		bad(w, err)
		return
	}
	p, err := a.Projects.Create(r.Context(), p)
	if err == nil {
		w.WriteHeader(201)
	}
	respond(w, p, err)
}
func (a *API) uploadJAR(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.upload") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.Blobs.MaxBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		bad(w, err)
		return
	}
	project, err := a.Projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	side := r.FormValue("side")
	if side != "client" && side != "server" && side != "both" {
		bad(w, errors.New("side must be client, server, or both"))
		return
	}
	display := strings.TrimSpace(r.FormValue("display_name"))
	if display == "" {
		bad(w, errors.New("display_name required"))
		return
	}
	destination, err := security.SafeRelative(r.FormValue("destination"))
	if err != nil || path.Dir(destination) != "mods" || !strings.HasSuffix(strings.ToLower(destination), ".jar") {
		bad(w, errors.New("custom JAR destination must be directly under mods/"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		bad(w, err)
		return
	}
	defer file.Close()
	safe, err := security.SafeFilename(header.Filename)
	if err != nil {
		bad(w, err)
		return
	}
	blob, err := a.Blobs.Put(file, safe, "application/java-archive", true)
	if err != nil {
		bad(w, err)
		return
	}
	metaRel := strings.TrimSuffix(destination, ".jar") + ".pw.toml"
	destFilename, err := security.SafeFilename(path.Base(destination))
	if err != nil {
		bad(w, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	itemID := projects.ID()
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO blobs(sha256,byte_size,mime_type,storage_path,original_filename,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(sha256) DO NOTHING`, blob.SHA256, blob.Size, blob.MIME, blob.Path, blob.Filename, now); err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO items(id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,blob_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, itemID, project.ID, "mod", "custom", display, destination, destFilename, side, blob.SHA256, blob.SHA256, now, now)
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	err = a.Projects.MutateAndCommit(r.Context(), project.ID, func(project projects.Project) error {
		metaPath := filepath.Join(project.WorkingDirectory, filepath.FromSlash(metaRel))
		if err := os.MkdirAll(filepath.Dir(metaPath), 0750); err != nil {
			return err
		}
		content := fmt.Sprintf("name = %q\nfilename = %q\nside = %q\n\n[download]\nurl = %q\nhash-format = \"sha256\"\nhash = %q\n", display, destFilename, side, a.PublicBaseURL+"/blobs/sha256/"+blob.SHA256+"/"+destFilename, blob.SHA256)
		tmp := metaPath + ".tmp"
		if err := os.WriteFile(tmp, []byte(content), 0640); err != nil {
			return err
		}
		if err := os.Rename(tmp, metaPath); err != nil {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), project.WorkingDirectory, "refresh")
	}, tx.Commit)
	if err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, map[string]any{"id": itemID, "sha256": blob.SHA256, "filename": destFilename}, err)
}
func (a *API) addMod(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	var in struct {
		Provider    string `json:"provider"`
		ProjectID   string `json:"project_id"`
		VersionID   string `json:"version_id"`
		DisplayName string `json:"display_name"`
		Side        string `json:"side"`
		IconURL     string `json:"icon_url"`
		Author      string `json:"author"`
	}
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	if in.Side != "client" && in.Side != "server" && in.Side != "both" {
		bad(w, errors.New("invalid side"))
		return
	}
	if !publicPart.MatchString(in.ProjectID) || (in.VersionID != "" && !publicPart.MatchString(in.VersionID)) {
		bad(w, errors.New("invalid provider identifiers"))
		return
	}
	in.IconURL = strings.TrimSpace(in.IconURL)
	in.Author = strings.TrimSpace(in.Author)
	if len(in.IconURL) > 2048 || (in.IconURL != "" && !strings.HasPrefix(strings.ToLower(in.IconURL), "https://")) {
		bad(w, errors.New("icon_url must be an HTTPS URL"))
		return
	}
	if len(in.Author) > 200 {
		bad(w, errors.New("author is too long"))
		return
	}
	meta := map[string]string{}
	if in.IconURL != "" {
		meta["icon_url"] = in.IconURL
	}
	if in.Author != "" {
		meta["author"] = in.Author
	}
	metadata, err := json.Marshal(meta)
	if err != nil {
		respond(w, nil, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := projects.ID()
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO items(id,project_id,kind,provider,provider_project_id,provider_version_id,display_name,target_path,filename,side,metadata_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, r.PathValue("id"), "mod", in.Provider, in.ProjectID, in.VersionID, in.DisplayName, "mods", "", in.Side, string(metadata), now, now); err != nil {
		respond(w, nil, err)
		return
	}
	err = a.Projects.MutateAndCommit(r.Context(), r.PathValue("id"), func(p projects.Project) error {
		before, err := metadataFiles(p.WorkingDirectory)
		if err != nil {
			return err
		}
		var args []string
		switch in.Provider {
		case "modrinth":
			args = packwiz.ModrinthAddArgs(in.ProjectID, in.VersionID)
		case "curseforge":
			args = packwiz.CurseForgeAddArgs(in.ProjectID, in.VersionID)
		default:
			return errors.New("unsupported provider")
		}
		if err = a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, args...); err != nil {
			return err
		}
		metaPath, err := providerMetadataPath(p.WorkingDirectory, before, in.Provider, in.ProjectID)
		if err != nil {
			return err
		}
		if err = setMetadataSide(p.WorkingDirectory, metaPath, in.Side); err != nil {
			return err
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE items SET target_path=?,filename=? WHERE id=?`, metaPath, path.Base(metaPath), id); err != nil {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	if err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, map[string]string{"id": id}, err)
}
func (a *API) uploadFile(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.upload") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.Blobs.MaxBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		bad(w, err)
		return
	}
	target, err := security.SafeRelative(r.FormValue("target_path"))
	if err != nil {
		bad(w, err)
		return
	}
	clientName, isClientFile, clientErr := clientFileName(target)
	if clientErr != nil {
		bad(w, clientErr)
		return
	}
	if !isClientFile && !allowedTarget(target) {
		bad(w, errors.New("target path is outside configured allowlist"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		bad(w, err)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, a.Blobs.MaxBytes+1))
	if err != nil || int64(len(content)) > a.Blobs.MaxBytes {
		bad(w, errors.New("file exceeds configured limit"))
		return
	}
	if isClientFile {
		id, digest, storeErr := a.storeClientFile(r.Context(), r.PathValue("id"), target, clientName, content)
		respond(w, map[string]any{"id": id, "target_path": target, "filename": clientName, "sha256": digest}, storeErr)
		return
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := projects.ID()
	kind := "file"
	side := "both"
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()

	var existingID, existingKind, existingSide string
	err = tx.QueryRowContext(r.Context(), `SELECT id,kind,side FROM items WHERE project_id=? AND target_path=? AND kind<>'mod' ORDER BY created_at ASC LIMIT 1`, r.PathValue("id"), target).Scan(&existingID, &existingKind, &existingSide)
	switch {
	case err == nil:
		id = existingID
		if existingKind != "" {
			kind = existingKind
		}
		if existingSide == "client" || existingSide == "server" || existingSide == "both" {
			side = existingSide
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM items WHERE project_id=? AND target_path=? AND kind<>'mod' AND id<>?`, r.PathValue("id"), target, id); err != nil {
			respond(w, nil, err)
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE items SET kind=?,provider='local',display_name=?,filename=?,side=?,expected_sha256=?,source_url=NULL,updated_at=? WHERE id=?`, kind, path.Base(target), path.Base(target), side, digest, now, id)
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(r.Context(), `INSERT INTO items(id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, r.PathValue("id"), kind, "local", path.Base(target), target, path.Base(target), side, digest, now, now)
	default:
		respond(w, nil, err)
		return
	}
	if err != nil {
		respond(w, nil, err)
		return
	}

	err = a.Projects.MutateAndCommit(r.Context(), r.PathValue("id"), func(p projects.Project) error {
		name, err := security.SafeJoin(p.WorkingDirectory, target)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(name), 0750); err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(name), ".managed-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, err = tmp.Write(content); err == nil {
			err = tmp.Chmod(0640)
		}
		if closeErr := tmp.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmpName, name)
		}
		if err != nil {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	respond(w, map[string]any{"id": id, "target_path": target, "filename": header.Filename, "sha256": digest}, err)
}
func (a *API) publish(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.publish") {
		return
	}
	var in struct {
		Changelog string `json:"changelog"`
	}
	_ = decode(r, &in)
	v, err := a.Publisher.Publish(r.Context(), r.PathValue("id"), r.Header.Get("X-Pterodactyl-Actor"), in.Changelog)
	respond(w, v, err)
}
func (a *API) listRevisions(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,revision_number,pack_version,content_digest,changelog,created_by,created_at FROM revisions WHERE project_id=? ORDER BY revision_number DESC`, r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, version, digest, changelog, created string
		var actor sql.NullString
		var number int64
		if err = rows.Scan(&id, &number, &version, &digest, &changelog, &actor, &created); err != nil {
			respond(w, nil, err)
			return
		}
		out = append(out, map[string]any{"id": id, "revision": number, "pack_version": version, "content_digest": digest, "changelog": changelog, "actor": actor.String, "created_at": created})
	}
	respond(w, out, rows.Err())
}
func (a *API) diffRevisions(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	load := func(number string) (revisions.Manifest, error) {
		var dir string
		if _, err := strconv.ParseInt(number, 10, 64); err != nil {
			return revisions.Manifest{}, err
		}
		if err := a.DB.QueryRowContext(r.Context(), `SELECT release_directory FROM revisions WHERE project_id=? AND revision_number=?`, r.PathValue("id"), number).Scan(&dir); err != nil {
			return revisions.Manifest{}, err
		}
		raw, err := os.ReadFile(filepath.Join(dir, "revision.json"))
		if err != nil {
			return revisions.Manifest{}, err
		}
		var m revisions.Manifest
		err = json.Unmarshal(raw, &m)
		return m, err
	}
	before, err := load(from)
	if err != nil {
		respond(w, nil, err)
		return
	}
	after, err := load(to)
	if err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, revisions.Compare(before, after), nil)
}
func (a *API) rollback(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.publish") {
		return
	}
	n, err := strconv.ParseInt(r.PathValue("revision"), 10, 64)
	if err == nil {
		err = a.Publisher.Rollback(r.Context(), r.PathValue("id"), n, r.Header.Get("X-Pterodactyl-Actor"))
	}
	respond(w, map[string]bool{"ok": err == nil}, err)
}
func (a *API) searchModrinth(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	if projectID := strings.TrimSpace(r.URL.Query().Get("versions_for")); projectID != "" {
		if !publicPart.MatchString(projectID) {
			bad(w, errors.New("invalid provider project id"))
			return
		}
		versions, err := a.Modrinth.Versions(r.Context(), projectID, r.URL.Query().Get("minecraft"), r.URL.Query().Get("loader"))
		if err != nil {
			respond(w, nil, err)
			return
		}
		items := make([]map[string]any, 0, len(versions))
		for _, version := range versions {
			items = append(items, map[string]any{
				"id":             version.ID,
				"name":           version.Name,
				"version_number": version.VersionNumber,
				"channel":        version.VersionType,
				"published":      version.DatePublished,
			})
		}
		respond(w, map[string]any{"items": items, "total": len(items), "page": 1, "page_size": len(items)}, nil)
		return
	}
	page, pageSize, offset := pageParams(r, 20, 100)
	result, err := a.Modrinth.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("minecraft"), r.URL.Query().Get("loader"), pageSize, offset)
	if err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, map[string]any{"items": result.Hits, "total": result.Total, "page": page, "page_size": pageSize}, nil)
}
func (a *API) searchCurseForge(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	if projectID := strings.TrimSpace(r.URL.Query().Get("versions_for")); projectID != "" {
		if !publicPart.MatchString(projectID) {
			bad(w, errors.New("invalid provider project id"))
			return
		}
		result, err := a.CurseForge.Files(r.Context(), projectID, r.URL.Query().Get("minecraft"), r.URL.Query().Get("loader"), 50, 0)
		if err != nil {
			respond(w, nil, err)
			return
		}
		allFiles := append([]curseforge.File(nil), result.Files...)
		for len(allFiles) < result.Total && len(result.Files) > 0 {
			result, err = a.CurseForge.Files(r.Context(), projectID, r.URL.Query().Get("minecraft"), r.URL.Query().Get("loader"), 50, len(allFiles))
			if err != nil {
				respond(w, nil, err)
				return
			}
			allFiles = append(allFiles, result.Files...)
		}
		items := make([]map[string]any, 0, len(allFiles))
		for _, file := range allFiles {
			channel := "unknown"
			switch file.ReleaseType {
			case 1:
				channel = "release"
			case 2:
				channel = "beta"
			case 3:
				channel = "alpha"
			}
			items = append(items, map[string]any{
				"id":             strconv.Itoa(file.ID),
				"name":           file.DisplayName,
				"version_number": file.DisplayName,
				"channel":        channel,
				"published":      file.FileDate,
				"filename":       file.FileName,
			})
		}
		respond(w, map[string]any{"items": items, "total": len(items), "page": 1, "page_size": len(items)}, nil)
		return
	}
	page, pageSize, offset := pageParams(r, 20, 50)
	result, err := a.CurseForge.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("minecraft"), r.URL.Query().Get("loader"), 6, pageSize, offset)
	if err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, map[string]any{"items": result.Mods, "total": result.Total, "page": page, "page_size": pageSize}, nil)
}
func pageParams(r *http.Request, defaultSize, maxSize int) (page, size, offset int) {
	page = 1
	size = defaultSize
	if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 0 {
		page = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && n > 0 {
		size = n
	}
	if size > maxSize {
		size = maxSize
	}
	if size < 1 {
		size = defaultSize
	}
	offset = (page - 1) * size
	return
}
func permission(w http.ResponseWriter, r *http.Request, want string) bool {
	for _, p := range strings.Split(r.Header.Get("X-Packwiz-Permissions"), ",") {
		if strings.TrimSpace(p) == want {
			return true
		}
	}
	w.WriteHeader(403)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "permission denied"})
	return false
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func bad(w http.ResponseWriter, err error) {
	w.WriteHeader(400)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
func respond(w http.ResponseWriter, v any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		code := 500
		if errors.Is(err, sql.ErrNoRows) {
			code = 404
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = projects.ID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
func recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				respond(w, nil, errors.New("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func rateMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-Pterodactyl-Actor")
		if key == "" {
			key = "unknown"
		}
		now := time.Now()
		mutationRates.Lock()
		v := mutationRates.values[key]
		if now.Sub(v.at) >= time.Minute {
			v = rateWindow{at: now}
		}
		v.count++
		mutationRates.values[key] = v
		mutationRates.Unlock()
		if v.count > 30 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "mutation rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func Shutdown(ctx context.Context, servers ...*http.Server) error {
	var result error
	for _, s := range servers {
		if err := s.Shutdown(ctx); err != nil {
			result = err
		}
	}
	return result
}
