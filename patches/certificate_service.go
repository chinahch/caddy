package service

import (
 "crypto/tls"
 "crypto/x509"
 "time"
 "github.com/cedar2025/xboard-node/internal/model"
)

// Fixed states only: provider errors and credentials never leave this method.
func(s *Service)ProbeCertificateState()string{return s.cert.State()}
func(s *Service)certificateReady(n *model.NodeSpec)bool{
 if n==nil||n.TLS!=1{return true}
 material:=s.cert.TLSCert();pair,e:=tls.X509KeyPair(material.CertPEM,material.KeyPEM);if e!=nil||len(pair.Certificate)==0{return false}
 leaf,e:=x509.ParseCertificate(pair.Certificate[0]);if e!=nil||time.Now().Before(leaf.NotBefore)||!time.Now().Before(leaf.NotAfter){return false}
 sni:=n.ServerName;if n.TLSSettings!=nil{if name,ok:=n.TLSSettings["server_name"].(string);ok&&name!=""{sni=name}}
 if sni==""&&n.CertConfig!=nil{sni=n.CertConfig.Domain}
 return sni!=""&&leaf.VerifyHostname(sni)==nil
}
