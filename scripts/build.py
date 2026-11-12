#!/usr/bin/env python3
"""Cross-build the prepared source, produce normalized archives and checksums."""
import argparse
from datetime import datetime, timezone
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile


def archive(folder: Path, epoch: int) -> bytes:
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode='w', format=tarfile.PAX_FORMAT) as tar:
        for name in ['caddy', 'caddyctl']:
            data = (folder / name).read_bytes()
            info = tarfile.TarInfo(name)
            info.size, info.mtime, info.mode = len(data), epoch, 0o755
            info.uid = info.gid = 0
            info.uname = info.gname = ''
            tar.addfile(info, io.BytesIO(data))
    output = io.BytesIO()
    with gzip.GzipFile(fileobj=output, filename='', mode='wb', mtime=0, compresslevel=9) as zipped:
        zipped.write(raw.getvalue())
    return output.getvalue()


def build(source: Path, arch: str, out: Path):
    manifest = json.loads((source / 'build-source.json').read_text(encoding='utf-8'))
    lock = json.loads((Path(__file__).resolve().parents[1] / 'source.lock.json').read_text(encoding='utf-8'))
    if manifest['upstream_commit'] != lock['upstream_commit']:
        raise ValueError('prepared source does not match lock')
    folder = out / arch
    folder.mkdir(parents=True, exist_ok=True)
    version = 'custom-' + manifest['upstream_commit'][:7] + '-dual-ip-v2'
    if 'landing-v1' in manifest['features']:
        version += '-luma-v3' if 'certificate-lifecycle-v1' in manifest['features'] else '-luma-v2' if 'proxy-probes-v1' in manifest['features'] else '-luma-v1'
    if 'domain-audit-opt-in' in manifest['features']:
        version += '-audit'
    timestamp = datetime.fromtimestamp(manifest['source_date_epoch'], timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
    env = {**os.environ, 'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': arch, 'GOAMD64': 'v1'}
    ldflags = f'-s -w -X main.version={version} -X main.buildTime={timestamp}'
    for name, package in [('caddy', './cmd/xboard-node'), ('caddyctl', './cmd/xbctl')]:
        command = ['go', 'build', '-p', '2', '-mod=readonly', '-trimpath', '-buildvcs=false', '-ldflags', ldflags]
        if name == 'caddy':
            command += ['-tags', 'with_quic with_utls with_wireguard with_acme with_clash_api']
        subprocess.run(command + ['-o', str(folder / name), package], cwd=source, env=env, check=True)
    content = archive(folder, manifest['source_date_epoch'])
    sums = []
    for name in [f'caddy-linux-{arch}v1.tar.gz', f'caddy-alpine-{arch}.tar.gz']:
        (out / name).write_bytes(content)
        sums.append(hashlib.sha256(content).hexdigest() + '  ' + name)
    (out / f'SHA256SUMS-{arch}').write_text('\n'.join(sums) + '\n', encoding='utf-8')
    go_version = subprocess.check_output(['go', 'version'], text=True).strip()
    (out / f'build-info-{arch}.json').write_text(json.dumps({**manifest, 'version': version,
        'go_version': go_version, 'target': 'linux/' + arch, 'CGO_ENABLED': 0}, indent=2) + '\n', encoding='utf-8')
    print('\n'.join(sums))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--arch', choices=['amd64', 'arm64'], required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    build(args.source.resolve(), args.arch, args.out.resolve())
