#!/usr/bin/env python3
"""Copy Caddy-managed PEMs to root-only node paths; rerun after renewals."""
import glob
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
domain = sys.argv[1]
if os.geteuid() != 0 or not re.fullmatch(r'[a-z0-9][a-z0-9.-]+', domain):
    raise SystemExit('root and a DNS name required')
base = Path('/var/lib/caddy/.local/share/caddy/certificates')
matches = sorted(base.glob('*/' + domain + '/' + domain + '.crt'), key=lambda p: p.stat().st_mtime)
if not matches: raise SystemExit('Caddy certificate has not been issued yet')
cert = matches[-1]
key = cert.with_suffix('.key')
subprocess.run(['openssl','x509','-in',str(cert),'-checkend','3600','-noout'], check=True, stdout=subprocess.DEVNULL)
dest = Path('/etc/relaypilot/tls')
dest.mkdir(parents=True, exist_ok=True, mode=0o700)
changed = False
for src, name in [(cert,'cert.pem'),(key,'key.pem')]:
    target = dest / name
    data = src.read_bytes()
    if not target.exists() or target.read_bytes() != data:
        temporary = dest / (name + '.new')
        fd = os.open(temporary, os.O_WRONLY|os.O_CREAT|os.O_TRUNC, 0o600)
        with os.fdopen(fd,'wb') as f: f.write(data)
        os.replace(temporary, target)
        changed = True
if changed and Path('/etc/relaypilot/node/config.json').exists():
    subprocess.run(['/usr/local/bin/sing-box','check','-c','/etc/relaypilot/node/config.json'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(['systemctl','try-restart','relaypilot-node.service'],check=True)
