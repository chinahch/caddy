from pathlib import Path

def replace(path,old,new):
    text=path.read_text(encoding='utf-8')
    if text.count(old)!=1:
        raise ValueError('machine source anchor changed: '+old[:100])
    path.write_text(text.replace(old,new,1),encoding='utf-8',newline='\n')

root=Path.cwd()
replace(root/'internal/panel/types.go','type MachineNode struct {','type MachineNode struct {\n KernelType string `json:"kernel_type,omitempty"`')
p=root/'internal/machine/machine.go'
replace(p,'type nodeHandle struct {','type nodeHandle struct {\n discoveredType string\n requestedKernel string')
replace(p,'\tmu    sync.Mutex','\tdiscoveryMu sync.Mutex\n\tmu    sync.Mutex')
replace(p,'\t// Start machine-level WS as early as possible', '\t// Report connectivity even when this machine has zero logical nodes.\n\to.reportMachineStatus()\n\to.discoveryMu.Lock()\n\t// Start machine-level WS as early as possible')
replace(p,'\tdiscoveryTicker := time.NewTicker(o.pullInterval)','\to.discoveryMu.Unlock()\n\tdiscoveryTicker := time.NewTicker(o.pullInterval)')
replace(p,'''\to.mu.Lock()
\tif _, exists := o.nodes[mn.ID]; exists {
\t\to.mu.Unlock()
\t\treturn
\t}

\tnodeCtx''','''\to.mu.Lock()
 h, exists := o.nodes[mn.ID]
 o.mu.Unlock()
 if exists {
  finished := false
  select { case <-h.done: finished=true; default: }
  if !finished && h.discoveredType==mn.Type && h.requestedKernel==mn.KernelType { return }
  // A failed service must be retried, and protocol/kernel changes require a
  // new Service because its native kernel is constructed only once.
  o.stopNode(mn.ID)
 }
 o.mu.Lock()
\tnodeCtx''')
replace(p,'&nodeHandle{cancel: cancel, done: done, mailbox: mb}', '&nodeHandle{cancel: cancel, done: done, mailbox: mb, discoveredType:mn.Type, requestedKernel:mn.KernelType}')
replace(p,'\tnodeCfg := o.cfg.ExpandMachineNode(mn.ID, mn.Type)','\tnodeCfg := o.cfg.ExpandMachineNode(mn.ID, mn.Type)\n if mn.KernelType=="xray" || mn.KernelType=="singbox" { nodeCfg.Kernel.Type=mn.KernelType }')
replace(p,'func (o *Orchestrator) rediscover(ctx context.Context) {','func (o *Orchestrator) rediscover(ctx context.Context) {\n o.discoveryMu.Lock()\n defer o.discoveryMu.Unlock()\n if ctx.Err()!=nil {return}')
replace(p,'if o.pullInterval < 30*time.Second {','if o.pullInterval < 10*time.Second {')
