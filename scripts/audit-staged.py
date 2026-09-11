#!/usr/bin/env python3
"""Check staged Git blobs for private settings without printing matched values."""
import base64
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlsplit
from environment import read_environment

root=Path(__file__).resolve().parent.parent
values=read_environment(root/'.env',inherit=False)
private=[]
for key in ['FOODIE_PASSWORD','FOODIE_PASSWORD_SALT','FOODIE_PASSWORD_HASH','FOODIE_SESSION_SECRET','FOODIE_BOT_TOKEN','FOODIE_BOT_TOKEN_HASH']:
    if values.get(key):
        private.append((key,values[key]))
site=values.get('FOODIE_SITE_URL','')
host=urlsplit(site).hostname if site else None
if host and not (host.endswith(('.example.com','.example.org','.example.net','.example','.test')) or host=='localhost'):
    private.append(('deployment hostname',host))
extra=os.environ.get('FOODIE_AUDIT_VALUES_FILE')
if extra:
    for key,value in json.loads(Path(extra).read_text()).items():
        if value:
            private.append((key,value))
patterns=[
    re.compile(rb'github_pat_[A-Za-z0-9_]{30,}'),
    re.compile(rb'gh[pousr]_[A-Za-z0-9]{30,}'),
    re.compile(rb'-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----'),
    re.compile(rb'https?://[^\s/@:]+:[^\s/@]+@'),
]
files=subprocess.check_output(['git','ls-files','--cached','-z'],cwd=str(root)).split(b'\0')
failures=[]
for raw in files:
    if not raw:
        continue
    name=os.fsdecode(raw)
    if name=='.env' or (name.startswith('.env.') and name!='.env.example') or name.endswith(('.apk','.jks','.keystore','.sqlite','.pem')):
        failures.append((name,'private or generated file'))
    data=subprocess.check_output(['git','show',':'+name],cwd=str(root))
    for key,value in private:
        encoded=value.encode()
        forms=[encoded,encoded.decode().encode('utf-16-le'),json.dumps(value,ensure_ascii=True)[1:-1].encode()]
        if len(encoded)>=16:
            forms.append(base64.b64encode(encoded))
        if any(form.lower() in data.lower() for form in forms):
            failures.append((name,key))
    if any(pattern.search(data) for pattern in patterns):
        failures.append((name,'credential pattern'))
if failures:
    for name,kind in sorted(set(failures)):
        print('Blocked: {} ({})'.format(name,kind),file=sys.stderr)
    raise SystemExit(1)
print('Staged files passed the private-value and credential-pattern checks.')
