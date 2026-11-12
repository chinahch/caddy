package singbox

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/kernel"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/cedar2025/xboard-node/internal/panel"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singJSON "github.com/sagernet/sing/common/json"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestLumaNativePresetsPinnedSingbox(t *testing.T) {
	path := os.Getenv("LUMA_PROTOCOL_FIXTURES")
	if path == "" {
		t.Skip("Luma fixture file required")
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var fixtures []map[string]json.RawMessage
	if e = json.Unmarshal(data, &fixtures); e != nil {
		t.Fatal(e)
	}
	tc := lumaTestCert(t)
	for _, fixture := range fixtures {
		var core, name string
		json.Unmarshal(fixture["kernel_type"], &core)
		json.Unmarshal(fixture["fixture_name"], &name)
		if core != "singbox" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(fixture)
			var nc panel.NodeConfig
			if e := json.Unmarshal(b, &nc); e != nil {
				t.Fatal(e)
			}
			spec, e := model.NodeSpecFromPanelValidated(&nc, config.KernelConfig{Type: "singbox"})
			if e != nil {
				t.Fatal(e)
			}
			var outbound panel.OutboundConfig
			if e = json.Unmarshal(fixture["fixture_outbound"], &outbound); e != nil {
				t.Fatal(e)
			}
			nc.CustomOutbounds = []panel.OutboundConfig{outbound}
			spec = model.NodeSpecFromPanel(&nc)
			cfg := buildConfig(config.KernelConfig{Type: "singbox", LogLevel: "warn"}, spec, []model.UserSpec{{ID: 1, UUID: "00000000-0000-4000-8000-000000000001"}}, tc)
			b, _ = json.Marshal(cfg)
			ctx := include.Context(context.Background())
			opts, e := singJSON.UnmarshalExtendedContext[option.Options](ctx, b)
			if e != nil {
				t.Fatal(e)
			}
			instance, e := box.New(box.Options{Context: ctx, Options: opts})
			if e != nil {
				t.Fatal(e)
			}
			instance.Close()
		})
	}
}

func lumaTestCert(t *testing.T) kernel.TLSCert {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "cert.example"}, DNSNames: []string{"cert.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	private, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return kernel.TLSCert{CertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), KeyPEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})}
}
