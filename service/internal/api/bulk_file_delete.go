package api

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

const maxBulkFileDeletes = 500

func normalizeBulkFileDeleteIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, errors.New("at least one file id is required")
	}
	if len(ids) > maxBulkFileDeletes {
		return nil, errors.New("too many files in one delete request")
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 128 {
			return nil, errors.New("invalid file id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func managedFileRemovalTarget(v item) (string, error) {
	if v.Kind == "mod" {
		return "", errors.New("bulk file deletion cannot remove mods")
	}
	if v.Provider == "client-file" {
		filename, isClient, err := clientFileName(v.TargetPath)
		if err != nil || !isClient {
			if err == nil {
				err = errors.New("invalid client file target")
			}
			return "", err
		}
		return clientFileMetadataPath(filename), nil
	}
	if !allowedTarget(v.TargetPath) {
		return "", errors.New("file target is outside managed roots")
	}
	return v.TargetPath, nil
}

func sqlPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func (a *API) removeFileItems(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	ids, err := normalizeBulkFileDeleteIDs(in.IDs)
	if err != nil {
		bad(w, err)
		return
	}

	projectID := r.PathValue("id")
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()

	args := make([]any, 0, len(ids)+1)
	args = append(args, projectID)
	for _, id := range ids {
		args = append(args, id)
	}
	query := `SELECT ` + itemColumns + ` FROM items WHERE project_id=? AND id IN (` + sqlPlaceholders(len(ids)) + `)`
	rows, err := tx.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond(w, nil, err)
		return
	}
	items := make([]item, 0, len(ids))
	for rows.Next() {
		v, scanErr := scanItem(rows)
		if scanErr != nil {
			rows.Close()
			respond(w, nil, scanErr)
			return
		}
		items = append(items, v)
	}
	if err = rows.Close(); err != nil {
		respond(w, nil, err)
		return
	}
	if err = rows.Err(); err != nil {
		respond(w, nil, err)
		return
	}
	if len(items) != len(ids) {
		bad(w, errors.New("one or more files no longer exist"))
		return
	}

	targets := make([]string, 0, len(items))
	for _, v := range items {
		target, targetErr := managedFileRemovalTarget(v)
		if targetErr != nil {
			bad(w, targetErr)
			return
		}
		targets = append(targets, target)
	}

	deleteQuery := `DELETE FROM items WHERE project_id=? AND kind<>'mod' AND id IN (` + sqlPlaceholders(len(ids)) + `)`
	result, err := tx.ExecContext(r.Context(), deleteQuery, args...)
	if err != nil {
		respond(w, nil, err)
		return
	}
	removed, err := result.RowsAffected()
	if err != nil {
		respond(w, nil, err)
		return
	}
	if removed != int64(len(ids)) {
		bad(w, errors.New("one or more selected entries are not managed files"))
		return
	}

	err = a.Projects.MutateAndCommit(r.Context(), projectID, func(p projects.Project) error {
		for _, target := range targets {
			name, joinErr := security.SafeJoin(p.WorkingDirectory, target)
			if joinErr != nil {
				return joinErr
			}
			if removeErr := os.Remove(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
		}
		return a.Projects.Packwiz.Run(r.Context(), p.WorkingDirectory, "refresh")
	}, tx.Commit)
	respond(w, map[string]any{"ok": err == nil, "removed": removed}, err)
}
