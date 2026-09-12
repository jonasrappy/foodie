# Third-party components

| Component | Version | License | Source |
| --- | --- | --- | --- |
| Three.js | 0.186.0 | MIT | https://github.com/mrdoob/three.js |
| sherpa-onnx | 1.13.8 | Apache-2.0 | https://github.com/k2-fsa/sherpa-onnx |
| ONNX Runtime | Distributed with the pinned sherpa-onnx Android release | MIT | https://github.com/microsoft/onnxruntime |
| GigaSpeech keyword model | 3.3M, 2024-01-01 | Apache-2.0 | https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models |

The Three.js license is included in the web bundle. Android license notices are in `android/licenses` and are packaged into each APK. `android/vendor-checksums.json` identifies the exact model, native libraries and Java bindings used by the build.

The synthetic WAV files under `test/android/audio` are wake-word regression fixtures. They are not household recordings. Their generation sources are listed in `test/android/README.md`.

Go and build-tool dependencies are listed in `go.mod` and the npm lockfiles. They retain their upstream licenses.
