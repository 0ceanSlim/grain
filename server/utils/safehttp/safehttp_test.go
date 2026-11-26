package safehttp

import (
	"net"
	"testing"
)

func TestPublicOnlyRefusesNonPublicRanges(t *testing.T) {
	for _, a := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "100.100.100.100", "198.18.0.1",
		"192.0.0.8", "240.0.0.1", "0.1.2.3", "::1", "fd00::1", "64:ff9b::a00:1", "2002:a00:1::1", "::ffff:127.0.0.1"} {
		if err := PublicOnly("tcp", net.JoinHostPort(a, "443"), nil); err == nil {
			t.Errorf("%s: allowed", a)
		}
	}
	for _, a := range []string{"93.184.216.34", "2606:4700::6810:84e5"} {
		if err := PublicOnly("tcp", net.JoinHostPort(a, "443"), nil); err != nil {
			t.Errorf("%s: refused: %v", a, err)
		}
	}
}
