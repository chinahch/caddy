package singbox

import (
	"context"
	"encoding/json"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singJSON "github.com/sagernet/sing/common/json"
	"testing"
)

func TestLumaLandingChainUsesPinnedSingboxDetour(t *testing.T) {
	out := outboundConfigToSingbox(model.OutboundConfig{Tag: "landing", Protocol: "socks", ProxyTag: "upstream", Settings: map[string]any{"server": "exit.example", "server_port": 1080, "version": "5"}})
	b, _ := json.Marshal(out)
	opts, e := singJSON.UnmarshalExtendedContext[option.Outbound](include.Context(context.Background()), b)
	if e != nil {
		t.Fatal(e)
	}
	s := opts.Options.(*option.SOCKSOutboundOptions)
	if s.Detour != "upstream" {
		t.Fatal("proxy chain ignored")
	}
}
