package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/files"
	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

func (a *API) extractImportArchive(w http.ResponseWriter, r *http.Request) (string, func(), error) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxArchiveBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return "", func() {}, err
	}
	upload, _, err := r.FormFile("archive")
	if err != nil {
		return "", func() {}, err
	}
	defer upload.Close()

	zipFile, err := os.CreateTemp(a.TmpRoot, "import-*.zip")
	if err != nil {
		return "", func() {}, err
	}
	zipName := zipFile.Name()
	cleanup := func() { _ = os.Remove(zipName) }
	n, copyErr := io.Copy(zipFile, io.LimitReader(upload, a.MaxArchiveBytes+1))
	closeErr := zipFile.Close()
	if copyErr != nil || closeErr != nil || n > a.MaxArchiveBytes {
		cleanup()
		return "", func() {}, errors.New("archive exceeds configured limit")
	}

	expanded, err := os.MkdirTemp(a.TmpRoot, "import-tree-*")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	cleanupAll := func() {
		_ = os.Remove(zipName)
		_ = os.RemoveAll(expanded)
	}
	if err = files.ExtractZIP(zipName, expanded, files.ArchiveLimits{Files: a.MaxArchiveFiles, Bytes: a.MaxArchiveBytes}); err != nil {
		cleanupAll()
		return "", func() {}, err
	}
	packRoot, err := files.PackRoot(expanded)
	if err != nil {
		cleanupAll()
		return "", func() {}, err
	}
	return packRoot, cleanupAll, nil
}

func (a *API) importProject(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	packRoot, cleanup, err := a.extractImportArchive(w, r)
	if err != nil {
		bad(w, err)
		return
	}
	defer cleanup()

	if replaceID := strings.TrimSpace(r.FormValue("replace_project_id")); replaceID != "" {
		a.replaceProjectFromArchive(w, r, replaceID, packRoot)
		return
	}

	p := projects.Project{
		Slug:             r.FormValue("slug"),
		DisplayName:      r.FormValue("display_name"),
		MinecraftVersion: r.FormValue("minecraft_version"),
		Loader:           r.FormValue("loader"),
		LoaderVersion:    r.FormValue("loader_version"),
		PackVersion:      r.FormValue("pack_version"),
	}
	p, err = a.Projects.Import(r.Context(), p, packRoot)
	if err != nil {
		respond(w, p, err)
		return
	}

	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err == nil {
		err = a.syncImportedItems(r.Context(), tx, p.ID, p.WorkingDirectory)
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		_, _ = a.DB.ExecContext(r.Context(), `DELETE FROM projects WHERE id=?`, p.ID)
		_ = os.RemoveAll(p.WorkingDirectory)
		respond(w, nil, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	respond(w, p, nil)
}

func (a *API) replaceProjectFromArchive(w http.ResponseWriter, r *http.Request, projectID, packRoot string) {
	current, err := a.Projects.Get(r.Context(), projectID)
	if err != nil {
		respond(w, nil, err)
		return
	}

	updated := current
	if value := strings.TrimSpace(r.FormValue("display_name")); value != "" {
		updated.DisplayName = value
	}
	if value := strings.TrimSpace(r.FormValue("minecraft_version")); value != "" {
		updated.MinecraftVersion = value
	}
	if value := strings.TrimSpace(r.FormValue("loader")); value != "" {
		updated.Loader = value
	}
	if value := strings.TrimSpace(r.FormValue("loader_version")); value != "" {
		updated.LoaderVersion = value
	}
	if value := strings.TrimSpace(r.FormValue("pack_version")); value != "" {
		updated.PackVersion = value
	}

	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(r.Context(), `UPDATE projects SET display_name=?,minecraft_version=?,loader=?,loader_version=?,pack_version=?,updated_at=? WHERE id=?`, updated.DisplayName, updated.MinecraftVersion, updated.Loader, updated.LoaderVersion, updated.PackVersion, now, current.ID); err != nil {
		respond(w, nil, err)
		return
	}

	err = a.Projects.MutateAndCommit(r.Context(), current.ID, func(p projects.Project) error {
		if err := os.RemoveAll(p.WorkingDirectory); err != nil {
			return err
		}
		if err := os.CopyFS(p.WorkingDirectory, os.DirFS(packRoot)); err != nil {
			return err
		}
		args := append(packwiz.InitArgs(updated.DisplayName, updated.PackVersion, updated.MinecraftVersion, updated.Loader, updated.LoaderVersion), "--reinit")
		if err := a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, args...); err != nil {
			return err
		}
		if err := a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh"); err != nil {
			return err
		}
		return a.syncImportedItems(r.Context(), tx, current.ID, p.WorkingDirectory)
	}, tx.Commit)
	if err != nil {
		respond(w, nil, err)
		return
	}

	updated, err = a.Projects.Get(r.Context(), current.ID)
	respond(w, updated, err)
}

func (a *API) importURL(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.upload") {
		return
	}
	var in struct {
		URL         string `json:"url"`
		Target      string `json:"target_path"`
		DisplayName string `json:"display_name"`
		Kind        string `json:"kind"`
		Side        string `json:"side"`
	}
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	target, err := security.SafeRelative(in.Target)
	if err != nil || !allowedTarget(target) {
		bad(w, errors.New("target path rejected"))
		return
	}
	if in.Side != "client" && in.Side != "server" && in.Side != "both" {
		bad(w, errors.New("invalid side"))
		return
	}
	if !map[string]bool{"file": true, "config": true, "kubejs": true, "datapack": true, "resourcepack": true}[in.Kind] {
		bad(w, errors.New("invalid kind"))
		return
	}
	if destination, parseErr := url.Parse(strings.TrimSpace(in.URL)); parseErr == nil {
		slog.Info("remote file import", "destination", destination.Scheme+"://"+destination.Hostname(), "project", r.PathValue("id"))
	}
	content, err := files.FetchRemote(r.Context(), a.RemoteHTTP, strings.TrimSpace(in.URL), a.Blobs.MaxBytes)
	if err != nil {
		bad(w, err)
		return
	}
	id, digest, err := a.storeManagedFile(r.Context(), r.PathValue("id"), in.Kind, "url", in.DisplayName, target, in.Side, in.URL, content)
	respond(w, map[string]string{"id": id, "sha256": digest, "target_path": filepath.ToSlash(target)}, err)
}
