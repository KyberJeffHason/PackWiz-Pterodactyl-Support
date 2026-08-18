package api

import "testing"

func TestNormalizeManagedFolderDeletePath(t *testing.T) {
	valid := []string{
		"config",
		"config/example",
		"defaultconfigs",
		"defaultconfigs/nested/folder",
		"kubejs",
		"kubejs/server_scripts",
		"datapacks",
		"datapacks/my pack/data",
		"resourcepacks",
		"resourcepacks/custom/assets",
	}
	for _, input := range valid {
		actual, err := normalizeManagedFolderDeletePath(input)
		if err != nil {
			t.Fatalf("%q rejected: %v", input, err)
		}
		if actual != input {
			t.Fatalf("%q normalized to %q", input, actual)
		}
	}

	invalid := []string{
		"",
		"client-files",
		"client-files/nested",
		"mods/example",
		"../config/example",
		"/config/example",
	}
	for _, input := range invalid {
		if _, err := normalizeManagedFolderDeletePath(input); err == nil {
			t.Fatalf("unsafe or undeletable folder path %q accepted", input)
		}
	}
}
