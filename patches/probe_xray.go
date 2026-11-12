package xray

import (
	"bytes"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/model"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
)

// StartProbe starts an isolated loopback client. It never reloads the service.
func StartProbe(port int, out model.OutboundConfig) (func(), error) {
	if out.Protocol == "direct" {
		out.Protocol = "freedom"
	}
	cfg := M{"log": M{"loglevel": "none"}, "inbounds": []M{{"listen": "127.0.0.1", "port": port, "protocol": "socks", "settings": M{"auth": "noauth", "udp": false}}}, "outbounds": []M{outboundConfigToXray(out)}}
	raw, e := json.Marshal(cfg)
	if e != nil {
		return nil, e
	}
	parsed, e := serial.DecodeJSONConfig(bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	built, e := parsed.Build()
	if e != nil {
		return nil, e
	}
	// The pinned core logger is process-global. A temporary client must not
	// replace it with a silent logger, nor close the production log handler.
	apps := built.App[:0]
	for _, app := range built.App {
		if app.GetType() != "xray.app.log.Config" {
			apps = append(apps, app)
		}
	}
	built.App = apps
	// Coordinate the dispatcher's global factory pointer with production Start.
	xrayCreationMu.Lock()
	previous := globalLimitDispatcher.Load()
	instance, e := core.New(built)
	globalLimitDispatcher.Store(previous)
	xrayCreationMu.Unlock()
	if e != nil {
		return nil, e
	}
	if e = instance.Start(); e != nil {
		instance.Close()
		return nil, e
	}
	return func() { instance.Close() }, nil
}
