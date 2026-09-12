'use strict';
// Synthetic native callbacks against an isolated HTTP instance; no microphone or list mutations.
const { chromium } = require(process.env.MAD_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
(async () => {
  const base = process.env.MAD_TEST_URL, password = process.env.MAD_TEST_PASSWORD;
  if (!base || !password) throw Error('Set MAD_TEST_URL and MAD_TEST_PASSWORD for an isolated test instance.');
  const browser = await chromium.launch({ headless: true, executablePath: process.env.MAD_CHROMIUM_PATH || undefined, args: ['--no-sandbox', '--enable-unsafe-swiftshader'] });
  try {
    const options = { viewport: { width: 1280, height: 800 }, hasTouch: true, serviceWorkers: 'block' };
    const context = await browser.newContext({ ...options, userAgent: 'Mozilla/5.0 (Linux; Android 14; SM-X200) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 MadTablet/6.0' });
    const page = await context.newPage(), errors = [], nativeCommands = [];
    page.on('pageerror', e => errors.push(e.message));
    const cdp = await context.newCDPSession(page); await cdp.send('Page.enable');
    cdp.on('Page.frameRequestedNavigation', e => { if (e.url.startsWith('mad-app:')) nativeCommands.push(e.url); });
    const emit = (phase, text = '', extra = {}) => page.evaluate(detail => dispatchEvent(new CustomEvent('mad-voice-status', { detail })), { running: true, visible: true, session: 1, phase, text, level: 0, ...extra });
    const open = () => page.locator('#foodie-dialog').evaluate(el => el.open);
    const mouth = () => page.locator('.foodie-mouth').evaluate(el => ({ opacity: getComputedStyle(el).opacity, animation: getComputedStyle(el).animationName }));
    const login = async p => { await p.locator('#password').fill(password); await p.locator('#login-form button').click(); await p.waitForFunction(() => document.querySelector('#sync-label').textContent === 'Synkroniseret'); };
    const dismissCount = () => nativeCommands.filter(x => x === 'mad-app://voice/dismiss').length;
    const waitDismiss = async expected => { for (let n = 0; n < 30 && dismissCount() < expected; n++) await page.waitForTimeout(30); assert.equal(dismissCount(), expected); };
    await page.addInitScript(() => {
      window.__foodieDraws = 0;
      const original = HTMLCanvasElement.prototype.getContext;
      HTMLCanvasElement.prototype.getContext = function(...args) {
        const gl = original.apply(this, args);
        if (gl && args[0] === 'webgl2' && !gl.__counted) {
          gl.__counted = true;
          for (const method of ['drawElements', 'drawArrays']) {
            const draw = gl[method].bind(gl);
            gl[method] = (...values) => { window.__foodieDraws++; return draw(...values); };
          }
        }
        return gl;
      };
    });
    await page.goto(base);
    await emit('speaking', 'Ingen adgang'); assert.equal(await open(), false, 'No avatar before login');
    await login(page);
    await emit('idle', 'Lytter efter Hey Foodie', { visible: false, session: 0 }); assert.equal(await open(), false);
    await page.locator('#shopping-input').fill('Behold min kladde');
    await emit('waking', 'Hey Foodie'); assert.equal(await open(), true);
    await page.locator('.foodie-orb.has-3d').waitFor();
    assert.equal(await page.evaluate(() => document.activeElement.matches('input,textarea')), false);
    assert.equal((await mouth()).opacity, '0');
    await emit('thinking', 'Hey. Hvad skal jeg tilføje til indkøbslisten?');
    assert.notEqual((await mouth()).animation, 'foodie-talk', 'Queued speech must not animate');
    await emit('speaking', 'Hey. Hvad skal jeg tilføje til indkøbslisten?');
    assert.deepEqual(await mouth(), { opacity: '1', animation: 'foodie-talk' });
    const shots = process.env.MAD_SCREENSHOTS || '/tmp/mad-browser'; fs.mkdirSync(shots, { recursive: true });
    await page.waitForTimeout(450);
    await page.screenshot({ path: shots + '/foodie-speaking-1280x800.png' });
    await emit('listening', 'Hvad skal på listen?');
    assert.equal(await page.locator('#foodie-title').textContent(), 'Jeg lytter');
    assert.equal((await mouth()).opacity, '0');
    assert.equal(await page.locator('.foodie-body').evaluate(el => getComputedStyle(el).animationName), 'foodie-attentive');
    const livingBefore = await page.locator('.foodie-canvas').screenshot();
    await page.waitForTimeout(400);
    assert.notDeepEqual(await page.locator('.foodie-canvas').screenshot(), livingBefore, 'The 3D character must stay alive while listening');
    await emit('hearing', '', { level: .85 });
    assert.equal(await page.locator('#foodie-dialog').evaluate(el => el.style.getPropertyValue('--voice-level')), '0.85');
    await page.screenshot({ path: shots + '/foodie-listening-1280x800.png' });
    await emit('hearing', '', { level: 900 });
    assert.equal(await page.locator('#foodie-dialog').evaluate(el => el.style.getPropertyValue('--voice-level')), '1.00');
    await emit('thinking', '<img src=x onerror=alert(1)>');
    assert.equal(await page.locator('#foodie-caption').textContent(), '<img src=x onerror=alert(1)>');
    assert.equal(await page.locator('#foodie-caption img').count(), 0);
    assert.equal(await page.locator('#foodie-dialog').evaluate(el => el.style.getPropertyValue('--voice-level')), '0.00');
    await emit('speaking', 'Tilføjet 2 pakker vindruer til indkøbslisten. Var der andet?');
    for (const [width, height] of [[1280,800],[1280,720],[1200,750],[1024,600],[960,600],[853,533],[800,480],[740,360],[390,844]]) {
      await page.setViewportSize({ width, height });
      const bounds = await page.evaluate(() => {
        const dialog = document.querySelector('#foodie-dialog'), nodes = ['.foodie-stage', '.foodie-copy', '#foodie-close'];
        return { scroll: dialog.scrollHeight <= dialog.clientHeight + 1, boxes: nodes.map(s => { const r = document.querySelector(s).getBoundingClientRect(); return r.left >= 0 && r.top >= 0 && r.right <= innerWidth + 1 && r.bottom <= innerHeight + 1; }) };
      });
      assert(bounds.scroll && bounds.boxes.every(Boolean), `Scene clipped at ${width}x${height}: ${JSON.stringify(bounds)}`);
      if (width === 853 || width === 740) await page.screenshot({ path: shots + `/foodie-speaking-${width}x${height}.png` });
    }
    await page.setViewportSize(options.viewport);
    await emit('listening', 'Sig en vare eller nej tak');
    await emit('idle', '', { visible: false });
    await page.locator('#foodie-dialog').waitFor({ state: 'hidden' });
    assert.equal(dismissCount(), 0, 'Native timeout must not send cancellation back');
    const sleepingDraws = await page.evaluate(() => window.__foodieDraws);
    assert(sleepingDraws > 0, '3D frames were rendered');
    await page.waitForTimeout(250);
    assert.equal(await page.evaluate(() => window.__foodieDraws), sleepingDraws, 'Closed avatar must stop rendering');
    assert.equal(await page.locator('#shopping-input').inputValue(), 'Behold min kladde');
    assert.equal(await page.evaluate(() => document.activeElement.matches('input,textarea')), false, 'Keyboard stays closed');
    await emit('speaking', 'For sent'); assert.equal(await open(), false, 'Ended session cannot reopen');
    await emit('speaking', 'Hej igen', { session: 2 });
    await page.locator('.foodie-canvas').evaluate(canvas => {
      window.__foodieContextExtension = canvas.getContext('webgl2').getExtension('WEBGL_lose_context');
      window.__foodieContextExtension.loseContext();
    });
    await page.waitForFunction(() => !document.querySelector('.foodie-orb').classList.contains('has-3d'));
    assert.equal(await page.locator('.foodie-character').evaluate(el => getComputedStyle(el).visibility), 'visible');
    await page.evaluate(() => window.__foodieContextExtension.restoreContext());
    await page.locator('.foodie-orb.has-3d').waitFor();
    await page.locator('#foodie-close').click(); await waitDismiss(1);
    await emit('listening', 'Gammel callback', { session: 2 }); assert.equal(await open(), false);
    await emit('listening', 'Ny samtale', { session: 3 }); assert.equal(await open(), true);
    await page.evaluate(() => dispatchEvent(new Event('mad-back'))); await waitDismiss(2);
    await emit('speaking', 'Kort pause', { session: 4 });
    await emit('idle', '', { session: 4, visible: false });
    await emit('listening', 'Næste samtale', { session: 5 });
    await page.waitForTimeout(300); assert.equal(await open(), true, 'Old exit timer must not close a new session');
    await emit('idle', '', { session: 4, visible: false }); assert.equal(await open(), true, 'Stale frame must be ignored');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await emit('speaking', 'Rolige bevægelser', { session: 5 });
    assert.equal((await mouth()).animation, 'none');
    const still = await page.locator('.foodie-canvas').screenshot();
    await page.waitForTimeout(250);
    assert.deepEqual(await page.locator('.foodie-canvas').screenshot(), still, 'Reduced motion must stop continuous 3D movement');
    assert.equal(await page.locator('.foodie-body').evaluate(el => getComputedStyle(el).animationName), 'none');
    await emit('idle', '', { session: 5, visible: false }); assert.equal(await open(), false);
    await emit('listening', 'Sidste samtale', { session: 6 });
    await page.route('**/api/state', route => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"Log ind igen"}' }));
    await page.evaluate(() => dispatchEvent(new Event('online')));
    await page.locator('#login-view').waitFor(); assert.equal(await open(), false);
    await emit('speaking', 'En gammel stemme', { session: 6 }); assert.equal(await open(), false);
    await page.waitForTimeout(150); assert.equal(dismissCount(), 2, 'Logout must not also dismiss a stopped service');
    for (const version of [0, 5]) {
      const legacy = await browser.newContext({ ...options, ...(version ? { userAgent: 'Mozilla/5.0 MadTablet/5.0' } : {}) });
      const p = await legacy.newPage(); await p.goto(base); await login(p);
      await p.evaluate(() => dispatchEvent(new CustomEvent('mad-voice-status', { detail: { running: true, visible: true, session: 99, phase: 'speaking', text: 'Legacy' } })));
      assert.equal(await p.locator('#foodie-dialog').evaluate(el => el.open), false);
      await legacy.close();
    }
    assert.deepEqual(errors, []);
    console.log('PASS: live 3D movement and reduced-motion rendering, authenticated native-only avatar, real playback phases, listening/RMS, safe captions, nine viewport bounds, keyboard/draft preservation, timeout, close/back commands, stale sessions, reduced motion, logout and legacy clients.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
