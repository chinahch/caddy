#!/usr/bin/env python3
"""Reject silent core downgrades and honor explicit Machine kernel selection."""
from pathlib import Path

def edit(path, old, new):
    p=Path(path)
    text=p.read_text(encoding='utf-8')
    if text.count(old)!=1:
        raise ValueError('protocol source anchor changed: '+path)
    p.write_text(text.replace(old,new),encoding='utf-8',newline='\n')

edit('internal/model/validate.go', '\tadditionalOutboundSources, err := collectAdditionalOutboundTagSources',
     '\tif n.KernelType!="" { requested,e:=normalizeKernelType(n.KernelType); if e!=nil{return e};if requested!=kernelType{return fmt.Errorf("panel requires %s kernel; local kernel is %s",requested,kernelType)} }\n'
     '\tif kernelType=="singbox" && n.Decryption!="" && n.Decryption!="none" {return fmt.Errorf("VLESS native encryption requires xray kernel")}\n'
     '\tadditionalOutboundSources, err := collectAdditionalOutboundTagSources')
edit('internal/machine/machine.go', '\t\tif resolved := model.ResolveKernelForTransport(cfgSnapshot.Network, nodeCfg.Kernel.Type);',
     '\t\tif cfgSnapshot.KernelType=="xray" || cfgSnapshot.KernelType=="singbox" { nodeCfg.Kernel.Type=cfgSnapshot.KernelType }\n'
     '\t\tif resolved := model.ResolveKernelForTransport(cfgSnapshot.Network, nodeCfg.Kernel.Type);')

edit('internal/kernel/singbox/config.go',
     '\tif tls := buildTLSConfig(nc, tc); tls != nil {\n\t\tbase["tls"] = tls\n\t} else {\n\t\tnlog.Core().Warn("anytls',
     '\tif nc.TLS == 2 {\n\t\tbase["tls"] = buildRealityConfig(nc)\n\t} else if tls := buildTLSConfig(nc, tc); tls != nil {\n\t\tbase["tls"] = tls\n\t} else {\n\t\tnlog.Core().Warn("anytls')

# Pinned core errors may concatenate a malformed private key/decryption string.
# Never propagate those values to service logs, including reload/start failures.
for phase, old in [('parse','parse xray config'),('create','create xray'),('start','start xray')]:
    edit('internal/kernel/xray/xray.go',
         'return fmt.Errorf("'+old+': %w", err)',
         'return fmt.Errorf("xray configuration rejected during '+phase+'; check keys, certificates and protocol settings")')
