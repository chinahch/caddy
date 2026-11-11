#!/usr/bin/env python3
"""Download pinned official connector assets and verify SHA256 before exposing files."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import urllib.request

ROOT=Path(__file__).resolve().parents[1]

def fetch(arch, destination):
    if destination.exists():
        raise ValueError('destination exists; refusing to overwrite connector binaries')
    lock=json.loads((ROOT/'connector/versions.lock.json').read_text(encoding='utf-8'))
    destination.parent.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.connectors-',dir=destination.parent) as temp:
        stage=Path(temp)/'bin'
        stage.mkdir()
        for name in ['mtg','cloudflared']:
            asset=lock[name]['assets'][arch]
            request=urllib.request.Request(asset['url'],headers={'User-Agent':'Luma-connector-build'})
            with urllib.request.urlopen(request,timeout=90) as response:
                data=response.read(128*1024*1024+1)
            if len(data)>128*1024*1024 or hashlib.sha256(data).hexdigest()!=asset['sha256']:
                raise ValueError(name+': asset integrity verification failed')
            binary=data
            if asset['archive']:
                with tarfile.open(fileobj=io.BytesIO(data),mode='r:gz') as archive:
                    candidates=[m for m in archive.getmembers() if m.isfile() and Path(m.name).name==name]
                    if len(candidates)!=1 or candidates[0].size>128*1024*1024:
                        raise ValueError(name+': unexpected binary archive')
                    binary=archive.extractfile(candidates[0]).read()
            target=stage/name
            target.write_bytes(binary)
            target.chmod(0o755)
        (stage/'versions.lock.json').write_text(json.dumps(lock,indent=2)+'\n',encoding='utf-8')
        stage.rename(destination)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--arch',choices=['amd64','arm64'],required=True)
    p.add_argument('--out',type=Path,required=True)
    args=p.parse_args()
    fetch(args.arch,args.out.resolve())
