package api

import "testing"

func TestNormalizeBulkFileDeleteIDs(t *testing.T) {
	ids, err := normalizeBulkFileDeleteIDs([]string{" first ", "second", "first"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "first" || ids[1] != "second" {
		t.Fatalf("unexpected normalized ids: %#v", ids)
	}

	for _, input := range [][]string{nil, {}, {""}, {"   "}} {
		if _, err := normalizeBulkFileDeleteIDs(input); err == nil {
			t.Fatalf("invalid ids accepted: %#v", input)
		}
	}

	tooMany := make([]string, maxBulkFileDeletes+1)
	for i := range tooMany {
		tooMany[i] = "id"
	}
	if _, err := normalizeBulkFileDeleteIDs(tooMany); err == nil {
		t.Fatal("oversized bulk delete accepted")
	}
}

func TestManagedFileRemovalTarget(t *testing.T) {
	tests := []struct {
		name   string
		value  item
		target string
		bad    bool
	}{
		{name: "managed config", value: item{Kind: "file", Provider: "local", TargetPath: "config/example.toml"}, target: "config/example.toml"},
		{name: "managed kubejs", value: item{Kind: "file", Provider: "url", TargetPath: "kubejs/server_scripts/example.js"}, target: "kubejs/server_scripts/example.js"},
		{name: "client root", value: item{Kind: "file", Provider: "client-file", TargetPath: "client-files/servers.dat"}, target: "servers.dat.pw.toml"},
		{name: "mod rejected", value: item{Kind: "mod", Provider: "modrinth", TargetPath: "mods/example.pw.toml"}, bad: true},
		{name: "outside roots", value: item{Kind: "file", Provider: "local", TargetPath: "mods/not-a-managed-file.txt"}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := managedFileRemovalTarget(tt.value)
			if tt.bad {
				if err == nil {
					t.Fatalf("unsafe target accepted as %q", actual)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if actual != tt.target {
				t.Fatalf("got %q, want %q", actual, tt.target)
			}
		})
	}
}
