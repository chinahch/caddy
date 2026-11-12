from pathlib import Path
import shutil
root=Path(__file__).resolve().parents[1]
def edit(path,old,new):
 p=Path(path);s=p.read_text(encoding='utf-8')
 if s.count(old)!=1: raise ValueError('probe source anchor changed: '+path)
 p.write_text(s.replace(old,new,1),encoding='utf-8',newline='\n')
for name,dest in [('probe_diagnostics','internal/probe/diagnostics.go'),('probe','internal/probe/probe.go'),('probe_panel','internal/panel/caddy_probes.go'),('probe_machine','internal/machine/caddy_probes.go'),('probe_xray','internal/kernel/xray/caddy_probes.go'),('probe_singbox','internal/kernel/singbox/caddy_probes.go')]:
 p=Path(dest);p.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(root/f'patches/{name}.go',p)
edit('internal/machine/machine.go','\to.runCtx = ctx','\to.runCtx = ctx\n probeCtx, stopProbes := context.WithCancel(ctx)\n probeDone:=make(chan struct{})\n go func(){defer close(probeDone);o.runProbes(probeCtx)}()\n defer func(){stopProbes();<-probeDone}()')
edit('internal/service/service.go', """func (s *Service) applyUserUpdate(ctx context.Context, users []model.UserSpec, newHash string) {
	if !s.ensureRunning() {
		return
	}

	prevUsers, prevHash := s.prepareUserState(users)""", """func (s *Service) applyUserUpdate(ctx context.Context, users []model.UserSpec, newHash string) {
 prevUsers, prevHash := s.prepareUserState(users)
 if !s.kernel.IsRunning() {
  if len(users)>0 && s.lastConfig!=nil && !s.startKernel(s.lastConfig,users) {s.restoreUserState(prevUsers,prevHash)}
  return
 }""")
edit('internal/service/service.go', """		merged := mergeUsers(s.lastUsers, deltaUsers)

		if !s.ensureRunning() {
			return
		}""", """		merged := mergeUsers(s.lastUsers, deltaUsers)
  if !s.kernel.IsRunning() {s.applyUserUpdate(ctx,merged,computeUserHash(merged));return}""")
shutil.copyfile(root/'tests/first_user_test.go','internal/service/caddy_first_user_test.go')
shutil.copyfile(root/'tests/probe_runtime_test.go','internal/probe/probe_test.go')

shutil.copyfile(root/'tests/probe_panel_test.go','internal/panel/caddy_probes_test.go')

edit('internal/machine/machine.go','type nodeHandle struct {','type nodeHandle struct {\n service *service.Service')
edit('internal/machine/machine.go','svc := service.NewWithControlPlane(nodeCfg, cp)','svc := service.NewWithControlPlane(nodeCfg, cp)\n o.mu.Lock();if h,ok:=o.nodes[mn.ID];ok{h.service=svc};o.mu.Unlock()')
p=Path('internal/service/caddy_probe_status.go');p.write_text('package service\n// ProbeKernelRunning observes the kernel atomic lifecycle flag.\nfunc(s *Service)ProbeKernelRunning()bool{return s.kernel.IsRunning()}\n',encoding='utf-8',newline='\n')
shutil.copyfile(root/'tests/probe_diagnostics_test.go','internal/probe/diagnostics_test.go')

shutil.copyfile(root/'patches/physical_addresses.go','internal/machine/caddy_physical_addresses.go')
shutil.copyfile(root/'tests/physical_addresses_test.go','internal/machine/caddy_physical_addresses_test.go')
edit('internal/panel/client.go','netIn, netOut float64) error {','netIn, netOut float64, physical ...map[string]any) error {')
edit('internal/panel/client.go','return c.postJSON("/api/v2/server/machine/status", payload)','if len(physical)>0 {payload["public_addresses"]=physical[0]}\n return c.postJSON("/api/v2/server/machine/status", payload)')
edit('internal/machine/machine.go','s.NetInSpeed, s.NetOutSpeed,','s.NetInSpeed, s.NetOutSpeed,physicalAddresses(),')

shutil.copyfile(root/'tests/probe_isolation_test.go','internal/kernel/xray/caddy_probe_isolation_test.go')

shutil.copyfile(root/'patches/probe_ping.go','internal/probe/ping.go')
shutil.copyfile(root/'tests/probe_ping_test.go','internal/probe/ping_test.go')
