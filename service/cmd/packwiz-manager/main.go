package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/packwiz-manager/packwiz-manager/service/internal/api"
	"github.com/packwiz-manager/packwiz-manager/service/internal/auth"
	"github.com/packwiz-manager/packwiz-manager/service/internal/config"
	database "github.com/packwiz-manager/packwiz-manager/service/internal/db"
	"github.com/packwiz-manager/packwiz-manager/service/internal/files"
	"github.com/packwiz-manager/packwiz-manager/service/internal/packwiz"
	"github.com/packwiz-manager/packwiz-manager/service/internal/projects"
	"github.com/packwiz-manager/packwiz-manager/service/internal/providers/curseforge"
	"github.com/packwiz-manager/packwiz-manager/service/internal/providers/modrinth"
	"github.com/packwiz-manager/packwiz-manager/service/internal/publishing"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	for _, d := range []string{"projects", "blobs", "releases", "tmp"} {
		if err = os.MkdirAll(filepath.Join(cfg.DataDir, d), 0750); err != nil {
			slog.Error("storage setup failed", "error", err)
			os.Exit(1)
		}
	}
	d, err := database.Open(cfg.DataDir)
	if err != nil {
		slog.Error("database failed", "error", err)
		os.Exit(1)
	}
	defer d.Close()
	runner := packwiz.Runner{Binary: cfg.PackwizBinary, Timeout: cfg.CommandTimeout}
	pm := &projects.Manager{DB: d, ProjectsRoot: filepath.Join(cfg.DataDir, "projects"), Packwiz: runner}
	pub := &publishing.Publisher{DB: d, ReleasesRoot: filepath.Join(cfg.DataDir, "releases"), Packwiz: runner, Manager: pm, ManagerVersion: version, PackwizCommit: os.Getenv("PWM_PACKWIZ_COMMIT"), PackwizSHA256: fileSHA(cfg.PackwizBinary)}
	app := &api.API{Projects: pm, Publisher: pub, Blobs: files.Store{Root: filepath.Join(cfg.DataDir, "blobs"), MaxBytes: cfg.MaxUpload}, DB: d, Modrinth: modrinth.New(version), CurseForge: curseforge.New(cfg.CurseForgeKey), PublicBaseURL: cfg.PublicBaseURL}
	management := http.NewServeMux()
	management.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	management.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if d.PingContext(r.Context()) != nil {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte("ready\n"))
	})
	management.Handle("/api/v1/", http.StripPrefix("/api/v1", auth.Middleware(cfg.Token, app.Routes())))
	ms := server(cfg.Listen, management)
	ps := server(cfg.PublicListen, http.StripPrefix("/public/", api.PublicHandler(filepath.Join(cfg.DataDir, "releases"), filepath.Join(cfg.DataDir, "blobs"))))
	go serve(ms)
	go serve(ps)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = api.Shutdown(shutdown, ms, ps)
}
func server(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}
func serve(s *http.Server) {
	slog.Info("listener started", "address", s.Addr)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("listener stopped", "error", err)
		os.Exit(1)
	}
}
func fileSHA(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		return "unavailable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
