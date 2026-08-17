package api

import "testing"

func TestClientFileName(t *testing.T) {
	valid := map[string]string{
		"client-files/servers.dat":      "servers.dat",
		"client-files/servers+prod.dat": "servers+prod.dat",
		"client-files/options.txt":      "options.txt",
	}
	for target, expected := range valid {
		actual, isClient, err := clientFileName(target)
		if err != nil {
			t.Fatalf("%q rejected: %v", target, err)
		}
		if !isClient {
			t.Fatalf("%q was not identified as a client file", target)
		}
		if actual != expected {
			t.Fatalf("%q mapped to %q, want %q", target, actual, expected)
		}
	}
}

func TestClientFileNameRejectsUnsafeOrReservedTargets(t *testing.T) {
	for _, target := range []string{
		"client-files",
		"client-files/sub/servers.dat",
		"client-files/pack.toml",
		"client-files/index.toml",
		"client-files/.packwizignore",
		"client-files/servers.pw.toml",
		"client-files/bad name.dat",
	} {
		if _, isClient, err := clientFileName(target); !isClient || err == nil {
			t.Fatalf("unsafe or reserved client target %q was accepted", target)
		}
	}
}

func TestClientFileNameIgnoresOtherManagedRoots(t *testing.T) {
	name, isClient, err := clientFileName("config/client.toml")
	if err != nil || isClient || name != "" {
		t.Fatalf("normal managed file was treated as client root file: name=%q client=%v err=%v", name, isClient, err)
	}
}

func TestClientFileMetadataPath(t *testing.T) {
	if actual := clientFileMetadataPath("servers.dat"); actual != "servers.dat.pw.toml" {
		t.Fatalf("metadata path = %q", actual)
	}
}
