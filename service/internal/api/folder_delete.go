package api

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

func normalizeManagedFolderDeletePath(value string) (string, error) {
	rel, err := normalizeManagedTreePath(value)
	if err != nil {
		return "", err
	}
	if rel == "" {
		return "", errors.New("folder path required")
	}
	if rel == clientFilesRoot || strings.HasPrefix(rel, clientFilesRoot+"/") {
		return "", errors.New("client-files does not support folders")
	}
	for _, root := range managedFileRoots {
		if rel == root {
			return "", errors.New("managed root directories cannot be deleted")
		}
	}
	return rel, nil
}

func (a *API) removeFolder(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	rel, err := normalizeManagedFolderDeletePath(r.URL.Query().Get("path"))
	if err != nil {
		bad(w, err)
		return
	}
	project, err := a.Projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	name, err := security.SafeJoin(project.WorkingDirectory, rel)
	if err != nil {
		bad(w, err)
		return
	}
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		bad(w, errors.New("directory does not exist"))
		return
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		bad(w, errors.New("path is not a managed directory"))
		return
	}

	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	prefix := rel + "/"
	result, err := tx.ExecContext(r.Context(), `DELETE FROM items WHERE project_id=? AND kind<>'mod' AND (target_path=? OR substr(target_path,1,?)=?)`, project.ID, rel, len(prefix), prefix)
	if err != nil {
		respond(w, nil, err)
		return
	}
	removedItems, _ := result.RowsAffected()

	err = a.Projects.MutateAndCommit(r.Context(), project.ID, func(p projects.Project) error {
		target, err := security.SafeJoin(p.WorkingDirectory, rel)
		if err != nil {
			return err
		}
		current, err := os.Lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("directory does not exist")
		}
		if err != nil {
			return err
		}
		if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() {
			return errors.New("path is not a managed directory")
		}
		if err = os.RemoveAll(target); err != nil {
			return err
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	respond(w, map[string]any{"ok": err == nil, "path": rel, "removed_items": removedItems}, err)
}
