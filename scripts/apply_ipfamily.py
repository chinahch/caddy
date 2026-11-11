from pathlib import Path

def replace_once(path, old, new):
    p = Path(path)
    s = p.read_text()
    if s.count(old) != 1:
        raise SystemExit(f'{path}: expected exactly one anchor, found {s.count(old)}: {old[:90]!r}')
    p.write_text(s.replace(old, new, 1))

# A single Machine can host nodes with different actual kernels.
# Each core reads only its own setting, after its normal config merge.
p = 'internal/config/config.go'
replace_once(p,
    '\tCustomConfig string `yaml:"custom_config"`',
    '\tCustomConfig string `yaml:"custom_config"`\n'
    '\tCustomConfigSingbox string `yaml:"custom_config_singbox,omitempty"`\n'
    '\tCustomConfigXray string `yaml:"custom_config_xray,omitempty"`\n'
    '\tIPFamily map[string]string `yaml:"ip_family,omitempty"`')

# After XHTTP auto-switch, choose the config file matching actual core.
p = 'internal/machine/machine.go'
replace_once(p,
    '\t// Reset cached ETag so the subsequent GetConfig in Initial() gets a full response.',
    '''\t// A Machine may host sing-box and Xray concurrently. Select the
\t// kernel-native custom config *after* transport auto-switching.
\tswitch nodeCfg.Kernel.Type {
\tcase "singbox":
\t\tif path := nodeCfg.Kernel.CustomConfigSingbox; path != "" {
\t\t\tnodeCfg.Kernel.CustomConfig = path
\t\t}
\tcase "xray":
\t\tif path := nodeCfg.Kernel.CustomConfigXray; path != "" {
\t\t\tnodeCfg.Kernel.CustomConfig = path
\t\t}
\t}
\t// Reset cached ETag so the subsequent GetConfig in Initial() gets a full response.''')

p = 'internal/kernel/singbox/config.go'
replace_once(p,
    '\tmergeCustomSingbox(cfg, kcfg)\n\treturn cfg',
    '\tmergeCustomSingbox(cfg, kcfg)\n\tapplyMachineIPFamily(cfg, kcfg)\n\treturn cfg')
with Path(p).open('a') as f:
    f.write('''
// applyMachineIPFamily sets an address-family preference for DNS
// resolution. Existing panel rules remain after this non-final action.
func applyMachineIPFamily(cfg M, kcfg config.KernelConfig) {
\tstrategy := kcfg.IPFamily["singbox"]
\tswitch strategy {
\tcase "prefer_ipv6", "prefer_ipv4", "ipv6_only", "ipv4_only":
\tdefault:
\t\treturn
\t}
\troute, ok := cfg["route"].(M)
\tif !ok {
\t\treturn
\t}
\trule := M{"action": "resolve", "strategy": strategy}
\tif rules, ok := route["rules"].([]M); ok {
\t\troute["rules"] = append([]M{rule}, rules...)
\t} else if rules, ok := route["rules"].([]any); ok {
\t\troute["rules"] = append([]any{rule}, rules...)
\t} else {
\t\troute["rules"] = []M{rule}
\t}
}
''')

p = 'internal/kernel/xray/config.go'
replace_once(p,
    '\tmergeCustomXray(cfg, kcfg)\n\treturn cfg',
    '\tmergeCustomXray(cfg, kcfg)\n\tapplyMachineIPFamily(cfg, kcfg)\n\treturn cfg')
with Path(p).open('a') as f:
    f.write('''
// applyMachineIPFamily sets the address-family strategy on the
// existing default freedom outbound, without appending a duplicate.
func applyMachineIPFamily(cfg M, kcfg config.KernelConfig) {
\tstrategies := map[string]string{
\t\t"prefer_ipv6": "UseIPv6v4",
\t\t"prefer_ipv4": "UseIPv4v6",
\t\t"ipv6_only":   "ForceIPv6",
\t\t"ipv4_only":   "ForceIPv4",
\t}
\tstrategy, ok := strategies[kcfg.IPFamily["xray"]]
\tif !ok {
\t\treturn
\t}
\toutbounds, ok := cfg["outbounds"].([]M)
\tif !ok {
\t\treturn
\t}
\tfor _, outbound := range outbounds {
\t\tif outbound["tag"] != "direct" || outbound["protocol"] != "freedom" {
\t\t\tcontinue
\t\t}
\t\tstream, ok := outbound["streamSettings"].(M)
\t\tif !ok {
\t\t\tstream = M{}
\t\t\toutbound["streamSettings"] = stream
\t\t}
\t\tsockopt, ok := stream["sockopt"].(M)
\t\tif !ok {
\t\t\tsockopt = M{}
\t\t\tstream["sockopt"] = sockopt
\t\t}
\t\tsockopt["domainStrategy"] = strategy
\t\treturn
\t}
}
''')
