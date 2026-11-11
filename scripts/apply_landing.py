#!/usr/bin/env python3
"""Apply the typed Luma outbound bridge to the pinned source in cwd."""
from pathlib import Path

def edit(path, old, new, count=1):
    file = Path(path)
    text = file.read_text(encoding='utf-8')
    if text.count(old) != count:
        raise ValueError(f'landing source anchor changed: {path}')
    file.write_text(text.replace(old, new), encoding='utf-8', newline='\n')

edit('internal/panel/types.go', 'type OutboundConfig struct {',
     'type OutboundConfig struct {\n\tStreamSettings map[string]any `json:"stream_settings,omitempty"`')
edit('internal/model/types.go', 'type OutboundConfig struct {',
     'type OutboundConfig struct {\n\tStreamSettings map[string]any')
edit('internal/model/panel.go', 'Settings: cloneAnyMap(outbound.Settings),',
     'Settings: cloneAnyMap(outbound.Settings),\n\t\t\tStreamSettings: cloneAnyMap(outbound.StreamSettings),', 2)
edit('internal/kernel/xray/config.go', '\tif oc.ProxyTag != "" {',
     '\tif len(oc.StreamSettings)>0 { m["streamSettings"] = oc.StreamSettings }\n\tif oc.ProxyTag != "" {')
edit('internal/kernel/singbox/config.go', 'm["proxy_tag"] = oc.ProxyTag',
     'm["detour"] = oc.ProxyTag')
edit('internal/model/custom_outbound_validate.go', '\t\tif err := validateOutboundSettings(i, outbound.Settings); err != nil {',
     '\t\tif kernelType=="singbox" && len(outbound.StreamSettings)>0 { return fmt.Errorf("custom_outbounds[%d].stream_settings requires xray",i) }\n\t\tif err := validateOutboundSettings(i, outbound.Settings); err != nil {')
edit('internal/model/custom_outbound_support.go', '\t\t\t\t"settings",',
     '\t\t\t\t"settings",\n\t\t\t\t"stream_settings",', 2)
edit('internal/panel/client.go', '\treq.Header.Set("Accept", "application/json")',
     '\treq.Header.Set("Accept", "application/json")\n\treq.Header.Set("X-Luma-Capabilities", "landing-v1,protocols-v1")')
