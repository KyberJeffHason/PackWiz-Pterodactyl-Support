package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var publicPart = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func PublicHandler(releasesRoot, blobsRoot string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		var name string
		immutable := false
		if len(parts) >= 4 && parts[0] == "blobs" && parts[1] == "sha256" && len(parts[2]) == 64 && publicPart.MatchString(parts[2]) && publicPart.MatchString(parts[3]) {
			name = filepath.Join(blobsRoot, "sha256", parts[2][:2], parts[2])
			immutable = true
		} else if len(parts) >= 2 && publicPart.MatchString(parts[0]) {
			for _, p := range parts {
				if !publicPart.MatchString(p) {
					http.NotFound(w, r)
					return
				}
			}
			if len(parts) >= 3 && parts[1] == "releases" {
				name = filepath.Join(append([]string{releasesRoot, parts[0]}, parts[1:]...)...)
				immutable = true
			} else {
				name = filepath.Join(append([]string{releasesRoot, parts[0], "current"}, parts[1:]...)...)
			}
		} else {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(name)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if immutable {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, name)
	})
}
