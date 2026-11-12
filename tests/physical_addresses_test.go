package machine

import (
	"net"
	"net/netip"
	"testing"
)

func TestPhysicalPublicAddressesExcludeWarpVirtualAndNAT(t *testing.T) {
	cases := []struct {
		name, ip string
		physical bool
		flags    net.Flags
		want     bool
	}{{"eth0", "8.8.8.8", true, net.FlagUp, true}, {"ens3", "2606:4700:4700::1111", true, net.FlagUp, true}, {"warp", "8.8.8.8", true, net.FlagUp, false}, {"wg0", "2606:4700:4700::1111", false, net.FlagUp, false}, {"eth0", "8.8.8.8", false, net.FlagUp, false}, {"eth0", "192.168.1.2", true, net.FlagUp, false}, {"eth0", "100.64.1.2", true, net.FlagUp, false}, {"eth0", "203.0.113.2", true, net.FlagUp, false}, {"eth0", "2001:db8::1", true, net.FlagUp, false}, {"eth0", "fd00::1", true, net.FlagUp, false}, {"eth0", "8.8.8.8", true, net.FlagUp | net.FlagPointToPoint, false}, {"eth0", "8.8.8.8", true, 0, false}}
	for _, c := range cases {
		if got := physicalPublicAddress(c.name, c.flags, c.physical, netip.MustParseAddr(c.ip)); got != c.want {
			t.Errorf("%s %s physical=%v got=%v", c.name, c.ip, c.physical, got)
		}
	}
}
