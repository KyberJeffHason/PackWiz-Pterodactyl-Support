package publishing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/revisions"
)

type Publisher struct {
	DB                                           *sql.DB
	ReleasesRoot                                 string
	Packwiz                                      packwiz.Runner
	Manager                                      *projects.Manager
	ManagerVersion, PackwizCommit, PackwizSHA256 string
}

func (p *Publisher) Publish(ctx context.Context, projectID, actor, changelog string) (revisions.Manifest, error) {
	unlock := p.Manager.Lock(projectID)
	defer unlock()
	project, err := p.Manager.Get(ctx, projectID)
	if err != nil {
		return revisions.Manifest{}, err
	}
	if _, err = os.Stat(filepath.Join(project.WorkingDirectory, "pack.toml")); err != nil {
		return revisions.Manifest{}, errors.New("pack.toml missing")
	}
	if err = p.Packwiz.Run(ctx, project.WorkingDirectory, "refresh"); err != nil {
		return revisions.Manifest{}, err
	}
	var number int64
	if err = p.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_number),0)+1 FROM revisions WHERE project_id=?`, projectID).Scan(&number); err != nil {
		return revisions.Manifest{}, err
	}
	id := projects.ID()
	base := filepath.Join(p.ReleasesRoot, project.Slug, "releases")
	stage := filepath.Join(base, "."+id+".staging")
	final := filepath.Join(base, fmt.Sprint(number))
	if err = os.MkdirAll(stage, 0750); err != nil {
		return revisions.Manifest{}, err
	}
	defer os.RemoveAll(stage)
	manifest := revisions.Manifest{RevisionID: id, Revision: number, ProjectID: projectID, ManagerVersion: p.ManagerVersion, PackwizCommit: p.PackwizCommit, PackwizSHA256: p.PackwizSHA256}
	if err = copyTree(project.WorkingDirectory, stage, &manifest); err != nil {
		return manifest, err
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	h := sha256.New()
	for _, f := range manifest.Files {
		fmt.Fprintf(h, "%s\x00%s\x00%d\n", f.Path, f.SHA256, f.Size)
	}
	manifest.ContentDigest = hex.EncodeToString(h.Sum(nil))
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "revision.json"), raw, 0640); err != nil {
		return manifest, err
	}
	if err = os.Rename(stage, final); err != nil {
		return manifest, err
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return manifest, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO revisions(id,project_id,revision_number,pack_version,content_digest,release_directory,changelog,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, projectID, number, project.PackVersion, manifest.ContentDigest, final, changelog, actor, now)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE projects SET current_revision_id=?,updated_at=? WHERE id=?`, id, now, projectID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(actor,project_id,operation,request_id,metadata_json,created_at) VALUES(?,?,?,?,?,?)`, actor, projectID, "publish", id, `{"revision":`+fmt.Sprint(number)+`}`, now)
	}
	if err != nil {
		return manifest, err
	}
	root := filepath.Join(p.ReleasesRoot, project.Slug)
	previous, _ := os.Readlink(filepath.Join(root, "current"))
	if err = atomicCurrent(root, final); err != nil {
		return manifest, err
	}
	if err = tx.Commit(); err != nil {
		restoreCurrent(root, previous)
		return manifest, err
	}
	return manifest, nil
}
func copyTree(src, dst string, m *revisions.Manifest) error {
	return filepath.Walk(src, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, name)
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink rejected during publish")
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0750)
		}
		if !info.Mode().IsRegular() {
			return errors.New("non-regular file rejected")
		}
		in, e := os.Open(name)
		if e != nil {
			return e
		}
		defer in.Close()
		target := filepath.Join(dst, rel)
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
		if e != nil {
			return e
		}
		h := sha256.New()
		n, e := io.Copy(io.MultiWriter(out, h), in)
		ce := out.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		m.Files = append(m.Files, revisions.File{Path: filepath.ToSlash(rel), SHA256: hex.EncodeToString(h.Sum(nil)), Size: n})
		return nil
	})
}
func atomicCurrent(root, final string) error {
	if err := os.MkdirAll(root, 0750); err != nil {
		return err
	}
	tmp := filepath.Join(root, ".current-"+fmt.Sprint(time.Now().UnixNano()))
	target, err := filepath.Rel(root, final)
	if err != nil {
		return err
	}
	if err = os.Symlink(target, tmp); err != nil {
		return err
	}
	if err = os.Rename(tmp, filepath.Join(root, "current")); err != nil {
		_ = os.Remove(filepath.Join(root, "current"))
		err = os.Rename(tmp, filepath.Join(root, "current"))
	}
	return err
}
func restoreCurrent(root, previous string) {
	if previous == "" {
		_ = os.Remove(filepath.Join(root, "current"))
		return
	}
	_ = atomicCurrent(root, filepath.Join(root, previous))
}
func (p *Publisher) Rollback(ctx context.Context, projectID string, number int64, actor string) error {
	unlock := p.Manager.Lock(projectID)
	defer unlock()
	project, err := p.Manager.Get(ctx, projectID)
	if err != nil {
		return err
	}
	var id, dir string
	if err = p.DB.QueryRowContext(ctx, `SELECT id,release_directory FROM revisions WHERE project_id=? AND revision_number=?`, projectID, number).Scan(&id, &dir); err != nil {
		return err
	}
	if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(p.ReleasesRoot)+string(os.PathSeparator)) {
		return errors.New("invalid release path")
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET current_revision_id=?,updated_at=? WHERE id=?`, id, now, projectID); err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(actor,project_id,operation,request_id,metadata_json,created_at) VALUES(?,?,?,?,?,?)`, actor, projectID, "rollback", projects.ID(), `{"revision":`+fmt.Sprint(number)+`}`, now)
	}
	if err != nil {
		return err
	}
	root := filepath.Join(p.ReleasesRoot, project.Slug)
	previous, _ := os.Readlink(filepath.Join(root, "current"))
	if err = atomicCurrent(root, dir); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		restoreCurrent(root, previous)
	}
	return err
}
