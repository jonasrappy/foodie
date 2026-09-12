'use strict';
const {chromium} = require(process.env.MAD_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({headless:true, executablePath:process.env.MAD_CHROMIUM_PATH || undefined,args:['--no-sandbox']});
  try {
    for (const [ua,code,visible,legacy] of [
      ['MadTablet/7.0 FoodieAndroid/8',9,true,false],
      ['MadTablet/7.0 FoodieAndroid/8',8,false,false],
      ['MadTablet/7.0 FoodieAndroid/8',7,false,false],
      ['MadTablet/7.0',8,true,true],
      ['',9,false,false]
    ]) {
      const context=await browser.newContext({userAgent:'Mozilla/5.0 '+ua,serviceWorkers:'block'});
      const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
      let checks=0,apkRequests=0;
      await page.route('**/api/android/release',route=>{checks++;return route.fulfill({json:{version_code:code,version_name:'1.3.3'}})});
      page.on('request',r=>{if(new URL(r.url()).pathname==='/api/download/android')apkRequests++});
      await page.goto(process.env.MAD_TEST_URL);
      assert.equal(await page.locator('#install').isVisible(),false);assert.equal(checks,0);
      await page.locator('#password').fill(process.env.MAD_TEST_PASSWORD);await page.locator('#login-form button').click();
      await page.waitForFunction(()=>document.querySelector('#sync-label').textContent==='Synkroniseret');
      if (ua) await page.waitForFunction(()=>!document.querySelector('#app-view').hidden);
      if (visible) {
        await page.getByRole('link',{name:'Opdater Foodie'}).waitFor({state:'visible'});
        assert.equal(apkRequests,0,'must not download without tapping');
        if (legacy) {
          await page.locator('#install').click();await page.locator('#android-update-dialog').waitFor({state:'visible'});
          assert.equal(apkRequests,0);
        } else {
          await page.waitForFunction(()=>document.querySelector('#install').href.includes('grant='));
          const downloaded=page.waitForEvent('download');await page.locator('#install').click();
          assert.equal((await downloaded).suggestedFilename(),'foodie.apk');
        }
      } else if (ua) {
        await page.waitForTimeout(250);assert.equal(await page.locator('#install').isVisible(),false);
      } else {
        assert.equal(checks,0);assert.equal(await page.locator('#install').isVisible(),true);
      }
      assert.deepEqual(errors,[]);await context.close();
    }
    console.log('PASS: APK update checks after login only, newer versions only, no automatic download, signed download click, legacy migration dialog and unchanged browser install link.');
  } finally {await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
