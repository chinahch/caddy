#!/usr/bin/env python3
"""Prepare a new source directory from the pinned commit, never modify upstream."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
LOCK = json.loads((ROOT / 'source.lock.json').read_text(encoding='utf-8'))


def run(*args, cwd=None):
    return subprocess.run(args, cwd=cwd, check=True, capture_output=True)


def prepare(upstream: Path, destination: Path, audit: bool = False):
    if destination.exists():
        raise ValueError('destination already exists; refusing to overwrite')
    try:
        commit = run('git', '-C', str(upstream), 'rev-parse', 'HEAD').stdout.decode().strip()
    except subprocess.CalledProcessError as exc:
        raise ValueError('requires pinned upstream checkout with a valid HEAD') from exc
    if commit != LOCK['upstream_commit']:
        raise ValueError('requires pinned upstream commit ' + LOCK['upstream_commit'])
    epoch = int(run('git', '-C', str(upstream), 'show', '-s', '--format=%ct', 'HEAD').stdout)
    destination.parent.mkdir(parents=True, exist_ok=True)
    # Stage alongside destination so rename is atomic and failures expose no output.
    with tempfile.TemporaryDirectory(prefix='.prepare-', dir=destination.parent) as temp:
        stage = Path(temp) / 'source'
        stage.mkdir()
        archive = run('git', '-C', str(upstream), 'archive', 'HEAD').stdout
        with tarfile.open(fileobj=io.BytesIO(archive)) as contents:
            contents.extractall(stage, filter='data')
        run('git', 'apply', '--check', str(ROOT / 'patches/xhttp-v2-fixes.patch'), cwd=stage)
        run('git', 'apply', str(ROOT / 'patches/xhttp-v2-fixes.patch'), cwd=stage)
        shutil.copyfile(ROOT / 'patches/xhttp_tls_v2_test.go', stage / 'internal/kernel/xray/xhttp_tls_v2_test.go')
        run(sys.executable, '-X', 'utf8', str(ROOT / 'scripts/apply_ipfamily.py'), cwd=stage)
        run(sys.executable, '-X', 'utf8', str(ROOT / 'patches/strict_ipfamily.py'), cwd=stage)
        run(sys.executable, '-X', 'utf8', str(ROOT / 'scripts/apply_landing.py'), cwd=stage)
        run(sys.executable, '-X', 'utf8', str(ROOT / 'scripts/apply_protocols.py'), cwd=stage)
        for name, target in [('xray','kernel/xray'), ('singbox','kernel/singbox'), ('client','panel')]:
            shutil.copyfile(ROOT / f'tests/landing_{name}_test.go', stage / f'internal/{target}/luma_landing_test.go')
        for name in ['xray','singbox']:
            shutil.copyfile(ROOT / f'tests/preset_{name}_test.go', stage / f'internal/kernel/{name}/luma_presets_test.go')
            shutil.copyfile(ROOT / f'tests/dataplane_{name}_test.go', stage / f'internal/kernel/{name}/luma_dataplane_test.go')
        shutil.copyfile(ROOT / 'tests/protocol_kernel_test.go', stage / 'internal/model/luma_kernel_test.go')
        shutil.copyfile(ROOT / 'tests/private_errors_xray_test.go',stage / 'internal/kernel/xray/luma_private_errors_test.go')
        cli = stage / 'cmd/xbctl/main.go'
        text = cli.read_text(encoding='utf-8')
        for old, new in {
            '/etc/xboard-node': '/etc/caddy',
            '/usr/local/bin/xboard-node': '/usr/local/bin/caddy',
            '/usr/local/bin/xbctl': '/usr/local/bin/caddyctl',
            'xboard-node.service': 'caddy.service',
            'case "upgrade":\n\t\treturn runUpgrade(args[1:])':
                'case "upgrade":\n\t\treturn fmt.Errorf("custom caddy build: install a verified caddy package")',
        }.items():
            if old not in text:
                raise ValueError('upstream CLI anchor changed: ' + old)
            text = text.replace(old, new)
        # The upstream CLI has its own YAML wire structs. Preserve newly added
        # fields there as well, otherwise bind/config commands drop IP policy.
        anchors = {
            'type fileKernelConfig struct {': 'type fileKernelConfig struct {\n\tCustomConfigSingbox string `yaml:"custom_config_singbox,omitempty"`\n\tCustomConfigXray string `yaml:"custom_config_xray,omitempty"`\n\tIPFamily map[string]string `yaml:"ip_family,omitempty"`',
            '\t\t\t\tCustomConfig: inst.Kernel.CustomConfig,': '\t\t\t\tCustomConfig: inst.Kernel.CustomConfig,\n\t\t\t\tCustomConfigSingbox: inst.Kernel.CustomConfigSingbox,\n\t\t\t\tCustomConfigXray: inst.Kernel.CustomConfigXray,\n\t\t\t\tIPFamily: inst.Kernel.IPFamily,',
            '\tif p.Kernel.Type != "" || p.Kernel.LogLevel != "" {\n\t\tout.Kernel = &fileKernelConfig{Type: p.Kernel.Type, LogLevel: p.Kernel.LogLevel}\n\t}':
                '\tout.Kernel = &fileKernelConfig{Type:p.Kernel.Type, ConfigDir:p.Kernel.ConfigDir, LogLevel:p.Kernel.LogLevel, GeoDataDir:p.Kernel.GeoDataDir, CustomConfig:p.Kernel.CustomConfig, CustomConfigSingbox:p.Kernel.CustomConfigSingbox, CustomConfigXray:p.Kernel.CustomConfigXray, IPFamily:p.Kernel.IPFamily, CustomRoute:p.Kernel.CustomRoute, CustomOut:p.Kernel.CustomOutbound}',
        }
        for old,new in anchors.items():
            if text.count(old)!=1:
                raise ValueError('upstream CLI serialization anchor changed')
            text=text.replace(old,new,1)
        cli.write_text(text, encoding='utf-8', newline='\n')
        shutil.copyfile(ROOT / 'tests/cli_roundtrip_test.go',stage / 'cmd/xbctl/caddy_config_roundtrip_test.go')
        features = ['xhttp', 'dual-ip-strict', 'caddy-cli', 'landing-v1']
        inputs = ['patches/xhttp-v2-fixes.patch', 'patches/xhttp_tls_v2_test.go',
                  'scripts/apply_ipfamily.py', 'patches/strict_ipfamily.py', 'scripts/prepare_source.py', 'scripts/apply_landing.py', 'scripts/apply_protocols.py']
        if audit:
            shutil.copytree(ROOT / 'audit-overlay/internal/domainaudit', stage / 'internal/domainaudit')
            run(sys.executable, '-X', 'utf8', str(ROOT / 'scripts/patch_source.py'), str(stage))
            features.append('domain-audit-opt-in')
            inputs += ['scripts/patch_source.py', 'audit-overlay/internal/domainaudit/audit.go']
        go_files = [str(p) for p in (stage / 'internal').rglob('*.go')]
        run('gofmt', '-w', str(cli), *go_files)
        manifest = {**LOCK, 'source_date_epoch': epoch, 'features': features,
                    'customization_sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in inputs}}
        (stage / 'build-source.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
        stage.rename(destination)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--upstream', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    try:
        prepare(args.upstream.resolve(), args.out.resolve(), args.audit)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        detail = getattr(error, 'stderr', b'')
        print(str(error) + ('\n' + detail.decode(errors='replace') if detail else ''), file=sys.stderr)
        sys.exit(1)
    print('Prepared pinned source:', args.out)
