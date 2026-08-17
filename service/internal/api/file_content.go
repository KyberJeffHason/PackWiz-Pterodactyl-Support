package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

const editableFileMaxBytes int64 = 2 << 20

func (a *API) resolveManagedFile(r *http.Request, requested string) (string, string, os.FileInfo, error) {
	rel, err := normalizeManagedTreePath(requested)
	if err != nil {
		return "", "", nil, err
	}
	if rel == "" {
		return "", "", nil, errors.New("file path required")
	}
	if _, isClient, clientErr := clientFileName(rel); isClient {
		if clientErr != nil {
			return "", "", nil, clientErr
		}
		name, err := a.clientFileBlobPath(r.Context(), r.PathValue("id"), rel)
		if err != nil {
			return "", "", nil, err
		}
		info, err := os.Lstat(name)
		if err != nil {
			return "", "", nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", "", nil, errors.New("path is not a regular managed file")
		}
		return rel, name, info, nil
	}
	project, err := a.Projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return "", "", nil, err
	}
	name, err := security.SafeJoin(project.WorkingDirectory, rel)
	if err != nil {
		return "", "", nil, err
	}
	info, err := os.Lstat(name)
	if err != nil {
		return "", "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", nil, errors.New("path is not a regular managed file")
	}
	return rel, name, info, nil
}

func (a *API) readManagedFileContent(w http.ResponseWriter, r *http.Request, requested string) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	rel, name, info, err := a.resolveManagedFile(r, requested)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if info.Size() > editableFileMaxBytes {
		bad(w, fmt.Errorf("file is too large to edit in the browser (maximum %d bytes)", editableFileMaxBytes))
		return
	}
	file, err := os.Open(name)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, editableFileMaxBytes+1))
	if err != nil {
		respond(w, nil, err)
		return
	}
	if int64(len(content)) > editableFileMaxBytes {
		bad(w, fmt.Errorf("file is too large to edit in the browser (maximum %d bytes)", editableFileMaxBytes))
		return
	}
	if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
		bad(w, errors.New("binary files cannot be edited in the browser"))
		return
	}
	respond(w, map[string]any{
		"path":        rel,
		"content":     string(content),
		"size":        len(content),
		"modified_at": info.ModTime().UTC().Format(http.TimeFormat),
	}, nil)
}

func (a *API) downloadManagedFile(w http.ResponseWriter, r *http.Request, requested string) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	rel, name, info, err := a.resolveManagedFile(r, requested)
	if err != nil {
		respond(w, nil, err)
		return
	}
	file, err := os.Open(name)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer file.Close()
	filename := filepath.Base(rel)
	if disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filename, info.ModTime(), file)
}
