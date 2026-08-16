package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
)

var (
	importStringField = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*"((?:\\.|[^"])*)"\s*$`)
	}
	importBareField = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*("(?:\\.|[^"])*"|[0-9A-Za-z._-]+)\s*$`)
	}
	importSection = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?ms)^\[` + regexp.QuoteMeta(name) + `\]\s*\n(.*?)(?:^\[|\z)`)
	}
)

type importedMetadata struct {
	DisplayName       string
	Filename          string
	Side              string
	SHA256            string
	Provider          string
	ProviderProjectID string
	ProviderVersionID string
	MetadataPath      string
	DestinationPath   string
}

func decodeTOMLString(v string) string {
	decoded, err := strconv.Unquote("\"" + v + "\"")
	if err != nil {
		return v
	}
	return decoded
}

func stringField(raw []byte, name string) string {
	m := importStringField(name).FindSubmatch(raw)
	if len(m) != 2 {
		return ""
	}
	return decodeTOMLString(string(m[1]))
}

func section(raw []byte, name string) []byte {
	m := importSection(name).FindSubmatch(raw)
	if len(m) != 2 {
		return nil
	}
	return m[1]
}

func bareField(raw []byte, name string) string {
	m := importBareField(name).FindSubmatch(raw)
	if len(m) != 2 {
		return ""
	}
	v := string(m[1])
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		return decodeTOMLString(v[1 : len(v)-1])
	}
	return v
}

func parseImportedMetadata(root, name string) (importedMetadata, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return importedMetadata{}, err
	}
	if len(raw) > 1<<20 {
		return importedMetadata{}, errors.New("Packwiz metadata file exceeds 1 MiB")
	}
	rel, err := filepath.Rel(root, name)
	if err != nil {
		return importedMetadata{}, err
	}
	meta := importedMetadata{
		DisplayName:  stringField(raw, "name"),
		Filename:     stringField(raw, "filename"),
		Side:         stringField(raw, "side"),
		Provider:     "local",
		MetadataPath: filepath.ToSlash(rel),
	}
	if meta.DisplayName == "" {
		meta.DisplayName = strings.TrimSuffix(filepath.Base(name), ".pw.toml")
	}
	if meta.Side == "" {
		meta.Side = "both"
	}
	if meta.Side != "client" && meta.Side != "server" && meta.Side != "both" {
		meta.Side = "both"
	}
	if meta.Filename == "" {
		meta.Filename = strings.TrimSuffix(filepath.Base(name), ".pw.toml")
	}
	meta.DestinationPath = filepath.ToSlash(filepath.Join(filepath.Dir(rel), filepath.FromSlash(meta.Filename)))
	if download := section(raw, "download"); download != nil {
		if strings.EqualFold(stringField(download, "hash-format"), "sha256") {
			meta.SHA256 = stringField(download, "hash")
		}
	}
	if mr := section(raw, "update.modrinth"); mr != nil {
		meta.Provider = "modrinth"
		meta.ProviderProjectID = bareField(mr, "mod-id")
		meta.ProviderVersionID = bareField(mr, "version")
	} else if cf := section(raw, "update.curseforge"); cf != nil {
		meta.Provider = "curseforge"
		meta.ProviderProjectID = bareField(cf, "project-id")
		meta.ProviderVersionID = bareField(cf, "file-id")
	}
	return meta, nil
}

func importedKind(relative string) string {
	switch {
	case strings.HasPrefix(relative, "config/"), strings.HasPrefix(relative, "defaultconfigs/"):
		return "config"
	case strings.HasPrefix(relative, "kubejs/"):
		return "kubejs"
	case strings.HasPrefix(relative, "datapacks/"):
		return "datapack"
	case strings.HasPrefix(relative, "resourcepacks/"):
		return "resourcepack"
	default:
		return "file"
	}
}

func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *API) syncImportedItems(ctx context.Context, tx *sql.Tx, projectID, root string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE project_id=?`, projectID); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	referenced := map[string]bool{}
	metadataPaths := map[string]bool{}
	var metadata []importedMetadata

	err := filepath.Walk(root, func(name string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(strings.ToLower(name), ".pw.toml") {
			return nil
		}
		meta, err := parseImportedMetadata(root, name)
		if err != nil {
			return err
		}
		metadata = append(metadata, meta)
		metadataPaths[meta.MetadataPath] = true
		referenced[meta.DestinationPath] = true
		return nil
	})
	if err != nil {
		return err
	}

	for _, meta := range metadata {
		metadataJSON, _ := json.Marshal(map[string]any{"imported": true})
		_, err = tx.ExecContext(ctx, `INSERT INTO items(id,project_id,kind,provider,provider_project_id,provider_version_id,display_name,target_path,filename,side,expected_sha256,metadata_json,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
			projects.ID(), projectID, "mod", meta.Provider, nullable(meta.ProviderProjectID), nullable(meta.ProviderVersionID), meta.DisplayName, meta.MetadataPath, meta.Filename, meta.Side, nullable(meta.SHA256), string(metadataJSON), now, now)
		if err != nil {
			return err
		}
	}

	return filepath.Walk(root, func(name string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "pack.toml" || rel == "index.toml" || metadataPaths[rel] || referenced[rel] || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		digest, err := fileSHA256(name)
		if err != nil {
			return err
		}
		metadataJSON, _ := json.Marshal(map[string]any{"imported": true})
		_, err = tx.ExecContext(ctx, `INSERT INTO items(id,project_id,kind,provider,display_name,target_path,filename,side,expected_sha256,metadata_json,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,1,?,?)`,
			projects.ID(), projectID, importedKind(rel), "local", filepath.Base(rel), rel, filepath.Base(rel), "both", digest, string(metadataJSON), now, now)
		return err
	})
}
