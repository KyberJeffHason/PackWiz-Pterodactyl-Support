package api

import "testing"

func TestNormalizeManagedTreePath(t *testing.T) {
	valid := map[string]string{
		"":                      "",
		"config":                "config",
		"config/client.toml":    "config/client.toml",
		"defaultconfigs/test":   "defaultconfigs/test",
		"kubejs/server_scripts": "kubejs/server_scripts",
	}
	for input, expected := range valid {
		actual, err := normalizeManagedTreePath(input)
		if err != nil {
			t.Fatalf("%q rejected: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("%q normalized to %q, want %q", input, actual, expected)
		}
	}
	for _, input := range []string{"mods", "../config", "/config", "config\\test"} {
		if _, err := normalizeManagedTreePath(input); err == nil {
			t.Fatalf("unsafe or unmanaged path %q accepted", input)
		}
	}
}
