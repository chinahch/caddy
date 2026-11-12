package singbox

import (
	"context"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/model"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singJSON "github.com/sagernet/sing/common/json"
)

func StartProbe(port int, out model.OutboundConfig) (func(), error) {
	cfg := M{"log": M{"disabled": true}, "inbounds": []M{{"type": "socks", "listen": "127.0.0.1", "listen_port": port}}, "outbounds": []M{outboundConfigToSingbox(out)}}
	data, e := json.Marshal(cfg)
	if e != nil {
		return nil, e
	}
	ctx := include.Context(context.Background())
	opts, e := singJSON.UnmarshalExtendedContext[option.Options](ctx, data)
	if e != nil {
		return nil, e
	}
	instance, e := box.New(box.Options{Context: ctx, Options: opts})
	if e != nil {
		return nil, e
	}
	if e = instance.Start(); e != nil {
		instance.Close()
		return nil, e
	}
	return func() { instance.Close() }, nil
}
