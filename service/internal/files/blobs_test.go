package files

import (
	"archive/zip"
	"bytes"
	"fmt"
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

func TestJAREntryLimit(t *testing.T) {
	jar := testJAR(t, 3)
	rejected := Store{Root: t.TempDir(), MaxBytes: 1 << 20, MaxJAREntries: 2}
	if _, err := rejected.Put(bytes.NewReader(jar), "test.jar", "application/java-archive", true); err == nil || err.Error() != "invalid JAR entry count" {
		t.Fatalf("expected JAR entry count rejection, got %v", err)
	}

	accepted := Store{Root: t.TempDir(), MaxBytes: 1 << 20, MaxJAREntries: 3}
	if _, err := accepted.Put(bytes.NewReader(jar), "test.jar", "application/java-archive", true); err != nil {
		t.Fatalf("JAR at configured entry limit rejected: %v", err)
	}
}

func testJAR(t *testing.T, entries int) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for i := 0; i < entries; i++ {
		w, err := z.Create(fmt.Sprintf("entry-%d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
