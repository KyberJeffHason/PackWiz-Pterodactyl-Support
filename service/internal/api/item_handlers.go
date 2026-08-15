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

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,enabled FROM items WHERE project_id=? ORDER BY display_name`, r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer rows.Close()
	var out []item
	for rows.Next() {
		v, err := scanItem(rows)
		if err != nil {
			respond(w, nil, err)
			return
		}
		out = append(out, v)
	}
	respond(w, out, rows.Err())
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
	v, err := scanItem(a.DB.QueryRowContext(r.Context(), `SELECT id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,enabled FROM items WHERE id=? AND project_id=?`, r.PathValue("item"), r.PathValue("id")))
	if err != nil {
		respond(w, nil, err)
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
	v, err := scanItem(a.DB.QueryRowContext(r.Context(), `SELECT id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,enabled FROM items WHERE id=? AND project_id=?`, r.PathValue("item"), r.PathValue("id")))
	if err != nil {
		respond(w, nil, err)
		return
	}
	target := v.TargetPath
	if v.Provider == "custom" {
		target = strings.TrimSuffix(target, ".jar") + ".pw.toml"
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
