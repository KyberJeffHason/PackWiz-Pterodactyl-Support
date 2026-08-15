package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicHandlerRejectsTraversalAndMutation(t *testing.T) {
	h := PublicHandler(t.TempDir(), t.TempDir())
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"POST", "/pack/pack.toml", 405}, {"GET", "/../secret", 404}, {"GET", "/pack/%2e%2e/secret", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestPublicHandlerServesStableAndRange(t *testing.T) {
	releases, blobs := t.TempDir(), t.TempDir()
	revision := filepath.Join(releases, "pack", "releases", "1")
	if err := os.MkdirAll(revision, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(revision, "pack.toml"), []byte("abcdef"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("releases", "1"), filepath.Join(releases, "pack", "current")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	r := httptest.NewRequest(http.MethodGet, "/pack/pack.toml", nil)
	r.Header.Set("Range", "bytes=1-2")
	w := httptest.NewRecorder()
	PublicHandler(releases, blobs).ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "bc" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}
