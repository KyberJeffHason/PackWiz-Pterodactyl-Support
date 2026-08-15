package security

import (
	"net"
	"net/url"
	"testing"
)

func TestPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1", "224.0.0.1"} {
		if PublicIP(net.ParseIP(s)) {
			t.Errorf("accepted %s", s)
		}
	}
	if !PublicIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public IP rejected")
	}
}
func TestValidateRemoteURL(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1/x")
	if ValidateRemoteURL(u) == nil {
		t.Fatal("loopback accepted")
	}
}
