#!/usr/bin/env python3
"""Archive exact linked module sources and notices; refuse missing root licenses.

Includes all source rather than guessing which reciprocal licenses need source.
No dependency is relabeled with the customization repository's license.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile


def notice(path):
    return path.name.lower().startswith(('license','licence','copying','copyright','notice','unlicense'))


def module_files(root):
    if not any(p.is_file() and notice(p) for p in root.iterdir()):
        raise ValueError('missing root license/notice: '+str(root))
    return sorted(p for p in root.rglob('*') if p.is_file() and not p.is_symlink() and '.git' not in p.relative_to(root).parts)


def add_tree(tar,root,prefix,files):
    for path in files:
        data=path.read_bytes();info=tarfile.TarInfo(prefix+'/'+path.relative_to(root).as_posix())
        info.size=len(data);info.mode=0o644;info.mtime=0
        tar.addfile(info,io.BytesIO(data))


def run(source,*args):
    return subprocess.check_output(args,cwd=source,text=True,encoding='utf-8')


def collect(source,custom,out,arch):
    env={**os.environ,'GOOS':'linux','GOARCH':arch,'CGO_ENABLED':'0'}
    raw=subprocess.check_output(['go','list','-mod=readonly','-deps','-json','-tags','with_quic,with_utls,with_wireguard,with_acme,with_clash_api','./cmd/xboard-node','./cmd/xbctl'],cwd=source,env=env,text=True,encoding='utf-8')
    decoder=json.JSONDecoder();modules={};rest=raw
    while rest.strip():
        obj,n=decoder.raw_decode(rest.lstrip());rest=rest.lstrip()[n:]
        module=obj.get('Module')
        if module:
            actual=module.get('Replace',module)
            key=actual['Path']+'@'+actual.get('Version','main')
            modules[key]=actual
    if not modules:raise ValueError('empty dependency graph')
    out.mkdir(parents=True,exist_ok=True)
    inventory=[]
    with tarfile.open(out/f'caddy-sources-notices-{arch}.tar.gz','w:gz') as tar:
        for key,module in sorted(modules.items()):
            root=Path(module['Dir']);files=module_files(root);prefix='modules/'+hashlib.sha256(key.encode()).hexdigest()[:16]
            add_tree(tar,root,prefix,files)
            inventory.append({'module':module['Path'],'version':module.get('Version','main'),'archive_prefix':prefix,'notices':[{'path':p.relative_to(root).as_posix(),'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in files if notice(p)]})
        # Source changes, preparation/build scripts, and all original attribution.
        tracked=run(custom,'git','-c',f'safe.directory={custom.as_posix()}','ls-files','-z').split('\0')
        add_tree(tar,custom,'customization',[custom/p for p in tracked if p and (custom/p).is_file()])
        goroot=Path(run(source,'go','env','GOROOT').strip())
        add_tree(tar,goroot,'go-toolchain-sources',[goroot/'LICENSE',*(p for p in (goroot/'src').rglob('*') if p.is_file() and not p.is_symlink())])
    (out/f'dependency-inventory-{arch}.json').write_text(json.dumps({'target':'linux/'+arch,'go':run(source,'go','version').strip(),'modules':inventory},indent=2)+'\n',encoding='utf-8')
    print(f'{arch}: archived {len(modules)} exact modules, notices, customization and Go sources')


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--source',type=Path,required=True);p.add_argument('--custom',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--arch',choices=['amd64','arm64'],required=True);a=p.parse_args()
    collect(a.source.resolve(),a.custom.resolve(),a.out.resolve(),a.arch)
