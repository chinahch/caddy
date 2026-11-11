package main

import (
 "os"
 "path/filepath"
 "reflect"
 "testing"
)

func TestCaddyKernelSettingsSurviveCLIRewrite(t *testing.T){
 path:=filepath.Join(t.TempDir(),"config.yml")
 input:=`kernel:
  custom_config_singbox: /etc/caddy/singbox.json
  custom_config_xray: /etc/caddy/xray.json
  ip_family:
    singbox: ipv4_only
    xray: ipv6_only
instances:
  - id: existing
    panel:
      url: https://panel.example.invalid
      node_id: 1
    kernel:
      type: xray
      custom_config_singbox: /etc/caddy/instance-sb.json
      custom_config_xray: /etc/caddy/instance-xr.json
      ip_family:
        xray: ipv4_only
`
 if err:=os.WriteFile(path,[]byte(input),0600);err!=nil{t.Fatal(err)}
 before,err:=loadWritableRootConfig(path);if err!=nil{t.Fatal(err)}
 if err=writeRootConfig(path,before);err!=nil{t.Fatal(err)}
 after,err:=loadWritableRootConfig(path);if err!=nil{t.Fatal(err)}
 if !reflect.DeepEqual(before.Config.Kernel,after.Config.Kernel){t.Fatalf("shared kernel settings lost: before=%+v after=%+v",before.Config.Kernel,after.Config.Kernel)}
 if !reflect.DeepEqual(before.Instances[0].Kernel,after.Instances[0].Kernel){t.Fatalf("instance kernel settings lost: before=%+v after=%+v",before.Instances[0].Kernel,after.Instances[0].Kernel)}
}
