package xray

import (
	"bytes"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/cedar2025/xboard-node/internal/panel"
	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/infra/conf/serial"
	"testing"
)

func TestLumaLandingStreamReachesPinnedXray(t *testing.T) {
	var p panel.NodeConfig
	e := json.Unmarshal([]byte(`{"custom_outbounds":[{"tag":"landing","protocol":"vless","settings":{"vnext":[{"address":"exit.example","port":443,"users":[{"id":"00000000-0000-4000-8000-000000000001","encryption":"none"}]}]},"stream_settings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"cert.example"},"wsSettings":{"path":"/edge","headers":{"Host":"edge.example"}}}}]}`), &p)
	if e != nil {
		t.Fatal(e)
	}
	spec := model.NodeSpecFromPanel(&p)
	roundtrip := spec.ToPanel()
	spec = model.NodeSpecFromPanel(roundtrip)
	out := outboundConfigToXray(spec.CustomOutbounds[0])
	b, _ := json.Marshal(M{"outbounds": []M{out}})
	cfg, e := serial.DecodeJSONConfig(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	core, e := cfg.Build()
	if e != nil {
		t.Fatal(e)
	}
	sender, e := core.Outbound[0].SenderSettings.GetInstance()
	if e != nil {
		t.Fatal(e)
	}
	stream := sender.(*proxyman.SenderConfig).StreamSettings
	if stream == nil || stream.SecurityType != "xray.transport.internet.tls.Config" {
		t.Fatal("TLS stream was dropped before kernel")
	}
	if stream.ProtocolName != "websocket" {
		t.Fatal("WS path was dropped")
	}
}
