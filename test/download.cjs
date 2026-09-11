'use strict';
// Read-only regression: the APK must go to the browser's download manager,
// including when the previous Blob-based path is unavailable.
const { chromium } = require(process.env.MAD_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs'), crypto = require('node:crypto');
(async () => {
  const base = process.env.MAD_TEST_URL, password = process.env.MAD_TEST_PASSWORD;
  const apk = process.env.MAD_TEST_APK;
  if (!base || !password || !apk) throw Error('Set MAD_TEST_URL, MAD_TEST_PASSWORD and MAD_TEST_APK. This test does not modify lists.');
  const browser = await chromium.launch({headless:true, executablePath:process.env.MAD_CHROMIUM_PATH || undefined, args:['--no-sandbox']});
  try {
    const context = await browser.newContext({viewport:{width:1280,height:800}, hasTouch:true, serviceWorkers:'block',
      userAgent:'Mozilla/5.0 (Linux; Android 14; SM-X200) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/30.0 Chrome/143.0.0.0 Safari/537.36'});
    const page = await context.newPage(), errors = [], requests = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('request', request => {if (new URL(request.url()).pathname === '/api/download/android') requests.push(request);});
    await page.addInitScript(() => {URL.createObjectURL = () => {throw Error('Blob downloads are unavailable in this regression test');};});
    await context.route('**/api/download/android/session', route => route.abort());
    await page.goto(base);
    assert.equal(await page.locator('#install').isVisible(), false);
    await page.locator('#password').fill(password); await page.locator('#login-form button').click();
    await page.waitForFunction(() => document.querySelector('#sync-label').textContent === 'Synkroniseret');
    const state = () => page.evaluate(() => fetch('/api/state', {headers:{Authorization:'Bearer '+localStorage.getItem('mad.token')}}).then(r=>r.json()));
    const before = await state();
    await page.locator('#install').click();
    await page.waitForFunction(() => !document.querySelector('#toast').hidden && document.querySelector('#toast').textContent.includes('Ingen forbindelse'));
    assert.equal(await page.locator('#install-dialog').count(), 0);
    await context.unroute('**/api/download/android/session');
    await page.evaluate(() => dispatchEvent(new Event('online')));
    await page.waitForFunction(() => document.querySelector('#install').href.includes('/api/download/android?grant='));
    assert.equal(await page.locator('#install').evaluate(element => element.tagName), 'A');
    await context.clearCookies();
    const directURL = new URL(await page.locator('#install').getAttribute('href'));
    assert.equal((await context.cookies()).length, 0);
    const direct = await context.request.get(directURL.href, {headers:{Range:'bytes=0-15'}});
    assert.equal(direct.status(),206);
    assert.deepEqual(await direct.body(),fs.readFileSync(apk).subarray(0,16));
    assert.equal((await context.request.get(base + '/api/state?grant=' + directURL.searchParams.get('grant'))).status(),401);
    assert.equal((await context.request.post(base + '/api/download/android/session')).status(),401);
    assert.equal((await context.request.get(base + '/api/state')).status(),401);
    assert.equal(requests.filter(r => r.method() === 'GET').length, 0, 'APK was fetched before tapping Download');
    const downloading = page.waitForEvent('download');
    await page.locator('#install').click();
    const download = await downloading;
    assert.equal(download.suggestedFilename(), 'foodie.apk');
    assert.equal(new URL(download.url()).pathname, '/api/download/android');
    assert(new URL(download.url()).searchParams.has('grant'));
    assert(/^\d+$/.test(new URL(download.url()).searchParams.get('t')));
    const path = await download.path();
    const hash = path => crypto.createHash('sha256').update(fs.readFileSync(path)).digest('hex');
    assert.equal(hash(path),hash(apk));
    if (process.env.MAD_TEST_LINK_FILE) fs.writeFileSync(process.env.MAD_TEST_LINK_FILE, download.url(), {mode:0o600});
    // Chromium may hand downloads to its manager before emitting a page request.
    // If one is emitted, APK bytes must never go through JavaScript fetch/XHR.
    assert(!requests.some(r => r.method() === 'GET' && ['fetch','xhr'].includes(r.resourceType())));
    assert.match(await page.locator('#toast').textContent(), /Download starter/);
    assert.equal(await page.locator('dialog[open]').count(),0);
    await page.reload();
    await page.waitForFunction(() => document.querySelector('#sync-label').textContent === 'Synkroniseret');
    assert.deepEqual(await state(),before); assert.deepEqual(errors,[]);
    console.log('PASS: one-tap header APK link without cookies, Blob or install dialog; fresh timestamp, exact APK hash, download resume from a separate client, visible error recovery, persistent login and unchanged household lists.');
  } finally {await browser.close();}
})().catch(error => {console.error(error); process.exit(1);});
