package xray
import("bytes";"log/slog";"os";"strings";"testing"
 "github.com/cedar2025/xboard-node/internal/config"
 "github.com/cedar2025/xboard-node/internal/kernel"
 "github.com/cedar2025/xboard-node/internal/model"
 "github.com/cedar2025/xboard-node/internal/nlog"
)
func TestMalformedPrivateConfigsNeverEnterErrorsOrLogs(t *testing.T){
 for _,tc:=range []struct{name string;node model.NodeSpec}{
 {"ENC",model.NodeSpec{Protocol:"vless",ServerPort:18443,Network:"tcp",Decryption:"mlkem768x25519plus.invalid.600s.sentinel-sensitive-decryption"}},
 {"REALITY",model.NodeSpec{Protocol:"vless",ServerPort:18443,Network:"tcp",TLS:2,TLSSettings:map[string]any{"private_key":"sentinel-sensitive-reality","server_name":"cert.example","dest":"127.0.0.1:443","short_id":"abcd"}}},
 }{t.Run(tc.name,func(t *testing.T){var logs bytes.Buffer;nlog.Init(&logs,slog.LevelDebug,false);defer nlog.Init(os.Stdout,slog.LevelInfo,false);x:=New(config.KernelConfig{Type:"xray",LogLevel:"error"});for _,op:=range []func(*model.NodeSpec,[]model.UserSpec,kernel.TLSCert)error{x.Start,x.Reload}{e:=op(&tc.node,nil,kernel.TLSCert{});if e==nil{t.Fatal("malformed configuration accepted")};nlog.Core().Error("kernel configuration failed","error",e);if strings.Contains(e.Error(),"sentinel-sensitive")||strings.Contains(logs.String(),"sentinel-sensitive"){t.Fatal("private configuration escaped through error/log boundary")}}})}
}
