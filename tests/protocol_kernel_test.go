package model

import (
	"github.com/cedar2025/xboard-node/internal/config"
	"testing"
)

func TestLumaProtocolNeverSilentlyUsesWrongKernel(t *testing.T) {
	for _, n := range []*NodeSpec{{Protocol: "vless", KernelType: "xray"}, {Protocol: "vless", Decryption: "mlkem768x25519plus.native.600s.test"}, {Protocol: "anytls", KernelType: "invalid"}} {
		if e := ValidateNodeSpec(n, config.KernelConfig{Type: "singbox"}); e == nil {
			t.Fatal("incompatible panel configuration silently accepted")
		}
	}
	if e := ValidateNodeSpec(&NodeSpec{Protocol: "vless", KernelType: "xray"}, config.KernelConfig{Type: "xray"}); e != nil {
		t.Fatal(e)
	}
}
