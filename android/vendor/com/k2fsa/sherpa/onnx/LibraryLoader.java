package com.k2fsa.sherpa.onnx;
// Android resolves the packaged ABI-specific JNI library. No network loading.
public final class LibraryLoader {
 private static boolean loaded;
 static synchronized void maybeLoad() {
  if (!loaded) { System.loadLibrary("sherpa-onnx-jni"); loaded=true; }
 }
}
