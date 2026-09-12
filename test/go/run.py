#!/usr/bin/env python3
"""Run centrally stored Go tests in their original packages using Go's overlay."""
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

root = Path(__file__).resolve().parents[2]
sources = Path(__file__).resolve().parent / '_packages'
command = sys.argv[1] if len(sys.argv) > 1 else 'test'
if command not in ('test', 'vet'):
    raise SystemExit('Usage: python3 test/go/run.py [test|vet] [Go flags and packages]')
arguments = sys.argv[2:] or (['-race', './...'] if command == 'test' else ['./...'])
replacements = {}
for source in sorted(sources.rglob('*_test.go')):
    target = root / 'internal' / source.relative_to(sources)
    if not target.parent.is_dir() or target.exists():
        raise SystemExit('Test overlay target must be an existing package without a conflicting file')
    replacements[str(target)] = str(source)
if not replacements:
    raise SystemExit('No Go tests found')

# The temporary overlay is removed even when a test fails. Runtime fixtures are
# real files under test/fixtures because overlays only affect Go's build step.
with tempfile.TemporaryDirectory(prefix='foodie-go-tests-') as temporary:
    overlay = Path(temporary) / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': replacements}))
    result = subprocess.run([os.environ.get('GO', 'go'), command, '-overlay', str(overlay)] + arguments, cwd=str(root))
    sys.exit(result.returncode)
