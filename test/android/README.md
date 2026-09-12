# Voice regression fixtures

`FoodieTests.java` exercises the packaged Java/JNI keyword model with 16 kHz mono PCM WAV fixtures. Use the matching sherpa-onnx v1.13.8 Linux JNI libraries for a desktop run. Android itself must still be tested on a physical tablet for microphone capture, foreground permissions, TTS, SpeechRecognizer and Doze.

Fixtures contain only synthetic test phrases, not household recordings:
- `wake-positive-natural.wav`: “Hey Foodie”, Piper en_US lessac medium.
- `wake-positive-christel.wav`: “Hey Foodie”, Microsoft da-DK Christel neural test synthesis.
- `wake-negative-google.wav`: “Hey Google”, Piper en_US lessac medium.
- `wake-negative-groceries.wav`: “Tilføj to bakker vindruer”, Piper da_DK talesyntese medium.

Model/API source: https://github.com/k2-fsa/sherpa-onnx/releases/tag/v1.13.8
Keyword model: https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models
Bundled files: `android/vendor-checksums.json`; licenses: `android/licenses`.

After downloading the model assets with `python3 scripts/fetch-android-deps.py`, set `MAD_JNI_DIR` to the matching release's Linux `lib` directory:

```bash
MAD_JNI_DIR=/path/to/sherpa-onnx/lib make test-android
```

Speech recovery regression (no Android device needed):

```bash
make test-android
```

The runner uses a temporary class directory and removes it afterwards. It also runs `VoicePresentationTests`.

This covers provider selection priorities, bounded recovery, language failures, microphone denial, the exact five-second follow-up window, late callbacks and recognition finishing after a sentence started in time. Actual recognition still needs a physical Android device.
