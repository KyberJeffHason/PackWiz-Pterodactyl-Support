package security

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func SafeRelative(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, 0) || filepath.IsAbs(value) || strings.HasPrefix(value, "/") || filepath.VolumeName(value) != "" || strings.Contains(value, "\\") {
		return "", errors.New("invalid relative path")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path traversal rejected")
	}
	return clean, nil
}

func SafeFilename(value string) (string, error) {
	name, err := SafeRelative(value)
	if err != nil || strings.Contains(name, "/") {
		return "", errors.New("invalid filename")
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"|?*`, r) {
			return '-'
		}
		return r
	}, name)
	if name == "" {
		return "", errors.New("empty filename")
	}
	return name, nil
}

func SafeJoin(root, relative string) (string, error) {
	clean, err := SafeRelative(relative)
	if err != nil {
		return "", err
	}
	parts := strings.Split(clean, "/")
	current := filepath.Clean(root)
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink path rejected")
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
	}
	return filepath.Join(root, filepath.FromSlash(clean)), nil
}
