package files

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

type ArchiveLimits struct {
	Files int
	Bytes int64
}

func ExtractZIP(source string, destination string, limits ArchiveLimits) error {
	z, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer z.Close()
	if limits.Files < 1 || limits.Bytes < 1 || len(z.File) > limits.Files {
		return errors.New("archive entry limit exceeded")
	}
	var expanded int64
	for _, entry := range z.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "" {
			continue
		}
		rel, err := security.SafeRelative(name)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() && !entry.FileInfo().IsDir() {
			return errors.New("archive link or special file rejected")
		}
		if entry.UncompressedSize64 > uint64(limits.Bytes-expanded) {
			return errors.New("archive expansion limit exceeded")
		}
		expanded += int64(entry.UncompressedSize64)
		target, err := security.SafeJoin(destination, rel)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0750); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(in, limits.Bytes+1))
		inErr, outErr := in.Close(), out.Close()
		if copyErr != nil {
			return copyErr
		}
		if inErr != nil {
			return inErr
		}
		if outErr != nil {
			return outErr
		}
	}
	return nil
}

func PackRoot(extracted string) (string, error) {
	if info, err := os.Stat(filepath.Join(extracted, "pack.toml")); err == nil && info.Mode().IsRegular() {
		return extracted, nil
	}
	entries, err := os.ReadDir(extracted)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return "", errors.New("archive must contain pack.toml at root or in one top-level directory")
	}
	root := filepath.Join(extracted, entries[0].Name())
	if info, err := os.Stat(filepath.Join(root, "pack.toml")); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("Packwiz pack.toml not found")
	}
	return root, nil
}
