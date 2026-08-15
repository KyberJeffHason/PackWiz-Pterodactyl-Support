package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/packwiz-manager/packwiz-manager/service/internal/files"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

func (a *API) importProject(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.edit") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxArchiveBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		bad(w, err)
		return
	}
	upload, _, err := r.FormFile("archive")
	if err != nil {
		bad(w, err)
		return
	}
	defer upload.Close()
	zipFile, err := os.CreateTemp(a.TmpRoot, "import-*.zip")
	if err != nil {
		respond(w, nil, err)
		return
	}
	zipName := zipFile.Name()
	defer os.Remove(zipName)
	n, err := io.Copy(zipFile, io.LimitReader(upload, a.MaxArchiveBytes+1))
	closeErr := zipFile.Close()
	if err != nil || closeErr != nil || n > a.MaxArchiveBytes {
		bad(w, errors.New("archive exceeds configured limit"))
		return
	}
	expanded, err := os.MkdirTemp(a.TmpRoot, "import-tree-*")
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer os.RemoveAll(expanded)
	if err = files.ExtractZIP(zipName, expanded, files.ArchiveLimits{Files: a.MaxArchiveFiles, Bytes: a.MaxArchiveBytes}); err != nil {
		bad(w, err)
		return
	}
	packRoot, err := files.PackRoot(expanded)
	if err != nil {
		bad(w, err)
		return
	}
	p := projects.Project{Slug: r.FormValue("slug"), DisplayName: r.FormValue("display_name"), MinecraftVersion: r.FormValue("minecraft_version"), Loader: r.FormValue("loader"), LoaderVersion: r.FormValue("loader_version"), PackVersion: r.FormValue("pack_version")}
	p, err = a.Projects.Import(r.Context(), p, packRoot)
	if err == nil {
		w.WriteHeader(http.StatusCreated)
	}
	respond(w, p, err)
}

func (a *API) importURL(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.upload") {
		return
	}
	var in struct {
		URL         string `json:"url"`
		Target      string `json:"target_path"`
		DisplayName string `json:"display_name"`
		Kind        string `json:"kind"`
		Side        string `json:"side"`
	}
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	target, err := security.SafeRelative(in.Target)
	if err != nil || !allowedTarget(target) {
		bad(w, errors.New("target path rejected"))
		return
	}
	if in.Side != "client" && in.Side != "server" && in.Side != "both" {
		bad(w, errors.New("invalid side"))
		return
	}
	if !map[string]bool{"file": true, "config": true, "kubejs": true, "datapack": true, "resourcepack": true}[in.Kind] {
		bad(w, errors.New("invalid kind"))
		return
	}
	if destination, parseErr := url.Parse(strings.TrimSpace(in.URL)); parseErr == nil {
		slog.Info("remote file import", "destination", destination.Scheme+"://"+destination.Hostname(), "project", r.PathValue("id"))
	}
	content, err := files.FetchRemote(r.Context(), a.RemoteHTTP, strings.TrimSpace(in.URL), a.Blobs.MaxBytes)
	if err != nil {
		bad(w, err)
		return
	}
	id, digest, err := a.storeManagedFile(r.Context(), r.PathValue("id"), in.Kind, "url", in.DisplayName, target, in.Side, in.URL, content)
	respond(w, map[string]string{"id": id, "sha256": digest, "target_path": filepath.ToSlash(target)}, err)
}
