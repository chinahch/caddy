package cert

import (
 "context"
 "crypto/tls"
 "crypto/x509"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "time"
 "github.com/caddyserver/certmagic"
)

func(m *Manager)setState(state string){m.state.Store(&state)}
func(m *Manager)State()string{if state:=m.state.Load();state!=nil{return *state};return "none"}

func validateCertificateMaterial(cert,key []byte,domain string)error{
 pair,e:=tls.X509KeyPair(cert,key);if e!=nil||len(pair.Certificate)==0{return errors.New("certificate pair invalid")}
 leaf,e:=x509.ParseCertificate(pair.Certificate[0]);if e!=nil{return errors.New("certificate invalid")}
 now:=time.Now();if now.Before(leaf.NotBefore)||!now.Before(leaf.NotAfter){return errors.New("certificate outside validity period")}
 if strings.HasPrefix(domain,"*."){for _,name:=range leaf.DNSNames{if strings.EqualFold(name,domain){return nil}};return errors.New("certificate does not cover requested wildcard")}
 if domain!=""&&leaf.VerifyHostname(domain)!=nil{return errors.New("certificate domain mismatch")};return nil
}

// Renewal callbacks capture immutable configuration. The generation gate and
// material lock prevent an obsolete issuer from publishing after reconfigure.
func(m *Manager)applyRenewal(ctx context.Context,storage certmagic.Storage,issuer,domain,dir string,generation uint64)error{
 m.materialMu.Lock();defer m.materialMu.Unlock()
 if m.generation.Load()!=generation{return nil}
 cert,e:=storage.Load(ctx,certmagic.StorageKeys.SiteCert(issuer,domain));if e!=nil{return nil}
 key,e:=storage.Load(ctx,certmagic.StorageKeys.SitePrivateKey(issuer,domain));if e!=nil{return nil}
 if validateCertificateMaterial(cert,key,domain)!=nil{return nil}
 m.mat.Store(&certMaterial{certPEM:cert,keyPEM:key})
 if dir!=""&&os.MkdirAll(dir,0700)==nil{_ = atomicWriteFile(filepath.Join(dir,"cert.pem"),cert,0644);_ = atomicWriteFile(filepath.Join(dir,"key.pem"),key,0600)}
 m.renewed.Store(true);m.setState("ready");return nil
}
