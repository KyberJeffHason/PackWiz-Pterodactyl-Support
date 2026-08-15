package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(dataDir string) (*sql.DB, error) {
	dir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	d, err := sql.Open("sqlite", filepath.Join(dir, "manager.sqlite")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	schema, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		d.Close()
		return nil, err
	}
	if _, err = d.Exec(string(schema)); err != nil {
		d.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}
