package service

import(
 "context"
 "encoding/json"
 "os"
 "testing"
 "github.com/cedar2025/xboard-node/internal/config"
 "github.com/cedar2025/xboard-node/internal/controlplane"
 "github.com/cedar2025/xboard-node/internal/model"
 "github.com/cedar2025/xboard-node/internal/panel"
)
type certificateFixtureSource struct{bootstrap controlplane.Bootstrap}
func(f certificateFixtureSource)Initial(context.Context,func()map[string]interface{},chan<-controlplane.Event,chan<-controlplane.StatusChange)(controlplane.Bootstrap,error){return f.bootstrap,nil}
func(f certificateFixtureSource)Poll(context.Context)(controlplane.Snapshot,error){return controlplane.Snapshot{},nil}
func(f certificateFixtureSource)Discover(context.Context,func()map[string]interface{},chan<-controlplane.Event,chan<-controlplane.StatusChange)(controlplane.PushClient,error){return nil,nil}
func(f certificateFixtureSource)Metrics()controlplane.APIMetrics{return controlplane.APIMetrics{}}
func(f certificateFixtureSource)SupportsPolling()bool{return true}
func(f certificateFixtureSource)SupportsDiscovery()bool{return false}
type anyTLSFixtureKernel struct{*fakeKernel}
func(*anyTLSFixtureKernel)Protocols()[]string{return []string{"anytls"}}
func TestGeneratedAnyTLSRealityStartsServiceWithoutOrdinaryCertificate(t *testing.T){
 path:=os.Getenv("LUMA_PROTOCOL_FIXTURES");if path==""{t.Skip("generated protocol fixtures required")}
 raw,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)};var fixtures []json.RawMessage;if json.Unmarshal(raw,&fixtures)!=nil{t.Fatal("invalid fixtures")}
 var spec *model.NodeSpec
 for _,raw:=range fixtures{var cfg struct{Preset string `json:"preset"`};json.Unmarshal(raw,&cfg);if cfg.Preset=="anytls-reality"{var p panel.NodeConfig;if json.Unmarshal(raw,&p)!=nil{t.Fatal("invalid native fixture")};spec=model.NodeSpecFromPanel(&p);break}}
 if spec==nil{t.Fatal("AnyTLS REALITY fixture missing")}
 k:=&fakeKernel{};s:=newTestService(k);s.kernel=&anyTLSFixtureKernel{k};s.cfg=&config.Config{Kernel:config.KernelConfig{Type:"singbox"}}
 s.source=certificateFixtureSource{controlplane.Bootstrap{Config:spec,Users:[]model.UserSpec{{ID:1,UUID:"00000000-0000-4000-8000-000000000001"}}}}
 if s.cert.HasCert(){t.Fatal("fixture must start without ordinary certificate")}
 if err=s.initialSetup(context.Background());err!=nil{t.Fatal("generated AnyTLS REALITY failed Service startup",err)}
 if !k.running||k.startCalls!=1{t.Fatal("service did not start")}
}
func TestRemovingPanelTLSStopsObsoleteCertificateManager(t *testing.T){
 k:=&fakeKernel{};s:=newTestService(k);s.cfg=&config.Config{Cert:config.CertConfig{CertDir:t.TempDir()}}
 tls:=&model.NodeSpec{Protocol:"vless",TLS:1,ServerName:"old.example.test",CertConfig:&config.CertConfig{CertMode:"self",Domain:"old.example.test"}}
 s.applyRemoteOverrides(context.Background(),tls);if !s.cert.HasCert(){t.Fatal("local TLS fixture not ready")}
 if !s.startKernel(tls,[]model.UserSpec{{ID:1,UUID:"test"}}){t.Fatal("fixture service did not start")}
 reality:=&model.NodeSpec{Protocol:"vless",TLS:2,TLSSettings:map[string]any{"server_name":"addons.mozilla.org","private_key":"test"}}
 s.applyRemoteOverrides(context.Background(),reality)
 if s.cert.HasCert()||s.cert.State()!="none"||s.cfg.Cert.CertMode!="none"{t.Fatal("obsolete panel certificate state retained after protocol switch")}
}
func TestNoPanelCertificateKeepsIntentionalLegacyLocalTLS(t *testing.T){
 k:=&fakeKernel{};s:=newTestService(k);local:=config.CertConfig{CertMode:"self",Domain:"legacy.example.test",CertDir:t.TempDir()};s.cfg=&config.Config{Cert:local}
 if _,err:=s.cert.Reconfigure(context.Background(),local);err!=nil{t.Fatal(err)}
 before:=string(s.cert.TLSCert().CertPEM)
 s.applyRemoteOverrides(context.Background(),&model.NodeSpec{Protocol:"vless",TLS:1,ServerName:"legacy.example.test"})
 if !s.cert.HasCert()||string(s.cert.TLSCert().CertPEM)!=before{t.Fatal("legacy local TLS was cleared without a panel-managed override")}
}
