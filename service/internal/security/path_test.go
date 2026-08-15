package security

import "testing"

func TestSafeRelative(t *testing.T) {
	for _, bad := range []string{"", "../x", "/etc/passwd", `a\..\b`, "a\x00b"} {
		if _, err := SafeRelative(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if got, err := SafeRelative("config/app.toml"); err != nil || got != "config/app.toml" {
		t.Fatalf("got %q %v", got, err)
	}
}
