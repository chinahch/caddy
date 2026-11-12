package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/model"
	"net"
	"testing"
)

func TestLumaFirstUserStartsStoppedKernel(t *testing.T) {
	for _, delta := range []bool{false, true} {
		t.Run(map[bool]string{false: "full", true: "delta"}[delta], func(t *testing.T) {
			k := &fakeKernel{}
			s := newTestService(k)
			s.lastConfig = &model.NodeSpec{Protocol: "vless", ServerPort: 12345}
			u := []model.UserSpec{{ID: 1, UUID: "first", SpeedLimit: 8}}
			if delta {
				s.applyUserDelta(context.Background(), "add", u)
			} else {
				s.applyUserUpdate(context.Background(), u, computeUserHash(u))
			}
			if !k.running || k.startCalls != 1 || len(s.lastUsers) != 1 {
				t.Fatalf("first user did not start: running=%v starts=%d users=%v", k.running, k.startCalls, s.lastUsers)
			}
		})
	}
}
func TestLumaFirstUserFailedStartRestoresState(t *testing.T) {
	k := &fakeKernel{startErr: errors.New("no listener")}
	s := newTestService(k)
	s.lastConfig = &model.NodeSpec{Protocol: "vless"}
	s.applyUserUpdate(context.Background(), []model.UserSpec{{ID: 1, UUID: "first", SpeedLimit: 8}}, "")
	if k.startCalls != 1 || len(s.lastUsers) != 0 || s.speedTracker.GetLimiter("first") != nil {
		t.Fatal("first-user start not attempted or state not restored")
	}
}

func TestLumaFirstUserStartsRealNativeListener(t *testing.T) {
	for _, kind := range []string{"xray", "singbox"} {
		t.Run(kind, func(t *testing.T) {
			l, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			port := l.Addr().(*net.TCPAddr).Port
			l.Close()
			s := NewWithControlPlane(&config.Config{Kernel: config.KernelConfig{Type: kind, LogLevel: "error", ConfigDir: t.TempDir()}}, nil)
			defer s.kernel.Stop()
			s.lastConfig = &model.NodeSpec{Protocol: "vless", ListenIP: "127.0.0.1", ServerPort: port, Network: "tcp", Decryption: "none"}
			s.applyUserUpdate(context.Background(), []model.UserSpec{{ID: 1, UUID: "00000000-0000-4000-8000-000000000001"}}, "")
			c, e := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if e != nil || !s.kernel.IsRunning() {
				t.Fatal("first real user did not open native listener", e)
			}
			c.Close()
		})
	}
}
