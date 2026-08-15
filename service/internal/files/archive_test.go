package files

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func makeZIP(t *testing.T, entries map[string]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "pack.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for path, body := range entries {
		w, err := z.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestExtractZIP(t *testing.T) {
	dst := t.TempDir()
	if err := ExtractZIP(makeZIP(t, map[string]string{"pack.toml": "ok", "config/a": "x"}), dst, ArchiveLimits{Files: 5, Bytes: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "pack.toml")); err != nil {
		t.Fatal(err)
	}
}
func TestExtractZIPRejectsTraversalAndLimits(t *testing.T) {
	for _, tc := range []struct {
		entries map[string]string
		limits  ArchiveLimits
	}{{map[string]string{"../bad": "x"}, ArchiveLimits{5, 10}}, {map[string]string{"a": "12345"}, ArchiveLimits{5, 4}}} {
		if err := ExtractZIP(makeZIP(t, tc.entries), t.TempDir(), tc.limits); err == nil {
			t.Fatal("unsafe archive accepted")
		}
	}
}

func TestExtractZIPRejectsSymlink(t *testing.T) {
	name := filepath.Join(t.TempDir(), "link.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	header := &zip.FileHeader{Name: "config/link"}
	header.SetMode(os.ModeSymlink | 0777)
	w, err := z.CreateHeader(header)
	if err == nil {
		_, err = w.Write([]byte("../../outside"))
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = ExtractZIP(name, t.TempDir(), ArchiveLimits{Files: 5, Bytes: 100}); err == nil {
		t.Fatal("archive symlink accepted")
	}
}

func TestPackRootAcceptsWrappedArchive(t *testing.T) {
	root := t.TempDir()
	wrapped := filepath.Join(root, "my-pack")
	if err := os.Mkdir(wrapped, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wrapped, "pack.toml"), []byte("name='x'"), 0640); err != nil {
		t.Fatal(err)
	}
	got, err := PackRoot(root)
	if err != nil || got != wrapped {
		t.Fatalf("got %q, %v", got, err)
	}
}
