package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

const itemColumns = `id,project_id,kind,provider,provider_project_id,provider_version_id,display_name,target_path,filename,side,expected_sha256,metadata_json,enabled`

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	if view == "tree" {
		if !permission(w, r, "packwiz.read") {
			return
		}
		a.listFileTree(w, r, r.URL.Query().Get("path"))
		return
	}
	if view == "content" {
		a.readManagedFileContent(w, r, r.URL.Query().Get("path"))
		return
	}
	if view == "download" {
		a.downloadManagedFile(w, r, r.URL.Query().Get("path"))
		return
	}
	if !permission(w, r, "packwiz.read") {
		return
	}

	page, pageSize, offset := pageParams(r, 25, 100)
	where := []string{"project_id=?"}
	args := []any{r.PathValue("id")}

	if query := strings.TrimSpace(r.URL.Query().Get("q")); query != "" {
		where = append(where, `(display_name LIKE ? OR target_path LIKE ?)`)
		like := "%" + query + "%"
		args = append(args, like, like)
	}

	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" {
		if !map[string]bool{"modrinth": true, "curseforge": true, "custom": true, "local": true, "url": true, "client-file": true}[provider] {
			bad(w, errors.New("invalid provider filter"))
			return
		}
		where = append(where, "provider=?")
		args = append(args, provider)
	}

	if side := strings.TrimSpace(r.URL.Query().Get("side")); side != "" {
		if side != "client" && side != "server" && side != "both" {
			bad(w, errors.New("invalid side filter"))
			return
		}
		where = append(where, "side=?")
		args = append(args, side)
	}

	switch r.URL.Query().Get("group") {
	case "", "all":
	case "mods":
		where = append(where, `kind='mod' AND provider IN ('modrinth','curseforge')`)
	case "custom":
		where = append(where, `kind='mod' AND provider='custom'`)
	case "files":
		where = append(where, `kind<>'mod'`)
	default:
		bad(w, errors.New("invalid item group"))
		return
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := a.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM items WHERE `+whereSQL, args...).Scan(&total); err != nil {
		respond(w, nil, err)
		return
	}
	if total > 0 {
		pages := (total + pageSize - 1) / pageSize
		if page > pages {
			page = pages
			offset = (page - 1) * pageSize
		}
	}

	order := map[string]string{"name": "display_name", "provider": "provider", "path": "target_path"}[r.URL.Query().Get("sort")]
	if order == "" {
		order = "display_name"
	}
	direction := "ASC"
	if strings.EqualFold(r.URL.Query().Get("direction"), "desc") {
		direction = "DESC"
	}

	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := a.DB.QueryContext(r.Context(), `SELECT `+itemColumns+` FROM items WHERE `+whereSQL+` ORDER BY `+order+` `+direction+`, id ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer rows.Close()
	out := make([]item, 0, pageSize)
	for rows.Next() {
		v, err := scanItem(rows)
		if err != nil {
			respond(w, nil, err)
			return
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		respond(w, nil, err)
		return
	}
	respond(w, map[string]any{"items": out, "total": total, "page": page, "page_size": pageSize}, nil)
}

func (a *API) updateItemSide(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	var in struct {
		Side string `json:"side"`
	}
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	if in.Side != "client" && in.Side != "server" && in.Side != "both" {
		bad(w, errors.New("invalid side"))
		return
	}
	v, err := scanItem(a.DB.QueryRowContext(r.Context(), `SELECT `+itemColumns+` FROM items WHERE id=? AND project_id=?`, r.PathValue("item"), r.PathValue("id")))
	if err != nil {
		respond(w, nil, err)
		return
	}
	if v.Provider == "client-file" {
		bad(w, errors.New("client root files are always client-side"))
		return
	}
	if v.Provider == "local" || v.Provider == "url" {
		bad(w, errors.New("side applies only to Packwiz metadata entries"))
		return
	}
	meta := v.TargetPath
	if v.Provider == "custom" {
		meta = strings.TrimSuffix(meta, ".jar") + ".pw.toml"
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE items SET side=?,updated_at=? WHERE id=?`, in.Side, time.Now().UTC().Format(time.RFC3339Nano), v.ID); err != nil {
		respond(w, nil, err)
		return
	}
	err = a.Projects.MutateAndCommit(r.Context(), v.ProjectID, func(p projects.Project) error {
		if err := setMetadataSide(p.WorkingDirectory, meta, in.Side); err != nil {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	respond(w, map[string]bool{"ok": err == nil}, err)
}

func (a *API) removeItem(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	v, err := scanItem(a.DB.QueryRowContext(r.Context(), `SELECT `+itemColumns+` FROM items WHERE id=? AND project_id=?`, r.PathValue("item"), r.PathValue("id")))
	if err != nil {
		respond(w, nil, err)
		return
	}
	target := v.TargetPath
	if v.Provider == "custom" {
		target = strings.TrimSuffix(target, ".jar") + ".pw.toml"
	} else if v.Provider == "client-file" {
		filename, isClient, targetErr := clientFileName(v.TargetPath)
		if targetErr != nil || !isClient {
			if targetErr == nil {
				targetErr = errors.New("invalid client file target")
			}
			bad(w, targetErr)
			return
		}
		target = clientFileMetadataPath(filename)
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM items WHERE id=?`, v.ID); err != nil {
		respond(w, nil, err)
		return
	}
	err = a.Projects.MutateAndCommit(r.Context(), v.ProjectID, func(p projects.Project) error {
		name, err := security.SafeJoin(p.WorkingDirectory, target)
		if err != nil {
			return err
		}
		if err = os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	respond(w, map[string]bool{"ok": err == nil}, err)
}

func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	var in projects.Project
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	current, err := a.Projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	if in.DisplayName == "" {
		in.DisplayName = current.DisplayName
	}
	if in.MinecraftVersion == "" {
		in.MinecraftVersion = current.MinecraftVersion
	}
	if in.Loader == "" {
		in.Loader = current.Loader
	}
	if in.LoaderVersion == "" {
		in.LoaderVersion = current.LoaderVersion
	}
	if in.PackVersion == "" {
		in.PackVersion = current.PackVersion
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE projects SET display_name=?,minecraft_version=?,loader=?,loader_version=?,pack_version=?,updated_at=? WHERE id=?`, in.DisplayName, in.MinecraftVersion, in.Loader, in.LoaderVersion, in.PackVersion, time.Now().UTC().Format(time.RFC3339Nano), current.ID); err != nil {
		respond(w, nil, err)
		return
	}
	args := append(packwiz.InitArgs(in.DisplayName, in.PackVersion, in.MinecraftVersion, in.Loader, in.LoaderVersion), "--reinit")
	err = a.Projects.MutateAndCommit(r.Context(), current.ID, func(p projects.Project) error {
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, args...)
	}, tx.Commit)
	respond(w, map[string]bool{"ok": err == nil}, err)
}

func providerMetadataPath(root string, before map[string]bool, provider, projectID string) (string, error) {
	after, err := metadataFiles(root)
	if err != nil {
		return "", err
	}
	return newMetadata(root, before, after, provider, projectID)
}
