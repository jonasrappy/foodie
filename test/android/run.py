#!/usr/bin/env python3
"""Compile and run native policy/presentation tests without an Android device."""
import os
import subprocess
import tempfile
from pathlib import Path

root = Path(__file__).resolve().parents[2]
native = root / 'android/src/app/foodie/mobile'
tests = Path(__file__).resolve().parent
names = ['SpeechPolicy', 'SpeechWindow', 'VoicePresentation']
jni = os.environ.get('MAD_JNI_DIR')
with tempfile.TemporaryDirectory(prefix='foodie-android-tests-') as temporary:
    sources = [str(native / (name + '.java')) for name in names]
    sources += [str(tests / (name + 'Tests.java')) for name in names]
    if jni:
        sources += [str(path) for path in (root / 'android/vendor/com/k2fsa/sherpa/onnx').glob('*.java')]
        sources.append(str(tests / 'FoodieTests.java'))
    subprocess.run(['javac', '-d', temporary] + sources, check=True)
    for name in names:
        subprocess.run(['java', '-cp', temporary, 'app.foodie.mobile.' + name + 'Tests'], check=True)
    if jni:
        subprocess.run(['java', '-Djava.library.path=' + str(Path(jni).resolve()), '-cp', temporary,
                        'app.foodie.mobile.FoodieTests', str(root / 'android/assets/foodie')]
                       + [str(path) for path in sorted((tests / 'audio').glob('*.wav'))], check=True)
