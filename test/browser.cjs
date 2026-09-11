'use strict';
const { chromium } = require(process.env.MAD_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
(async () => {
  const base = process.env.MAD_TEST_URL, password = process.env.MAD_TEST_PASSWORD, bot = process.env.MAD_TEST_BOT_TOKEN;
  if (!base || !password || !bot) throw Error('Set MAD_TEST_URL, MAD_TEST_PASSWORD and MAD_TEST_BOT_TOKEN for an isolated test instance.');
  const browser = await chromium.launch({ headless: true, executablePath: process.env.MAD_CHROMIUM_PATH || undefined, args: ['--no-sandbox'] });
  try {
    const options = { viewport: { width: 1280, height: 800 }, hasTouch: true, serviceWorkers: 'block' };
    const native = await browser.newContext({ ...options, userAgent: 'Mozilla/5.0 (Linux; Android 14; SM-X200) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 MadTablet/3.0' });
    const web = await browser.newContext({ ...options, viewport: { width: 1200, height: 750 } });
    const p = await native.newPage(), p2 = await web.newPage(), errors = [];
    for (const page of [p, p2]) {
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(base);
      assert.equal(await page.locator('#install').isVisible(), false);
      assert.equal(await page.locator('#native-settings').isVisible(), false);
      await page.locator('#password').fill(password); await page.locator('#login-form button').click();
      await page.waitForFunction(() => document.querySelector('#sync-label').textContent === 'Synkroniseret');
    }
    assert.equal(await p.locator('#install').isVisible(), false);
    assert.equal(await p.locator('#native-settings').isVisible(), true);
    assert.equal(await p.locator('#voice').isVisible(), true);
    assert.equal(await p2.locator('#voice').isVisible(), false);
    await p.evaluate(() => dispatchEvent(new CustomEvent('mad-voice-status', {detail:{running:true,status:'Lytter efter Hey Foodie'}})));
    assert.equal(await p.locator('#voice').getAttribute('aria-pressed'), 'true');
    assert.equal(await p2.locator('#install').isVisible(), true);
    assert.equal((await fetch(base + '/api/download/android')).status, 401);
    await p2.waitForFunction(() => document.querySelector('#install').href.includes('/api/download/android?grant='));
    assert.equal(await p2.locator('#install').evaluate(element => element.tagName), 'A');
    assert.equal(await p2.locator('#install-dialog').count(), 0);
    const prefix = 'Tablet ' + Date.now() + ' ', name = prefix + 'mælk';
    const saved = () => p.waitForFunction(() => JSON.parse(localStorage.getItem('mad.outbox') || '[]').length === 0 && !document.querySelector('.item.busy'));
    const add = async (kind, text, quantity = '1', unit = 'stk.') => {
      await p.locator('#' + kind + '-input').fill(text);
      if (kind === 'shopping') { await p.locator('#shopping-quantity').fill(quantity); await p.locator('#shopping-unit').selectOption(unit); }
      await p.locator('.' + kind + ' .add-button').click();
    };
    assert.equal(await p.locator('#shopping-quantity').inputValue(), '1');
    assert.deepEqual(await p.locator('#shopping-unit option').allTextContents(), ['stk.', 'liter', 'milliliter', 'kilo', 'gram', 'pakker', 'poser', 'dåser', 'flasker', 'bundter', 'bakker', 'kasser']);
    await p.getByRole('button', { name: 'Flere', exact: true }).click(); assert.equal(await p.locator('#shopping-quantity').inputValue(), '2');
    await p.getByRole('button', { name: 'Færre', exact: true }).click();
    let releaseAdd, addStarted;
    const addGate = new Promise(resolve => releaseAdd = resolve), started = new Promise(resolve => addStarted = resolve);
    await p.route('**/api/items', async route => { if (route.request().method() === 'POST') { addStarted(); await addGate; } await route.continue(); });
    await add('shopping', name, '1.5', 'liter'); await started;
    assert.equal(await p.locator('#shopping-input').inputValue(), '');
    assert.equal(await p.locator('#shopping-input').isEnabled(), true);
    assert.equal(await p.evaluate(() => document.activeElement.matches('input,textarea,select')), false);
    assert.equal(await p.locator('#toast').textContent(), 'Tilføjet');
    assert.equal(await p.locator('.addition-flight').count(), 1);
    assert.equal(await p.locator('.item.queued').count(), 1);
    assert.equal(await p.locator('#shopping-quantity').inputValue(), '1'); assert.equal(await p.locator('#shopping-unit').inputValue(), 'stk.');
    await add('shopping', prefix + 'bananer', '6');
    assert.equal(await p.locator('.item.queued').count(), 2);
    releaseAdd(); await saved(); await p.unroute('**/api/items');
    assert.equal(await p.locator('#shopping-list .item-name').first().textContent(), prefix + 'bananer');
    await p2.waitForFunction(text => document.querySelector('#shopping-list .item-name')?.textContent === text, prefix + 'bananer');
    const badge = p2.getByRole('button', { name: 'Ret antal og enhed for ' + name, exact: true });
    await badge.waitFor(); assert.equal(await badge.textContent(), '1,5 liter');
    await badge.click(); await p2.locator('#edit-quantity').fill('500'); await p2.locator('#edit-unit').selectOption('gram');
    await p2.locator('#amount-form [type=submit]').click(); await p2.locator('#amount-dialog').waitFor({ state: 'hidden' });
    await p.waitForFunction(name => [...document.querySelectorAll('.amount-button')].some(button => button.getAttribute('aria-label') === 'Ret antal og enhed for ' + name && button.textContent === '500 gram'), name);
    const getBot = () => fetch(base + '/api/v1/requirements', { headers: { Authorization: 'Bearer ' + bot } }).then(response => response.json());
    let result = await getBot(); assert(result.required_shopping_items.includes('500 gram ' + name));
    let releaseCheck, checkStarted;
    const checkGate = new Promise(resolve => releaseCheck = resolve), checking = new Promise(resolve => checkStarted = resolve);
    await p.route('**/api/items/*', async route => { checkStarted(); await checkGate; await route.continue(); });
    await p.getByRole('checkbox', { name: 'Købt: ' + name, exact: true }).check(); await checking;
    assert.equal(await p.getByRole('checkbox', { name: 'Købt: ' + name, exact: true }).isChecked(), true);
    assert.equal(await p.locator('.item.busy.checked').count(), 1);
    releaseCheck(); await saved(); await p.unroute('**/api/items/*');
    await p2.waitForFunction(name => [...document.querySelectorAll('.item-check')].find(box => box.getAttribute('aria-label') === 'Købt: ' + name)?.checked, name);
    result = await getBot(); assert(!result.required_shopping_items.includes('500 gram ' + name)); assert(result.already_purchased_items.includes('500 gram ' + name));
    assert(result.shopping_items.some(item => item.text === name && item.quantity === 500 && item.unit === 'gram' && item.checked));
    // A conflicting optimistic toggle must roll back to the authoritative checked state.
    await p.route('**/api/items/*', route => route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'Ændret på en anden enhed.' }) }));
    await p.getByRole('checkbox', { name: 'Købt: ' + name, exact: true }).click();
    await p.waitForFunction(name => [...document.querySelectorAll('.item-check')].find(box => box.getAttribute('aria-label') === 'Købt: ' + name)?.checked, name);
    await p.unroute('**/api/items/*');
    await add('meals', prefix + 'lasagne\n' + prefix + 'tacos'); await saved();
    await p.getByRole('button', { name: 'Ret ' + prefix + 'lasagne', exact: true }).click();
    assert.equal(await p.locator('#edit-amount-fields').isVisible(), false);
    await p.locator('#edit-name').fill(prefix + 'grøntsagslasagne'); await p.locator('#amount-form [type=submit]').click();
    await p2.getByRole('button', { name: 'Ret ' + prefix + 'grøntsagslasagne', exact: true }).waitFor();
    // Offline additions and quantities survive a page restart and replay once.
    await native.setOffline(true); await add('shopping', prefix + 'offline', '2', 'poser');
    assert.equal(await p.locator('.item.queued').count(), 1);
    let blockPosts = true;
    await p.route('**/api/items', route => blockPosts ? route.abort('internetdisconnected') : route.continue());
    await native.setOffline(false); await p.reload(); await p.locator('.item.queued').waitFor();
    assert.equal(await p.locator('.item.queued .amount-button').textContent(), '2 poser');
    blockPosts = false; await p.locator('#sync-control').click(); await saved(); await p.unroute('**/api/items');
    result = await getBot(); assert.equal(result.shopping_items.filter(item => item.text === prefix + 'offline').length, 1);
    // A committed POST with a lost response must also replay without duplicates.
    let loseResponse = true;
    await p.route('**/api/items', async route => { if (loseResponse) { loseResponse = false; await route.fetch(); await route.abort('connectionreset'); } else await route.continue(); });
    await add('shopping', prefix + 'lost response'); await p.waitForFunction(() => document.querySelector('#sync-label').textContent === 'Afventer net');
    await p.reload(); await saved(); await p.unroute('**/api/items');
    result = await getBot(); assert.equal(result.shopping_items.filter(item => item.text === prefix + 'lost response').length, 1);
    // Ensure long lists scroll inside each panel, leaving the composers on screen.
    await add('shopping', Array.from({ length: 20 }, (_, index) => prefix + 'lang vare med god plads til navnet ' + index).join('\n')); await saved();
    const screenshots = process.env.MAD_SCREENSHOTS || '/tmp/mad-browser'; fs.mkdirSync(screenshots, { recursive: true });
    for (const [width, height] of [[1280, 800], [1200, 750], [1024, 640], [960, 600], [1280, 380], [960, 320], [800, 600], [600, 800], [390, 844]]) {
      await p.setViewportSize({ width, height }); await p.waitForTimeout(60);
      const layout = await p.evaluate(() => ({ overflow: document.documentElement.scrollWidth > innerWidth, vertical: document.documentElement.scrollHeight > innerHeight, composers: [...document.querySelectorAll('.composer')].map(element => ({ bottom: element.getBoundingClientRect().bottom, width: element.clientWidth, scroll: element.scrollWidth })), lists: [...document.querySelectorAll('.list-area')].map(element => ({ height: element.clientHeight, scroll: element.scrollHeight })) }));
      assert(!layout.overflow, 'horizontal overflow at ' + width);
      if (width >= 600) { assert(!layout.vertical, 'page scrolling at ' + width + 'x' + height); for (const composer of layout.composers) { assert(composer.bottom <= height); assert(composer.scroll <= composer.width + 1); } }
      assert(layout.lists[0].scroll > layout.lists[0].height);
      await p.screenshot({ path: screenshots + '/tablet-tested-' + width + 'x' + height + '.png' });
    }
    await p.setViewportSize({ width: 1280, height: 800 });
    await p.getByRole('button', { name: 'Fjern ' + name, exact: true }).click();
    await p.locator('#delete-dialog').waitFor({ state: 'visible' });
    assert.equal(await p.locator('#delete-name').textContent(), name);
    assert.equal(await badge.count(), 1);
    await p.locator('#cancel-delete').click(); assert.equal(await badge.count(), 1);
    await p.getByRole('button', { name: 'Fjern ' + name, exact: true }).click();
    await p.locator('#confirm-delete').click();
    assert.equal(await p.locator('.removal-clone').count(), 1);
    await badge.waitFor({ state: 'detached' });
    await p.locator('.removal-clone').waitFor({ state: 'detached' });
    assert.equal(await p.locator('#toast').textContent(), 'Slettet');
    await p.locator('#shopping-input').fill('Min næste kladde'); await p.locator('#shopping-quantity').fill('2'); await p.locator('#shopping-unit').selectOption('bakker');
    await p.reload(); assert.equal(await p.locator('#shopping-input').inputValue(), 'Min næste kladde'); assert.equal(await p.locator('#shopping-quantity').inputValue(), '2'); assert.equal(await p.locator('#shopping-unit').inputValue(), 'bakker');
    assert.equal(await p.locator('#login-view').isVisible(), false); assert.deepEqual(errors, []);
    console.log('PASS: native/browser login and install visibility, quantity units and editing, instant queued additions, receipt/flight animation, keyboard blur and newest-first ordering, two-device SSE, optimistic checks and conflict rollback, bot purchased exclusions, meal editing, offline/restart queue, lost-response idempotency, nine viewport layouts, contained scrolling, delete confirmation/cancel/animation and persistent drafts/login.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
