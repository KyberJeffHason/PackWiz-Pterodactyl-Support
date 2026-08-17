package api

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
)

const clientFilesRoot = "client-files"

var clientFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)

func clientFileName(target string) (string, bool, error) {
	if target == clientFilesRoot {
		return "", true, errors.New("client file name required")
	}
	if !strings.HasPrefix(target, clientFilesRoot+"/") {
		return "", false, nil
	}
	name := strings.TrimPrefix(target, clientFilesRoot+"/")
	if name == "" || strings.Contains(name, "/") || !clientFileNamePattern.MatchString(name) {
		return "", true, errors.New("client files must be direct root files using letters, numbers, '.', '_', '+', or '-'")
	}
	lower := strings.ToLower(name)
	if lower == "pack.toml" || lower == "index.toml" || lower == ".packwizignore" || strings.HasSuffix(lower, ".pw.toml") {
		return "", true, errors.New("client file name conflicts with Packwiz metadata")
	}
	return name, true, nil
}

func clientFileMetadataPath(filename string) string {
	return filename + ".pw.toml"
}

func (a *API) storeClientFile(ctx context.Context, projectID, target, filename string, content []byte) (string, string, error) {
	validated, isClient, err := clientFileName(target)
	if err != nil || !isClient || validated != filename {
		if err == nil {
			err = errors.New("invalid client file target")
		}
		return "", "", err
	}

	blob, err := a.Blobs.Put(bytes.NewReader(content), filename, "application/octet-stream", false)
	if err != nil {
		return "", "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `INSERT INTO blobs(sha256,byte_size,mime_type,storage_path,original_filename,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(sha256) DO NOTHING`, blob.SHA256, blob.Size, blob.MIME, blob.Path, blob.Filename, now); err != nil {
		return "", "", err
	}

	id := projects.ID()
	existing := false
	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM items WHERE project_id=? AND target_path=? AND provider='client-file' ORDER BY created_at ASC LIMIT 1`, projectID, target).Scan(&existingID)
	switch {
	case err == nil:
		id = existingID
		existing = true
		_, err = tx.ExecContext(ctx, `UPDATE items SET kind='file',provider='client-file',display_name=?,filename=?,side='client',expected_sha256=?,source_url=NULL,blob_id=?,updated_at=? WHERE id=?`, filename, filename, blob.SHA256, blob.SHA256, now, id)
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `INSERT INTO items(id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,blob_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, projectID, "file", "client-file", filename, target, filename, "client", blob.SHA256, blob.SHA256, now, now)
	default:
		return "", "", err
	}
	if err != nil {
		return "", "", err
	}

	err = a.Projects.MutateAndCommit(ctx, projectID, func(project projects.Project) error {
		rootTarget := filepath.Join(project.WorkingDirectory, filename)
		if info, statErr := os.Lstat(rootTarget); statErr == nil && info != nil {
			return fmt.Errorf("root file %q already exists; remove it before managing it through %s", filename, clientFilesRoot)
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}

		metaPath := filepath.Join(project.WorkingDirectory, clientFileMetadataPath(filename))
		if !existing {
			if _, statErr := os.Lstat(metaPath); statErr == nil {
				return fmt.Errorf("metadata file %q already exists", filepath.Base(metaPath))
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return statErr
			}
		}
		downloadURL := strings.TrimRight(a.PublicBaseURL, "/") + "/blobs/sha256/" + blob.SHA256 + "/" + url.PathEscape(filename)
		metadata := fmt.Sprintf("name = %q\nfilename = %q\nside = \"client\"\n\n[download]\nurl = %q\nhash-format = \"sha256\"\nhash = %q\n", filename, filename, downloadURL, blob.SHA256)
		tmp := metaPath + ".tmp"
		if err := os.WriteFile(tmp, []byte(metadata), 0640); err != nil {
			return err
		}
		if err := os.Rename(tmp, metaPath); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return a.Projects.Packwiz.Run(ctx, project.WorkingDirectory, "refresh")
	}, tx.Commit)
	return id, blob.SHA256, err
}

func (a *API) clientFileBlobPath(ctx context.Context, projectID, target string) (string, error) {
	var storagePath string
	err := a.DB.QueryRowContext(ctx, `SELECT b.storage_path FROM items i JOIN blobs b ON b.sha256=i.blob_id WHERE i.project_id=? AND i.target_path=? AND i.provider='client-file'`, projectID, target).Scan(&storagePath)
	return storagePath, err
}

func (a *API) clientFileCount(ctx context.Context, projectID string) (int, error) {
	var count int
	err := a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE project_id=? AND provider='client-file'`, projectID).Scan(&count)
	return count, err
}

func (a *API) clientFileTree(ctx context.Context, projectID string) ([]fileTreeEntry, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT i.id,i.filename,i.target_path,i.side,COALESCE(i.expected_sha256,''),COALESCE(b.byte_size,0),i.updated_at FROM items i LEFT JOIN blobs b ON b.sha256=i.blob_id WHERE i.project_id=? AND i.provider='client-file' ORDER BY i.filename`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []fileTreeEntry{}
	for rows.Next() {
		var id, filename, target, side, sha, updated string
		var size int64
		if err := rows.Scan(&id, &filename, &target, &side, &sha, &size, &updated); err != nil {
			return nil, err
		}
		out = append(out, fileTreeEntry{
			Name:       filename,
			Path:       target,
			Type:       "file",
			Exists:     true,
			Size:       size,
			ModifiedAt: updated,
			ItemID:     id,
			Kind:       "file",
			Provider:   "client-file",
			Side:       side,
			SHA256:     sha,
		})
	}
	return out, rows.Err()
}
