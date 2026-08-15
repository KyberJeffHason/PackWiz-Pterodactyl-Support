package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetMetadataSide(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "mods", "test.pw.toml")
	if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("name = \"Test\"\nside = \"both\"\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := setMetadataSide(root, "mods/test.pw.toml", "server"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(name)
	if err != nil || !strings.Contains(string(raw), "side = \"server\"") {
		t.Fatalf("side not updated: %s (%v)", raw, err)
	}
	if err = setMetadataSide(root, "../outside", "both"); err == nil {
		t.Fatal("traversal accepted")
	}
}
