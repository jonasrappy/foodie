const assert = require('node:assert/strict');
const { chromium } = require(process.env.MAD_PLAYWRIGHT_MODULE || 'playwright');

(async () => {
  const browser = await chromium.launch({ executablePath: process.env.MAD_CHROMIUM_PATH, headless: true, args: ['--no-sandbox'] });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    await page.goto(process.env.MAD_TEST_URL);
    await page.locator('#password').fill(process.env.MAD_TEST_PASSWORD);
    await page.locator('#login-form [type=submit]').click();
    await page.locator('#app-view').waitFor({ state: 'visible' });
    const language = await page.locator('html').getAttribute('lang');
    assert(['en', 'da'].includes(language));
    const identifiers = ['piece', 'liter', 'milliliter', 'kilogram', 'gram', 'pack', 'bag', 'can', 'bottle', 'bunch', 'tray', 'crate'];
    assert.deepEqual(await page.locator('#shopping-unit option').evaluateAll(options => options.map(option => option.value)), identifiers);
    assert.deepEqual(await page.locator('#edit-unit option').evaluateAll(options => options.map(option => option.value)), identifiers);
    const pack = await page.evaluate(() => window.FOODIE_I18N);
    assert.equal(pack.voice, undefined);
    assert.equal(pack.messages['Shopping list'], language === 'da' ? 'Indkøbsliste' : 'Shopping list');
    assert.equal(pack.messages['Indkøbsliste'], undefined);

    // Simulate a successful old-client request whose HTTP response was lost.
    const name = 'language upgrade ' + Date.now();
    const requestID = 'language-upgrade-' + Date.now();
    const before = await page.evaluate(async ({ name, requestID }) => {
      const response = await fetch('/api/items', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + localStorage.getItem('mad.token') }, body: JSON.stringify({ kind: 'shopping', texts: [name], quantity: 2, unit: 'pakker', request_id: requestID }) });
      if (!response.ok) throw new Error('Could not create upgrade fixture');
      return response.json();
    }, { name, requestID });
    assert.equal(before.shopping.find(item => item.text === name).unit, 'pack');
    await context.route('**/api/**', route => route.abort());
    await page.evaluate(({ name, requestID, state }) => {
      const item = { kind: 'shopping', texts: [name], quantity: 2, unit: 'pakker', request_id: requestID };
      localStorage.setItem('mad.outbox', JSON.stringify([item]));
      const existing = state.shopping.find(item => item.text === name); existing.unit = 'pakker';
      localStorage.setItem('mad.state', JSON.stringify(state));
      localStorage.setItem('mad.draft.amount', JSON.stringify({ quantity: '3', unit: 'bakker' }));
      localStorage.setItem('mad.draft.shopping', 'draft product');
    }, { name, requestID, state: before });
    await page.reload();
    await page.locator('#app-view').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#shopping-unit').inputValue(), 'tray');
    assert.equal(await page.locator('#shopping-quantity').inputValue(), '3');
    assert.equal(await page.locator('#shopping-input').inputValue(), 'draft product');
    assert(await page.locator('#shopping-list').innerText().then(text => text.includes(language === 'da' ? '2 pakker' : '2 packs')));
    await context.unroute('**/api/**');
    await page.reload();
    await page.waitForFunction(() => JSON.parse(localStorage.getItem('mad.outbox') || '[]').length === 0);
    const after = await page.evaluate(async () => (await fetch('/api/state', { headers: { Authorization: 'Bearer ' + localStorage.getItem('mad.token') } })).json());
    assert.equal(after.revision, before.revision);
    assert.equal(after.shopping.filter(item => item.text === name).length, 1);
    assert.equal(after.shopping.find(item => item.text === name).unit, 'pack');
    assert.deepEqual(errors, []);
    console.log(`PASS: ${language} English source keys and unit IDs, localized labels, legacy drafts/offline state and durable request replay.`);
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
