# Tests

All test code, runners and fixtures live in this directory:

- `go/_packages`: Go tests, grouped by the application package they exercise.
- `browser`: Playwright suites for the web app and APK downloads.
- `android`: native voice policy, presentation and keyword-model tests.
- `fixtures`: synthetic compatibility data. These credentials are not application defaults.

## Go

```sh
make test check
```

`make test` runs the complete Go suite with the race detector. `make check` runs Go vet, including the test code, and verifies dependencies.

Some tests exercise private package functions, database migrations and concurrent writes. `go/run.py` uses Go's `-overlay` option to compile the files in their application packages without copying them into the source tree or exporting private functions. The `_packages` directory is excluded from Go's normal package discovery. The overlay is temporary and removed after the command finishes. Runtime fixtures are read from `test/fixtures`.

Run a selected package or test with:

```sh
python3 test/go/run.py test -race -run TestVoice ./internal/web
```

Use this runner or `make test` for the full suite. Bare `go test ./...` does not include the centrally stored tests.

## Browser

Install Playwright and Chromium, then run the isolated test runner:

```sh
npm ci --prefix test/browser --ignore-scripts
npm exec --prefix test/browser -- playwright install chromium
make test-browser
```

The runner generates fresh credentials and starts temporary English and Danish instances on unused loopback ports. It deletes their databases afterwards. It does not use the local household configuration or connect to a production server.

The suites cover quantities, all units, edits, purchased flags, bot output, two-device sync, offline retry, duplicate prevention, delete confirmation, tablet layouts, login persistence and APK download transport. The voice presentation suite simulates native callbacks and checks the 3D scene, microphone levels, reduced motion, cancellation and WebGL context loss.

For download transport, the runner uses `downloads/foodie.apk`, or `android/build/foodie.apk` if no published APK exists. Without either file it creates a synthetic transport fixture. That fixture is not an installable app.

Optional environment variables:

- `MAD_PLAYWRIGHT_MODULE`: path to an existing Playwright module.
- `MAD_CHROMIUM_PATH`: path to a Chromium executable.
- `MAD_SCREENSHOTS`: directory for screenshots you want to keep.

The individual `.cjs` scripts also accept `MAD_TEST_URL`, `MAD_TEST_PASSWORD` and `MAD_TEST_BOT_TOKEN` for a manually prepared test instance. Some scripts perform real mutations. Use the isolated runner unless you specifically need a separate test server.

`android-update.cjs` checks authenticated update discovery, newer-version comparison, the signed download button and the one-time migration dialog for older APKs.

`language.cjs` runs in English and Danish. It checks English source keys and unit IDs, translated dropdown labels, legacy drafts, cached state and an offline retry whose original request already succeeded. The Go suite checks every spoken unit alias, isolates confirmations and number words by language, and verifies the SQLite migration and pending voice confirmations.

## Android

```sh
make test-android
```

This runs the Java policy and presentation tests in a temporary directory. See [android/README.md](android/README.md) for keyword-model fixtures and JNI setup. Microphone capture, screen wake, speech recognition, TTS and the Android installer still need a physical-device check.
