#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p build
python3 ../scripts/fetch-android-deps.py
python3 ../scripts/prepare-android.py > build/config.paths
mapfile -t FOODIE_BUILD_CONFIG < build/config.paths
FOODIE_SDK="${FOODIE_BUILD_CONFIG[0]}"
FOODIE_SIGN_DIR="${FOODIE_BUILD_CONFIG[1]}"
FOODIE_KEY_ALIAS="${FOODIE_BUILD_CONFIG[2]}"
FOODIE_BUILD_TOOLS="$FOODIE_SDK/build-tools/35.0.0"
FOODIE_ANDROID_JAR="$FOODIE_SDK/platforms/android-35/android.jar"
python3 - <<'CHECK_VENDOR'
import hashlib,json
for path,expected in json.load(open('vendor-checksums.json')).items():
    if hashlib.sha256(open(path,'rb').read()).hexdigest()!=expected:
        raise SystemExit('Bundled voice dependency checksum mismatch: '+path)
CHECK_VENDOR
mkdir -p "$FOODIE_SIGN_DIR"
chmod 700 "$FOODIE_SIGN_DIR"
if [ ! -f "$FOODIE_SIGN_DIR/release.jks" ]; then
    if [ -e "$FOODIE_SIGN_DIR/password" ]; then
        echo 'Signing password exists without its keystore; restore the matching key before building.' >&2
        exit 1
    fi
    (umask 077; openssl rand -hex 32 > "$FOODIE_SIGN_DIR/password")
    keytool -genkeypair -keystore "$FOODIE_SIGN_DIR/release.jks" -storetype JKS -alias "$FOODIE_KEY_ALIAS" -keyalg RSA -keysize 3072 -validity 10000 -dname 'CN=Foodie, OU=Personal applications' -storepass:file "$FOODIE_SIGN_DIR/password" -keypass:file "$FOODIE_SIGN_DIR/password"
    chmod 600 "$FOODIE_SIGN_DIR/release.jks"
fi
"$FOODIE_BUILD_TOOLS/aapt2" compile --dir res -o build/resources.zip
"$FOODIE_BUILD_TOOLS/aapt2" link -o build/resources.apk --manifest build/AndroidManifest.xml -I "$FOODIE_ANDROID_JAR" --java build/generated -A assets -A build/language-assets build/resources.zip
find build/src build/generated vendor -name '*.java' -print > build/sources.list
javac -source 8 -target 8 -Xlint:-options -bootclasspath "$FOODIE_ANDROID_JAR:$FOODIE_BUILD_TOOLS/core-lambda-stubs.jar" -d build/classes @build/sources.list
jar cf build/classes.jar -C build/classes .
"$FOODIE_BUILD_TOOLS/d8" --lib "$FOODIE_ANDROID_JAR" --min-api 23 --output build/dex build/classes.jar
cp build/resources.apk build/unsigned.apk
(cd build/dex && zip -q ../unsigned.apk classes.dex)
zip -q build/unsigned.apk lib/arm64-v8a/libonnxruntime.so lib/arm64-v8a/libsherpa-onnx-jni.so lib/armeabi-v7a/libonnxruntime.so lib/armeabi-v7a/libsherpa-onnx-jni.so
mkdir -p build/license-assets/assets/licenses
cp licenses/* build/license-assets/assets/licenses/
(cd build/license-assets && zip -q -r ../unsigned.apk assets/licenses)
"$FOODIE_BUILD_TOOLS/zipalign" -f 4 build/unsigned.apk build/aligned.apk
"$FOODIE_BUILD_TOOLS/apksigner" sign --ks "$FOODIE_SIGN_DIR/release.jks" --ks-key-alias "$FOODIE_KEY_ALIAS" --ks-pass "file:$FOODIE_SIGN_DIR/password" --out build/foodie.apk build/aligned.apk
"$FOODIE_BUILD_TOOLS/apksigner" verify --verbose build/foodie.apk
"$FOODIE_BUILD_TOOLS/zipalign" -c 4 build/foodie.apk
# Publish the complete signed APK atomically. Its embedded version is published with it.
if [ "${FOODIE_PUBLISH_APK:-0}" = "1" ]; then
    mkdir -p ../downloads
    cp build/foodie.apk ../downloads/.foodie.apk.new
    chmod 644 ../downloads/.foodie.apk.new
    mv ../downloads/.foodie.apk.new ../downloads/foodie.apk
fi
