#!/usr/bin/env python3
"""Generate deployment-specific Java/manifest files only inside ignored build/."""
import json
import os
import re
import shutil
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.parse import urlsplit
from environment import read_environment

root = Path(__file__).resolve().parent.parent
os.chdir(str(root))
try:
    values = read_environment(os.environ.get('FOODIE_ENV_FILE', '.env'))
    site = values.get('FOODIE_SITE_URL', '').rstrip('/') + '/'
    url = urlsplit(site)
    if (url.scheme != 'https' or not url.hostname or url.username or url.password or
            url.path != '/' or url.query or url.fragment or re.search(r'\s', site) or
            url.hostname.endswith('.example.com')):
        raise ValueError('Set FOODIE_SITE_URL to your own HTTPS origin before building Android')
    _ = url.port
    app_id = values.get('ANDROID_APPLICATION_ID', 'app.foodie.mobile')
    if not re.fullmatch(r'[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+', app_id):
        raise ValueError('Invalid ANDROID_APPLICATION_ID')
    version = values.get('ANDROID_VERSION_NAME', '1.3.2')
    code = values.get('ANDROID_VERSION_CODE', '8')
    if not re.fullmatch(r'\d+\.\d+\.\d+', version) or not code.isdigit() or not 0 < int(code) < 2100000000:
        raise ValueError('Invalid Android version')
    sdk = values.get('ANDROID_SDK_ROOT') or values.get('ANDROID_HOME', '')
    if not sdk or not Path(sdk, 'platforms/android-35/android.jar').is_file():
        raise ValueError('Set ANDROID_SDK_ROOT to an SDK with platform android-35 and build-tools 35.0.0')
    sign_dir = Path(values.get('ANDROID_SIGN_DIR', './.private/android-signing')).resolve()
    alias = values.get('ANDROID_KEY_ALIAS', 'foodie')
    if not re.fullmatch(r'[A-Za-z0-9_-]+', alias):
        raise ValueError('Invalid ANDROID_KEY_ALIAS')
    language = values.get('FOODIE_LANGUAGE', 'en')
    if language not in ('en', 'da'):
        raise ValueError('FOODIE_LANGUAGE must be en or da')
    build = root / 'android/build'
    for folder in ['classes', 'generated', 'dex', 'src']:
        path = build / folder
        if path.exists():
            shutil.rmtree(str(path))
        path.mkdir(parents=True)
    language_assets=build / 'language-assets'
    language_assets.mkdir(exist_ok=True)
    shutil.copyfile(str(root / 'internal/i18n/locales' / (language+'.json')),str(language_assets / 'i18n.json'))
    (language_assets / 'release.json').write_text(json.dumps({'version_code':int(code),'version_name':version}))
    dest = build / 'src' / app_id.replace('.', '/')
    dest.mkdir(parents=True)
    for source in (root / 'android/src/app/foodie/mobile').glob('*.java'):
        (dest / source.name).write_text(source.read_text().replace('app.foodie.mobile', app_id))
    (dest / 'BuildConfig.java').write_text('package {};\nfinal class BuildConfig {{ static final String SITE_URL={}; static final String VERSION_NAME={}; static final int VERSION_CODE={}; }}\n'.format(app_id,json.dumps(site),json.dumps(version),int(code)))
    ET.register_namespace('android', 'http://schemas.android.com/apk/res/android')
    manifest = ET.parse(str(root / 'android/AndroidManifest.xml'))
    manifest.getroot().set('package', app_id)
    manifest.getroot().set('{http://schemas.android.com/apk/res/android}versionName', version)
    manifest.getroot().set('{http://schemas.android.com/apk/res/android}versionCode', code)
    manifest.write(str(build / 'AndroidManifest.xml'), encoding='utf-8', xml_declaration=True)
    # Only build paths/alias are printed, never app credentials or the configured URL.
    print(Path(sdk).resolve())
    print(sign_dir)
    print(alias)
except (ValueError, OSError):
    print('Android configuration is invalid. Check the non-secret build settings in .env and the required SDK.', file=sys.stderr)
    sys.exit(1)
