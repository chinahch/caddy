package xray

import (
	"github.com/cedar2025/xboard-node/internal/model"
	xlog "github.com/xtls/xray-core/common/log"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func isolationPort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}
func TestProbeCreationUsesProductionCreationLock(t *testing.T) {
	port := isolationPort(t)
	type result struct {
		close func()
		err   error
	}
	ready := make(chan result, 1)
	xrayCreationMu.Lock()
	go func() {
		c, e := StartProbe(port, model.OutboundConfig{Tag: "probe", Protocol: "direct"})
		ready <- result{c, e}
	}()
	early := false
	var r result
	select {
	case r = <-ready:
		early = true
	case <-time.After(100 * time.Millisecond):
	}
	xrayCreationMu.Unlock()
	if !early {
		select {
		case r = <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("probe creation stuck")
		}
	}
	if r.close != nil {
		r.close()
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	if early {
		t.Fatal("probe bypassed production creation lock")
	}
}

type probeLogSentinel struct{ markers atomic.Int64 }

func (s *probeLogSentinel) Handle(m xlog.Message) {
	if g, ok := m.(*xlog.GeneralMessage); ok && g.Content == "production-log-marker" {
		s.markers.Add(1)
	}
}
func TestProbePreservesProductionLogHandler(t *testing.T) {
	sink := &probeLogSentinel{}
	xlog.RegisterHandler(sink)
	xlog.Record(&xlog.GeneralMessage{Content: "production-log-marker"})
	close, e := StartProbe(isolationPort(t), model.OutboundConfig{Tag: "probe", Protocol: "direct"})
	if e != nil {
		t.Fatal(e)
	}
	xlog.Record(&xlog.GeneralMessage{Content: "production-log-marker"})
	close()
	xlog.Record(&xlog.GeneralMessage{Content: "production-log-marker"})
	if sink.markers.Load() != 3 {
		t.Fatal("probe replaced or closed production log handler", sink.markers.Load())
	}
}
