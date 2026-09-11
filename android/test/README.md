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

Example (set MAD_JNI_DIR to the release's Linux `lib` directory):

```bash
mkdir -p /tmp/mad-jni-tests
javac -d /tmp/mad-jni-tests android/vendor/com/k2fsa/sherpa/onnx/*.java android/test/FoodieTests.java
java -Djava.library.path="$MAD_JNI_DIR" -cp /tmp/mad-jni-tests app.foodie.mobile.FoodieTests "$PWD/android/assets/foodie" "$PWD"/android/test/audio/*.wav
```

Speech recovery regression (no Android device needed):

```bash
mkdir -p /tmp/mad-speech-tests
javac -d /tmp/mad-speech-tests android/src/app/foodie/mobile/SpeechPolicy.java android/src/app/foodie/mobile/SpeechWindow.java android/test/SpeechPolicyTests.java android/test/SpeechWindowTests.java
java -cp /tmp/mad-speech-tests app.foodie.mobile.SpeechPolicyTests
java -cp /tmp/mad-speech-tests app.foodie.mobile.SpeechWindowTests
```

This covers provider selection priorities, bounded recovery, language failures, microphone denial, the exact five-second follow-up window, late callbacks and recognition finishing after a sentence started in time. Actual recognition still needs a physical Android device.
