package files

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Blob struct {
	SHA256, Path, Filename, MIME string
	Size                         int64
}

type Store struct {
	Root          string
	MaxBytes      int64
	MaxJAREntries int
}

func (s Store) Put(r io.Reader, filename, mime string, requireJAR bool) (Blob, error) {
	if s.MaxBytes < 1 {
		return Blob{}, errors.New("invalid upload limit")
	}
	if err := os.MkdirAll(filepath.Join(s.Root, "sha256"), 0750); err != nil {
		return Blob{}, err
	}
	tmp, err := os.CreateTemp(filepath.Join(s.Root, "sha256"), ".upload-*")
	if err != nil {
		return Blob{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, s.MaxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return Blob{}, copyErr
	}
	if closeErr != nil {
		return Blob{}, closeErr
	}
	if n > s.MaxBytes {
		return Blob{}, errors.New("upload exceeds configured limit")
	}
	digest := hex.EncodeToString(h.Sum(nil))
	finalDir := filepath.Join(s.Root, "sha256", digest[:2])
	final := filepath.Join(finalDir, digest)
	if requireJAR {
		if !strings.HasSuffix(strings.ToLower(filename), ".jar") {
			return Blob{}, errors.New("custom mod must use .jar extension")
		}
		if s.MaxJAREntries < 1 {
			return Blob{}, errors.New("invalid JAR entry limit")
		}
		if err := verifyJAR(tmpName, s.MaxJAREntries); err != nil {
			return Blob{}, err
		}
	}
	if err := os.MkdirAll(finalDir, 0750); err != nil {
		return Blob{}, err
	}
	if _, err := os.Stat(final); errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(tmpName, final); err != nil {
			return Blob{}, err
		}
		if err := os.Chmod(final, 0640); err != nil {
			return Blob{}, err
		}
	}
	return Blob{SHA256: digest, Path: final, Filename: filename, MIME: mime, Size: n}, nil
}

func verifyJAR(name string, maxEntries int) error {
	z, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf("invalid JAR/ZIP: %w", err)
	}
	defer z.Close()
	if len(z.File) == 0 || len(z.File) > maxEntries {
		return errors.New("invalid JAR entry count")
	}
	var expanded uint64
	for _, f := range z.File {
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return errors.New("JAR symlink rejected")
		}
		expanded += f.UncompressedSize64
		if expanded > 2<<30 {
			return errors.New("JAR expansion limit exceeded")
		}
	}
	return nil
}
