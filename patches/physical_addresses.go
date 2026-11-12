package machine

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Never contact an echo endpoint: WARP/routing can report the wrong egress IP.
func physicalAddresses() map[string]any {
	v4, v6 := []string{}, []string{}
	interfaces, e := net.Interfaces()
	if e == nil {
		for _, i := range interfaces {
			physical := false
			if i.Name != "" && !strings.ContainsAny(i.Name, "/\\") {
				_, e := os.Stat(filepath.Join("/sys/class/net", i.Name, "device"))
				physical = e == nil
			}
			addresses, e := i.Addrs()
			if e != nil {
				continue
			}
			for _, a := range addresses {
				prefix, e := netip.ParsePrefix(a.String())
				if e != nil {
					continue
				}
				ip := prefix.Addr().Unmap()
				if !physicalPublicAddress(i.Name, i.Flags, physical, ip) {
					continue
				}
				if ip.Is4() {
					v4 = append(v4, ip.String())
				} else {
					v6 = append(v6, ip.String())
				}
			}
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	return map[string]any{"ipv4": uniqueAddresses(v4), "ipv6": uniqueAddresses(v6), "source": "physical_interface", "detected_at": time.Now().UTC().Format(time.RFC3339)}
}
func uniqueAddresses(in []string) []string {
	out := []string{}
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func physicalPublicAddress(name string, flags net.Flags, physical bool, ip netip.Addr) bool {
	if !physical || flags&net.FlagUp == 0 || flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 || !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	name = strings.ToLower(name)
	for _, prefix := range []string{"lo", "tun", "tap", "wg", "warp", "tailscale", "docker", "veth", "virbr", "br-", "sit", "gre", "ip6tnl"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	return true
}

var nonPublicPrefixes = func() []netip.Prefix {
	out := []netip.Prefix{}
	for _, s := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001::/32", "2001:2::/48", "2001:10::/28", "2001:20::/28", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()
