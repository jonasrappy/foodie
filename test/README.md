# Browser tests

Build the server, install Playwright and run the isolated test runner:

```sh
make build
npm ci --prefix test --ignore-scripts
npm exec --prefix test -- playwright install chromium
python3 scripts/test-browser.py
```

The runner generates fresh credentials and starts temporary English and Danish instances on unused loopback ports. It deletes their databases afterwards. It does not use the local household configuration or connect to a production server.

The suites cover quantities, all units, edits, purchased flags, bot output, two-device sync, offline retry, duplicate prevention, delete confirmation, tablet layouts, login persistence and APK download transport. The voice presentation suite simulates native callbacks and checks the 3D scene, microphone levels, reduced motion, cancellation and WebGL context loss. These simulations do not replace testing a microphone or screen wake on a physical Android tablet.

If `android/build/foodie.apk` exists, the download test checks those exact bytes. Otherwise the runner uses a synthetic transport fixture; it does not claim that fixture is an installable app.

Optional environment variables:

- `MAD_PLAYWRIGHT_MODULE`: path to an existing Playwright module.
- `MAD_CHROMIUM_PATH`: path to a Chromium executable.
- `MAD_SCREENSHOTS`: directory for screenshots you want to keep.

The individual `.cjs` scripts also accept `MAD_TEST_URL`, `MAD_TEST_PASSWORD` and `MAD_TEST_BOT_TOKEN` for a manually prepared test instance. Those scripts perform real mutations. Use the isolated runner unless you specifically need a separate test server.

`android-update.cjs` checks authenticated update discovery, newer-version comparison, the signed download button and the one-time migration dialog for older APKs. The Android installer and unknown-source permission screen still need a physical-device check.

`language.cjs` runs in English and Danish. It checks English source keys and unit IDs, translated dropdown labels, legacy drafts, cached state and an offline retry whose original request already succeeded. Go tests separately check every spoken unit alias, isolate confirmations and number words by language, and verify the SQLite unit migration and pending voice confirmations.
