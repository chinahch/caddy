"""Apply bounded certificate lifecycle fixes to the pinned native source."""
from pathlib import Path
import shutil
root=Path(__file__).resolve().parents[1]
def edit(path,old,new):
 p=Path(path);s=p.read_text(encoding='utf-8')
 if s.count(old)!=1: raise ValueError('certificate anchor changed: '+path+' '+old[:70])
 p.write_text(s.replace(old,new,1),encoding='utf-8',newline='\n')

p='internal/cert/cert.go'
edit(p,'"sync/atomic"','"sync/atomic"\n "sync"\n "go.uber.org/zap"')
edit(p,'type Manager struct {\n\tcfg config.CertConfig','type Manager struct {\n\tcfg config.CertConfig\n state atomic.Pointer[string]\n materialMu sync.Mutex\n generation atomic.Uint64\n cache *certmagic.Cache\n issuerFactory func(*certmagic.Config)certmagic.Issuer')
old='''	if m.acmeStarted {
		newFp := acmeFingerprint(newCfg)
		newMode := resolveModeFor(newCfg)
		if newMode != "http" && newMode != "dns" || newFp != m.acmeFingerprint {
			m.tearDownACME()
		}
	}'''
edit(p,old,''' if acmeFingerprint(newCfg)!=acmeFingerprint(m.cfg)||resolveModeFor(newCfg)!=resolveModeFor(m.cfg){m.tearDownACME()}''')
edit(p,'func (m *Manager) tearDownACME() {','func (m *Manager) tearDownACME() {\n m.materialMu.Lock();m.generation.Add(1);m.mat.Store(nil);m.materialMu.Unlock()\n if m.cache!=nil{m.cache.Stop();m.cache=nil}')
edit(p,'b.WriteString(strings.TrimSpace(cfg.DNSProvider))','b.WriteString(strings.TrimSpace(cfg.DNSProvider))\n b.WriteString("|"+cfg.CertFile+"|"+cfg.KeyFile+"|"+cfg.CertContent+"|"+cfg.KeyContent)')
edit(p,'''func (m *Manager) Start(ctx context.Context) error {
	mode := m.resolveMode()''','''func (m *Manager) Start(ctx context.Context) (err error) {
 mode:=m.resolveMode();m.setState("pending")
 defer func(){
  if err==nil && mode!="none" && mode!="" {material:=m.TLSCert();err=validateCertificateMaterial(material.CertPEM,material.KeyPEM,m.cfg.Domain)}
  if err!=nil {m.mat.Store(nil);m.setState("failed");err=fmt.Errorf("certificate setup failed (%s); verify domain, credentials, files and ACME network access",mode)} else if mode=="none"||mode==""{m.setState("none")}else{m.setState("ready")}
 }()''')
edit(p,'func (m *Manager) Stop() {}','func (m *Manager) Stop() {m.tearDownACME();m.setState("none")}')
edit(p,'var magic *certmagic.Config','var magic *certmagic.Config\n domain,certDir,generation:=m.cfg.Domain,m.cfg.CertDir,m.generation.Load()')
edit(p,'cache := certmagic.NewCache(certmagic.CacheOptions{','cache := certmagic.NewCache(certmagic.CacheOptions{\n Logger:zap.NewNop(),')
edit(p,'Storage: storage,','Storage: storage,\n Logger:zap.NewNop(),')
start='''			if err := m.loadPEMFromStorage(evtCtx, storage, issuerKey, m.cfg.Domain); err != nil {
				nlog.Core().Error("failed to reload cert after renewal", "error", err)
				return nil
			}
			m.renewed.Store(true)
			nlog.Core().Info("TLS certificate reloaded after renewal", "domain", m.cfg.Domain)
			return nil'''
edit(p,start,'return m.applyRenewal(evtCtx,storage,issuerKey,domain,certDir,generation)')
edit(p,'Email: m.cfg.Email,','Email: m.cfg.Email,\n Logger:zap.NewNop(),')
edit(p,'m.magic = magic','''if m.issuerFactory!=nil{magic.Issuers=[]certmagic.Issuer{m.issuerFactory(magic)}}
 m.magic = magic;m.cache=cache
 success:=false;defer func(){if !success{cache.Stop();m.cache=nil;m.magic=nil}}()''')
edit(p,'m.acmeStarted = true\n\treturn nil','m.acmeStarted = true\n success=true\n\treturn nil')

p='internal/service/service.go'
edit(p,'if len(bootstrap.Users) == 0 {','s.applyRemoteOverrides(ctx, bootstrap.Config)\n if len(bootstrap.Users) == 0 {')
edit(p,'s.applyRemoteOverrides(ctx, bootstrap.Config)\n\tif !s.startKernel','if !s.certificateReady(bootstrap.Config){s.lastConfigHash="";s.markMailboxReadyAndDrain(ctx);return nil}\n\tif !s.startKernel')
edit(p,'if err != nil {\n\t\tnlog.Core().Error("failed to apply runtime cert config", "mode", cfgCopy.CertMode, "error", err)','if err != nil {\n  s.lastConfigHash=""\n\t\tnlog.Core().Error("failed to apply runtime cert config", "mode", cfgCopy.CertMode)')
edit(p,'func (s *Service) startKernel(nc *model.NodeSpec, users []model.UserSpec) bool {','func (s *Service) startKernel(nc *model.NodeSpec, users []model.UserSpec) bool {\n if !s.certificateReady(nc){s.lastConfigHash="";return false}')
edit(p,'// If config changed, delegate to kernel.Reload.','if !s.certificateReady(s.lastConfig){s.kernel.Stop();s.lastConfigHash="";return}\n\n // If config changed, delegate to kernel.Reload.')
edit(p,'certChanged := s.cert.CertRenewed()','certChanged := false')
edit(p,'result := pullResult{certChanged: certChanged}','certChanged = s.cert.CertRenewed()\n  result := pullResult{certChanged: certChanged}')

for name,target in [('certificate_state','internal/cert/caddy_certificate_state.go'),('certificate_service','internal/service/caddy_certificate_service.go')]:shutil.copyfile(root/f'patches/{name}.go',target)
for name,target in [('certificate_lifecycle','internal/cert/caddy_certificate_lifecycle_test.go'),('certificate_service','internal/service/caddy_certificate_service_test.go')]:shutil.copyfile(root/f'tests/{name}_test.go',target)
