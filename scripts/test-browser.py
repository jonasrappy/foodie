#!/usr/bin/env python3
"""Run browser tests on disposable servers. Never connects to household data."""
import json
import os
import select
import shutil
import subprocess
import tempfile
from pathlib import Path
from environment import read_environment

root = Path(__file__).resolve().parent.parent
base_env = {k:v for k,v in os.environ.items() if not k.startswith('FOODIE_')}
base_env.setdefault('MAD_PLAYWRIGHT_MODULE', str(root / 'test/node_modules/playwright'))
with tempfile.TemporaryDirectory(prefix='foodie-browser-') as temporary:
    home = Path(temporary)
    shutil.copytree(str(root / 'public'), str(home / 'public'))
    (home / 'downloads').mkdir()
    apk = home / 'downloads/foodie.apk'
    built = root / 'android/build/foodie.apk'
    if built.is_file():
        shutil.copyfile(str(built), str(apk))
    else:
        # Tests the authenticated transport, not Android package validity.
        apk.write_bytes(b'PK\x03\x04 Foodie synthetic download transport fixture\n')
    for language in ['da', 'en']:
        config = home / (language + '.env')
        subprocess.run([str(root / 'bin/foodie'), 'init', '--output', str(config)], cwd=str(root), env=base_env, check=True, stdout=subprocess.DEVNULL)
        values = read_environment(config, inherit=False)
        values = {k:v for k,v in values.items() if k.startswith('FOODIE_') or k.startswith('ANDROID_')}
        values.update(FOODIE_LANGUAGE=language, FOODIE_TIMEZONE='Europe/Copenhagen' if language=='da' else 'UTC', FOODIE_DATA_DIR=str(home / ('data-'+language)), FOODIE_PUBLIC_DIR=str(home / 'public'))
        config.write_text(''.join(k+'='+json.dumps(v)+'\n' for k,v in sorted(values.items())))
        server = subprocess.Popen([str(root / 'bin/foodie'), 'serve', '--config', str(config), '--listen', '127.0.0.1:0'], cwd=str(root), env=base_env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, universal_newlines=True)
        try:
            if not select.select([server.stdout], [], [], 20)[0]:
                raise RuntimeError('Test server did not start')
            line = server.stdout.readline()
            startup = json.loads(line)
            if 'address' not in startup:
                raise RuntimeError('Test server failed to start')
            env = dict(base_env)
            env.update(MAD_TEST_URL='http://'+startup['address'], MAD_TEST_PASSWORD=values['FOODIE_PASSWORD'], MAD_TEST_BOT_TOKEN=values['FOODIE_BOT_TOKEN'], MAD_TEST_APK=str(apk), MAD_SCREENSHOTS=os.environ.get('MAD_SCREENSHOTS',str(home/'screenshots')))
            tests = ['browser.cjs','voice-presentation.cjs','download.cjs','android-update.cjs','language.cjs'] if language=='da' else ['english.cjs','language.cjs']
            for name in tests:
                subprocess.run(['node',str(root/'test'/name)],cwd=str(root),env=env,check=True)
        finally:
            server.terminate()
            try:
                server.wait(timeout=20)
            except subprocess.TimeoutExpired:
                server.kill();server.wait()
