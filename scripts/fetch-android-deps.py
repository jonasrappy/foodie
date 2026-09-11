#!/usr/bin/env python3
"""Fetch pinned voice binaries; verify each file before installing it."""
import argparse
import hashlib
import json
import os
import tarfile
import tempfile
import urllib.request
from pathlib import Path

root = Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--directory', type=Path, default=root / 'android')
target = parser.parse_args().directory
manifest = json.loads((root / 'android/vendor-checksums.json').read_text())
needed = {p: h for p, h in manifest.items() if p.startswith('lib/') or (p.startswith('assets/') and not p.endswith('keywords.txt'))}
missing = {p: h for p, h in needed.items() if not (target / p).is_file() or hashlib.sha256((target / p).read_bytes()).hexdigest() != h}
archives = [
    ('lib/', 'https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.13.8/sherpa-onnx-v1.13.8-android.tar.bz2'),
    ('assets/', 'https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-gigaspeech-3.3M-2024-01-01-mobile.tar.bz2'),
    ('assets/', 'https://github.com/k2-fsa/sherpa-onnx/releases/download/kws-models/sherpa-onnx-kws-zipformer-gigaspeech-3.3M-2024-01-01.tar.bz2'),
]
for prefix, url in archives:
    if not any(p.startswith(prefix) for p in missing):
        continue
    print('Fetching ' + url.rsplit('/', 1)[1], flush=True)
    with urllib.request.urlopen(url, timeout=120) as response, tempfile.TemporaryFile() as archive:
        while True:
            block = response.read(1024 * 1024)
            if not block:
                break
            archive.write(block)
        archive.seek(0)
        with tarfile.open(fileobj=archive, mode='r|bz2') as package:
            for member in package:
                name = Path(member.name).name
                if not member.isfile() or not (name.endswith('.onnx') or name == 'tokens.txt' or name in ['libsherpa-onnx-jni.so', 'libonnxruntime.so']):
                    continue
                data = package.extractfile(member).read()
                digest = hashlib.sha256(data).hexdigest()
                for path, expected in list(missing.items()):
                    if path.startswith(prefix) and digest == expected:
                        destination = target / path
                        destination.parent.mkdir(parents=True, exist_ok=True)
                        temporary = destination.with_name('.' + destination.name + '.download')
                        temporary.write_bytes(data)
                        os.replace(str(temporary), str(destination))
                        del missing[path]
# Tar member paths are never extracted. Only manifest paths with matching hashes are written.
if missing:
    raise SystemExit('Pinned files were not found or did not match their checksums: ' + ', '.join(sorted(missing)))
print('Voice dependencies verified.')
