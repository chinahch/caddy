package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"time"
)

type pingConn interface {
	ReadFrom([]byte) (int, net.Addr, error)
	WriteTo([]byte, net.Addr) (int, error)
	SetDeadline(time.Time) error
	Close() error
}

// ICMP is a separate observation after the authenticated proxy roundtrip.
// It uses no subprocess, shell, remote command, or unverified echo service.
func pingRTT(parent context.Context, host string) (*float64, string) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	addresses, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil || len(addresses) == 0 {
		return nil, "icmp_resolve_failed"
	}
	ip := addresses[0].IP
	for _, a := range addresses {
		if a.IP.To4() != nil {
			ip = a.IP
			break
		}
	}
	network, bind := "ip4:icmp", "0.0.0.0"
	if ip.To4() == nil {
		network, bind = "ip6:ipv6-icmp", "::"
	}
	conn, e := icmp.ListenPacket(network, bind)
	if e != nil {
		return nil, "icmp_unavailable"
	}
	defer conn.Close()
	return pingSocket(ctx, conn, ip)
}
func pingSocket(ctx context.Context, conn pingConn, ip net.IP) (*float64, string) {
	random := make([]byte, 18)
	if _, e := rand.Read(random); e != nil {
		return nil, "icmp_unavailable"
	}
	id := int(binary.BigEndian.Uint16(random[:2]))
	nonce := random[2:]
	var echo, reply icmp.Type = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	protocol := 1
	if ip.To4() == nil {
		echo, reply, protocol = ipv6.ICMPTypeEchoRequest, ipv6.ICMPTypeEchoReply, 58
	}
	var total time.Duration
	received := 0
	buf := make([]byte, 1500)
	for seq := 1; seq <= 3; seq++ {
		if ctx.Err() != nil {
			break
		}
		deadline := time.Now().Add(time.Second)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		if conn.SetDeadline(deadline) != nil {
			return nil, "icmp_unavailable"
		}
		data, e := (&icmp.Message{Type: echo, Body: &icmp.Echo{ID: id, Seq: seq, Data: nonce}}).Marshal(nil)
		if e != nil {
			return nil, "icmp_unavailable"
		}
		started := time.Now()
		if _, e = conn.WriteTo(data, &net.IPAddr{IP: ip}); e != nil {
			return nil, "icmp_unavailable"
		}
		for {
			if ctx.Err() != nil {
				break
			}
			n, peer, e := conn.ReadFrom(buf)
			if e != nil {
				break
			}
			addr, ok := peer.(*net.IPAddr)
			if !ok || !addr.IP.Equal(ip) {
				continue
			}
			m, e := icmp.ParseMessage(protocol, buf[:n])
			if e != nil || m.Type != reply {
				continue
			}
			body, ok := m.Body.(*icmp.Echo)
			if !ok || body.ID != id || body.Seq != seq || !bytes.Equal(body.Data, nonce) {
				continue
			}
			total += time.Since(started)
			received++
			break
		}
	}
	if received == 0 {
		return nil, "icmp_timeout"
	}
	average := float64(total) / float64(time.Millisecond) / float64(received)
	return &average, ""
}
