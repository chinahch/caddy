package cert

import (
 "context"
 "crypto/ecdsa"
 "crypto/elliptic"
 "crypto/rand"
 "crypto/x509"
 "encoding/pem"
 "errors"
 "math/big"
 "strings"
 "sync/atomic"
 "testing"
 "time"

 "github.com/caddyserver/certmagic"
 "github.com/cedar2025/xboard-node/internal/config"
)

type localCertificateIssuer struct{calls atomic.Int32; fail atomic.Bool}
func(i *localCertificateIssuer)IssuerKey()string{return "local-unit-test-issuer"}
func(i *localCertificateIssuer)Issue(ctx context.Context,csr *x509.CertificateRequest)(*certmagic.IssuedCertificate,error){
 if ctx.Err()!=nil{return nil,ctx.Err()};n:=i.calls.Add(1)
 if i.fail.Load(){return nil,errors.New("private-test-api-token must never be logged")}
 key,e:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if e!=nil{return nil,e}
 tmpl:=&x509.Certificate{SerialNumber:big.NewInt(int64(n)),DNSNames:csr.DNSNames,NotBefore:time.Now().Add(-time.Hour),NotAfter:time.Now().Add(90*24*time.Hour),KeyUsage:x509.KeyUsageDigitalSignature,ExtKeyUsage:[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
 der,e:=x509.CreateCertificate(rand.Reader,tmpl,tmpl,csr.PublicKey,key);if e!=nil{return nil,e}
 return &certmagic.IssuedCertificate{Certificate:pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE",Bytes:der})},nil
}
func localManager(t *testing.T,dir string,issuer *localCertificateIssuer)*Manager{
 t.Helper();m:=NewManager(config.CertConfig{CertMode:"dns",Domain:"node.example.test",Email:"admin@example.test",DNSProvider:"cloudflare",DNSEnv:map[string]string{"CF_API_TOKEN":"unit-test-only"},CertDir:dir})
 m.issuerFactory=func(*certmagic.Config)certmagic.Issuer{return issuer};t.Cleanup(m.Stop);return m
}
func TestLumaCertificateIssueReuseRenewAndStop(t *testing.T){
 ctx:=context.Background();issuer:=&localCertificateIssuer{};dir:=t.TempDir();m:=localManager(t,dir,issuer)
 if err:=m.Start(ctx);err!=nil{t.Fatal(err)}
 if !m.HasCert()||m.State()!="ready"||issuer.calls.Load()!=1{t.Fatal("first issue did not publish material")}
 first:=string(m.TLSCert().CertPEM)
 if _,err:=m.Reconfigure(ctx,m.cfg);err!=nil||issuer.calls.Load()!=1{t.Fatal("unchanged sync reissued",err)}
 if err:=m.magic.RenewCertSync(ctx,m.cfg.Domain,true);err!=nil{t.Fatal(err)}
 if string(m.TLSCert().CertPEM)==first||!m.CertRenewed(){t.Fatal("renewal not applied")}
 oldMagic:=m.magic;m.Stop();m.Stop()
 if m.cache!=nil||m.acmeCancel!=nil||m.HasCert(){t.Fatal("stop retained live certificate worker")}
 if err:=oldMagic.OnEvent(ctx,"cert_obtained",map[string]any{"renewal":true,"issuer":issuer.IssuerKey()});err!=nil{t.Fatal(err)}
 if m.HasCert(){t.Fatal("stopped generation published stale material")}
 restarted:=localManager(t,dir,issuer);if err:=restarted.Start(ctx);err!=nil{t.Fatal(err)}
 if !restarted.HasCert()||issuer.calls.Load()!=2{t.Fatal("restart issued instead of using cached certificate",issuer.calls.Load())}
}
func TestLumaCertificateFailedApplyCanRetryAndDoesNotReuseWrongDomain(t *testing.T){
 issuer:=&localCertificateIssuer{};m:=localManager(t,t.TempDir(),issuer);ctx:=context.Background()
 if err:=m.Start(ctx);err!=nil{t.Fatal(err)}
 next:=m.cfg;next.Domain="different.example.test";issuer.fail.Store(true)
 _,err:=m.Reconfigure(ctx,next)
 if err==nil||strings.Contains(err.Error(),"private-test-api-token")||m.HasCert()||m.State()!="failed"{t.Fatal("failed issuance leaked details or retained wrong certificate",err)}
 issuer.fail.Store(false);if _,err=m.Reconfigure(ctx,next);err!=nil||!m.HasCert()||m.State()!="ready"{t.Fatal("same failed configuration cannot retry",err)}
 pair,_:=x509.ParseCertificate(mustLeaf(t,m.TLSCert().CertPEM));if pair.VerifyHostname(next.Domain)!=nil{t.Fatal("wrong domain after recovery")}
}
func mustLeaf(t *testing.T,b []byte)[]byte{t.Helper();p,_:=pem.Decode(b);if p==nil{t.Fatal("missing PEM")};return p.Bytes}
func TestLumaCertificateManagersDoNotShareMaterial(t *testing.T){
 issuer:=&localCertificateIssuer{};a:=localManager(t,t.TempDir(),issuer);b:=localManager(t,t.TempDir(),issuer)
 if err:=a.Start(context.Background());err!=nil{t.Fatal(err)};if err:=b.Start(context.Background());err!=nil{t.Fatal(err)}
 original:=string(b.TLSCert().CertPEM);a.Stop();if string(b.TLSCert().CertPEM)!=original||!b.HasCert()||issuer.calls.Load()!=2{t.Fatal("one node affected another node certificate")}
}
