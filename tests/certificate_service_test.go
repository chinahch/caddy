package service

import (
 "context"
 "testing"
 "github.com/cedar2025/xboard-node/internal/config"
 "github.com/cedar2025/xboard-node/internal/model"
)

func TestLumaCertificateFailureDoesNotStartPlainTLSOrBecomeApplied(t *testing.T){
 k:=&fakeKernel{};s:=newTestService(k);s.cfg=&config.Config{}
 node:=&model.NodeSpec{Protocol:"vless",TLS:1,ServerPort:24443,ServerName:"edge.example.test",CertConfig:&config.CertConfig{CertMode:"content",Domain:"edge.example.test",CertContent:"bad cert",KeyContent:"private-do-not-log"}}
 s.lastConfig=node;s.lastConfigHash="before";s.updateUserState([]model.UserSpec{{ID:1,UUID:"test"}})
 s.applyPullResult(context.Background(),pullResult{config:node,configHash:"failed-hash"})
 if k.running||k.startCalls!=0{t.Fatal("TLS node started without valid material")}
 if s.lastConfigHash=="failed-hash"{t.Fatal("failed certificate applied hash prevents retry")}
 if s.ProbeCertificateState()!="failed"{t.Fatal("missing fixed failure state")}
}
