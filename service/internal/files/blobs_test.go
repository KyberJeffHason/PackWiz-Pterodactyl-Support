package files

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestBlobStoreHashAndLimit(t *testing.T) {
	s := Store{Root: t.TempDir(), MaxBytes: 10}
	b, err := s.Put(strings.NewReader("hello"), "x.txt", "text/plain", false)
	if err != nil || b.SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("%+v %v", b, err)
	}
	if _, err := os.Stat(b.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(bytes.NewReader(make([]byte, 11)), "x", "", false); err == nil {
		t.Fatal("large upload accepted")
	}
}
