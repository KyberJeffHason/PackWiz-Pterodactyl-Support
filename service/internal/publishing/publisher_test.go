package publishing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/packwiz-manager/packwiz-manager/service/internal/revisions"
)

func TestCopyTreeHashesAndRejectsSymlink(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "pack.toml"), []byte("pack"), 0644); err != nil {
		t.Fatal(err)
	}
	var manifest revisions.Manifest
	if err := copyTree(src, dst, &manifest); err != nil || len(manifest.Files) != 1 || manifest.Files[0].SHA256 == "" {
		t.Fatalf("%+v %v", manifest, err)
	}
	if err := os.Symlink("pack.toml", filepath.Join(src, "bad")); err == nil {
		if err := copyTree(src, t.TempDir(), &revisions.Manifest{}); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}

func TestAtomicCurrentSwitchesRevision(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "releases", "1")
	two := filepath.Join(root, "releases", "2")
	if err := os.MkdirAll(one, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(two, 0755); err != nil {
		t.Fatal(err)
	}
	if err := atomicCurrent(root, one); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := atomicCurrent(root, two); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil || target != filepath.Join("releases", "2") {
		t.Fatalf("target=%q err=%v", target, err)
	}
}
