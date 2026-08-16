package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

var sideLine = regexp.MustCompile(`(?m)^side\s*=\s*"(?:client|server|both)"\s*$`)

type item struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"project_id"`
	Kind              string         `json:"kind"`
	Provider          string         `json:"provider"`
	ProviderProjectID string         `json:"provider_project_id,omitempty"`
	ProviderVersionID string         `json:"provider_version_id,omitempty"`
	DisplayName       string         `json:"display_name"`
	TargetPath        string         `json:"target_path"`
	Filename          string         `json:"filename"`
	Side              string         `json:"side"`
	SHA256            string         `json:"sha256,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	Enabled           bool           `json:"enabled"`
}

func allowedTarget(target string) bool {
	for _, prefix := range []string{"config/", "defaultconfigs/", "kubejs/", "datapacks/", "resourcepacks/"} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func (a *API) storeManagedFile(ctx context.Context, projectID, kind, provider, display, target, side, sourceURL string, content []byte) (string, string, error) {
	if !allowedTarget(target) {
		return "", "", errors.New("target path is outside configured allowlist")
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := projects.ID()
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO items(id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,source_url,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, projectID, kind, provider, display, target, path.Base(target), side, digest, nullable(sourceURL), now, now)
	if err != nil {
		return "", "", err
	}
	err = a.Projects.MutateAndCommit(ctx, projectID, func(p projects.Project) error {
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
		return a.Projects.Packwiz.Run(ctx, p.WorkingDirectory, "refresh")
	}, tx.Commit)
	return id, digest, err
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func scanItem(row interface{ Scan(...any) error }) (item, error) {
	var v item
	var providerProject, providerVersion, hash, metadata sql.NullString
	var enabled int
	err := row.Scan(&v.ID, &v.ProjectID, &v.Kind, &v.Provider, &providerProject, &providerVersion, &v.DisplayName, &v.TargetPath, &v.Filename, &v.Side, &hash, &metadata, &enabled)
	v.ProviderProjectID = providerProject.String
	v.ProviderVersionID = providerVersion.String
	v.SHA256 = hash.String
	if metadata.Valid && metadata.String != "" && metadata.String != "{}" {
		_ = json.Unmarshal([]byte(metadata.String), &v.Metadata)
	}
	v.Enabled = enabled != 0
	return v, err
}

func metadataFiles(root string) (map[string]bool, error) {
	out := map[string]bool{}
	err := filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && strings.HasSuffix(name, ".pw.toml") {
			rel, _ := filepath.Rel(root, name)
			out[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	return out, err
}
func newMetadata(root string, before, after map[string]bool, provider, projectID string) (string, error) {
	var created []string
	for name := range after {
		if !before[name] {
			created = append(created, name)
		}
	}
	key := "mod-id"
	if provider == "curseforge" {
		key = "project-id"
	}
	match := regexp.MustCompile(`(?m)^` + key + `\s*=\s*"?` + regexp.QuoteMeta(projectID) + `"?\s*$`)
	for _, name := range created {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err == nil && len(raw) <= 1<<20 && match.Match(raw) {
			return name, nil
		}
	}
	if len(created) == 1 {
		return created[0], nil
	}
	return "", errors.New("Packwiz primary metadata file could not be identified")
}
func setMetadataSide(root, relative, side string) error {
	if side != "client" && side != "server" && side != "both" {
		return errors.New("invalid side")
	}
	name, err := security.SafeJoin(root, relative)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return errors.New("metadata file too large")
	}
	updated := sideLine.ReplaceAllString(string(raw), fmt.Sprintf("side = %q", side))
	if !sideLine.Match(raw) {
		updated = fmt.Sprintf("side = %q\n", side) + updated
	}
	tmp := name + ".tmp"
	if err = os.WriteFile(tmp, []byte(updated), 0640); err == nil {
		err = os.Rename(tmp, name)
	}
	return err
}
