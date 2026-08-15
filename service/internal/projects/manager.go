package projects

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

type Project struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	DisplayName      string    `json:"display_name"`
	MinecraftVersion string    `json:"minecraft_version"`
	Loader           string    `json:"loader"`
	LoaderVersion    string    `json:"loader_version"`
	PackVersion      string    `json:"pack_version"`
	WorkingDirectory string    `json:"-"`
	CurrentRevision  *string   `json:"current_revision,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type Manager struct {
	DB           *sql.DB
	ProjectsRoot string
	Packwiz      packwiz.Runner
	locks        sync.Map
}

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(b[:4]), hex.EncodeToString(b[4:6]), hex.EncodeToString(b[6:8]), hex.EncodeToString(b[8:10]), hex.EncodeToString(b[10:]))
}
func (m *Manager) Lock(id string) func() {
	value, _ := m.locks.LoadOrStore(id, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
func (m *Manager) Mutate(ctx context.Context, id string, fn func(Project) error) error {
	return m.MutateAndCommit(ctx, id, fn, func() error { return nil })
}
func (m *Manager) MutateAndCommit(ctx context.Context, id string, fn func(Project) error, commit func() error) error {
	unlock := m.Lock(id)
	defer unlock()
	p, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	container, err := os.MkdirTemp(filepath.Dir(p.WorkingDirectory), "."+id+"-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(container)
	backup := filepath.Join(container, "tree")
	if err = os.CopyFS(backup, os.DirFS(p.WorkingDirectory)); err != nil {
		return err
	}
	if err = fn(p); err == nil {
		err = commit()
	}
	if err == nil {
		return nil
	}
	failed := p.WorkingDirectory + ".failed"
	_ = os.RemoveAll(failed)
	if moveErr := os.Rename(p.WorkingDirectory, failed); moveErr != nil {
		return fmt.Errorf("mutation failed: %v; rollback move failed: %w", err, moveErr)
	}
	if moveErr := os.Rename(backup, p.WorkingDirectory); moveErr != nil {
		_ = os.Rename(failed, p.WorkingDirectory)
		return fmt.Errorf("mutation failed: %v; rollback restore failed: %w", err, moveErr)
	}
	_ = os.RemoveAll(failed)
	return err
}
func (m *Manager) Create(ctx context.Context, p Project) (Project, error) {
	if !slugPattern.MatchString(p.Slug) {
		return p, errors.New("slug must be 3-64 lowercase letters, numbers, or hyphens")
	}
	p.ID = ID()
	p.WorkingDirectory = filepath.Join(m.ProjectsRoot, p.ID)
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	if err := os.MkdirAll(p.WorkingDirectory, 0750); err != nil {
		return p, err
	}
	if err := m.Packwiz.Run(ctx, p.WorkingDirectory, packwiz.InitArgs(p.DisplayName, p.PackVersion, p.MinecraftVersion, p.Loader, p.LoaderVersion)...); err != nil {
		os.RemoveAll(p.WorkingDirectory)
		return p, err
	}
	_, err := m.DB.ExecContext(ctx, `INSERT INTO projects(id,slug,display_name,minecraft_version,loader,loader_version,pack_version,working_directory,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Slug, p.DisplayName, p.MinecraftVersion, p.Loader, p.LoaderVersion, p.PackVersion, p.WorkingDirectory, p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		os.RemoveAll(p.WorkingDirectory)
	}
	return p, err
}
func (m *Manager) Import(ctx context.Context, p Project, source string) (Project, error) {
	if !slugPattern.MatchString(p.Slug) {
		return p, errors.New("invalid project slug")
	}
	if info, err := os.Stat(filepath.Join(source, "pack.toml")); err != nil || !info.Mode().IsRegular() {
		return p, errors.New("archive root must contain pack.toml")
	}
	p.ID = ID()
	p.WorkingDirectory = filepath.Join(m.ProjectsRoot, p.ID)
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	if err := os.CopyFS(p.WorkingDirectory, os.DirFS(source)); err != nil {
		return p, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(p.WorkingDirectory)
		}
	}()
	if err := m.Packwiz.Run(ctx, p.WorkingDirectory, "refresh"); err != nil {
		return p, err
	}
	_, err := m.DB.ExecContext(ctx, `INSERT INTO projects(id,slug,display_name,minecraft_version,loader,loader_version,pack_version,working_directory,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Slug, p.DisplayName, p.MinecraftVersion, p.Loader, p.LoaderVersion, p.PackVersion, p.WorkingDirectory, p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano))
	if err == nil {
		ok = true
	}
	return p, err
}
func (m *Manager) List(ctx context.Context) ([]Project, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT id,slug,display_name,minecraft_version,loader,loader_version,pack_version,working_directory,current_revision_id,created_at,updated_at FROM projects ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var current sql.NullString
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Slug, &p.DisplayName, &p.MinecraftVersion, &p.Loader, &p.LoaderVersion, &p.PackVersion, &p.WorkingDirectory, &current, &created, &updated); err != nil {
			return nil, err
		}
		if current.Valid {
			p.CurrentRevision = &current.String
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, p)
	}
	return out, rows.Err()
}
func (m *Manager) Get(ctx context.Context, id string) (Project, error) {
	var p Project
	var current sql.NullString
	var created, updated string
	err := m.DB.QueryRowContext(ctx, `SELECT id,slug,display_name,minecraft_version,loader,loader_version,pack_version,working_directory,current_revision_id,created_at,updated_at FROM projects WHERE id=?`, id).Scan(&p.ID, &p.Slug, &p.DisplayName, &p.MinecraftVersion, &p.Loader, &p.LoaderVersion, &p.PackVersion, &p.WorkingDirectory, &current, &created, &updated)
	if current.Valid {
		p.CurrentRevision = &current.String
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, err
}
