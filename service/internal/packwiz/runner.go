package packwiz

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type Runner struct {
	Binary  string
	Timeout time.Duration
}

func (r Runner) Run(ctx context.Context, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("packwiz %q failed: %w: %s", args, err, stderr.String())
	}
	return nil
}

func InitArgs(name, version, mcVersion, loader, loaderVersion string) []string {
	args := []string{"init", "--name", name, "--version", version, "--mc-version", mcVersion, "--modloader", loader, "-y"}
	if loaderVersion != "" {
		args = append(args, "--"+loader+"-version", loaderVersion)
	}
	return args
}
func ModrinthAddArgs(projectID, versionID string) []string {
	args := []string{"modrinth", "add", "--project-id", projectID}
	if versionID != "" {
		args = append(args, "--version-id", versionID)
	}
	return append(args, "-y")
}
func CurseForgeAddArgs(projectID, fileID string) []string {
	args := []string{"curseforge", "add", "--project-id", projectID}
	if fileID != "" {
		args = append(args, "--file-id", fileID)
	}
	return append(args, "-y")
}
