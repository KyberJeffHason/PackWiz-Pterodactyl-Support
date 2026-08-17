package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

var managedFileRoots = []string{"config", "defaultconfigs", "kubejs", "datapacks", "resourcepacks"}

type fileTreeManagedItem struct {
	ID       string
	Kind     string
	Provider string
	Side     string
	SHA256   string
}

type fileTreeEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Exists     bool   `json:"exists"`
	Children   int    `json:"children"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Side       string `json:"side,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

func normalizeManagedTreePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	rel, err := security.SafeRelative(value)
	if err != nil {
		return "", err
	}
	for _, root := range managedFileRoots {
		if rel == root || strings.HasPrefix(rel, root+"/") {
			return rel, nil
		}
	}
	return "", errors.New("path is outside managed file roots")
}

func visibleDirectoryCount(name string) int {
	rows, err := os.ReadDir(name)
	if err != nil {
		return 0
	}
	count := 0
	for _, row := range rows {
		if row.Type()&os.ModeSymlink == 0 {
			count++
		}
	}
	return count
}

func (a *API) listFileTree(w http.ResponseWriter, r *http.Request, requested string) {
	rel, err := normalizeManagedTreePath(requested)
	if err != nil {
		bad(w, err)
		return
	}
	project, err := a.Projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}

	managed := map[string]fileTreeManagedItem{}
	rows, err := a.DB.QueryContext(r.Context(), `SELECT id,kind,provider,target_path,side,COALESCE(expected_sha256,'') FROM items WHERE project_id=? AND kind<>'mod'`, project.ID)
	if err != nil {
		respond(w, nil, err)
		return
	}
	for rows.Next() {
		var id, kind, provider, target, side, sha string
		if err = rows.Scan(&id, &kind, &provider, &target, &side, &sha); err != nil {
			rows.Close()
			respond(w, nil, err)
			return
		}
		managed[filepath.ToSlash(target)] = fileTreeManagedItem{ID: id, Kind: kind, Provider: provider, Side: side, SHA256: sha}
	}
	if err = rows.Close(); err != nil {
		respond(w, nil, err)
		return
	}
	if err = rows.Err(); err != nil {
		respond(w, nil, err)
		return
	}

	if rel == "" {
		out := make([]fileTreeEntry, 0, len(managedFileRoots))
		for _, root := range managedFileRoots {
			name := filepath.Join(project.WorkingDirectory, root)
			entry := fileTreeEntry{Name: root, Path: root, Type: "directory"}
			info, statErr := os.Lstat(name)
			switch {
			case statErr == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir():
				entry.Exists = true
				entry.Children = visibleDirectoryCount(name)
				entry.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
			case statErr != nil && !errors.Is(statErr, os.ErrNotExist):
				respond(w, nil, statErr)
				return
			}
			out = append(out, entry)
		}
		respond(w, map[string]any{"path": "", "entries": out}, nil)
		return
	}

	name, err := security.SafeJoin(project.WorkingDirectory, rel)
	if err != nil {
		bad(w, err)
		return
	}
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		for _, root := range managedFileRoots {
			if rel == root {
				respond(w, map[string]any{"path": rel, "entries": []fileTreeEntry{}}, nil)
				return
			}
		}
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

	dir, err := os.ReadDir(name)
	if err != nil {
		respond(w, nil, err)
		return
	}
	out := make([]fileTreeEntry, 0, len(dir))
	for _, row := range dir {
		if row.Type()&os.ModeSymlink != 0 {
			continue
		}
		rowInfo, infoErr := row.Info()
		if infoErr != nil {
			respond(w, nil, infoErr)
			return
		}
		childRel := filepath.ToSlash(filepath.Join(rel, row.Name()))
		entry := fileTreeEntry{
			Name:       row.Name(),
			Path:       childRel,
			Exists:     true,
			ModifiedAt: rowInfo.ModTime().UTC().Format(time.RFC3339),
		}
		if row.IsDir() {
			entry.Type = "directory"
			entry.Children = visibleDirectoryCount(filepath.Join(name, row.Name()))
		} else if rowInfo.Mode().IsRegular() {
			entry.Type = "file"
			entry.Size = rowInfo.Size()
			if item, ok := managed[childRel]; ok {
				entry.ItemID = item.ID
				entry.Kind = item.Kind
				entry.Provider = item.Provider
				entry.Side = item.Side
				entry.SHA256 = item.SHA256
			} else {
				entry.Kind = "file"
				entry.Provider = "filesystem"
			}
		} else {
			continue
		}
		out = append(out, entry)
	}
	respond(w, map[string]any{"path": rel, "entries": out}, nil)
}
