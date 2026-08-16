package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseImportedCurseForgeMetadata(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "mods", "create.pw.toml")
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	raw := `name = "Create"
filename = "create.jar"
side = "both"

[download]
hash-format = "sha1"
hash = "abc"
mode = "metadata:curseforge"

[update.curseforge]
file-id = 123456
project-id = 328085
`
	if err := os.WriteFile(name, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	meta, err := parseImportedMetadata(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Provider != "curseforge" || meta.ProviderProjectID != "328085" || meta.ProviderVersionID != "123456" {
		t.Fatalf("unexpected provider metadata: %#v", meta)
	}
	if meta.MetadataPath != "mods/create.pw.toml" || meta.DestinationPath != "mods/create.jar" {
		t.Fatalf("unexpected paths: %#v", meta)
	}
}

func TestParseImportedModrinthMetadata(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "mods", "sodium.pw.toml")
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	raw := `name = "Sodium"
filename = "sodium.jar"
side = "client"

[download]
hash-format = "sha256"
hash = "deadbeef"
url = "https://cdn.modrinth.com/example.jar"

[update.modrinth]
mod-id = "AANobbMI"
version = "abc123"
`
	if err := os.WriteFile(name, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	meta, err := parseImportedMetadata(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Provider != "modrinth" || meta.ProviderProjectID != "AANobbMI" || meta.ProviderVersionID != "abc123" {
		t.Fatalf("unexpected provider metadata: %#v", meta)
	}
	if meta.Side != "client" || meta.SHA256 != "deadbeef" {
		t.Fatalf("unexpected side/hash: %#v", meta)
	}
}
