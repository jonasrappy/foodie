'use strict';
const {chromium}=require(process.env.MAD_PLAYWRIGHT_MODULE||'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const base=process.env.MAD_TEST_URL,password=process.env.MAD_TEST_PASSWORD,bot=process.env.MAD_TEST_BOT_TOKEN;
 if(!base||!password||!bot)throw Error('Use test/browser/run.py with an isolated instance.');
 const browser=await chromium.launch({headless:true,executablePath:process.env.MAD_CHROMIUM_PATH||undefined,args:['--no-sandbox','--enable-unsafe-swiftshader']});
 try{
  const context=await browser.newContext({viewport:{width:1280,height:800},serviceWorkers:'block',userAgent:'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/143.0.0.0 Safari/537.36 MadTablet/7.0'});
  const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base);assert.equal(await page.locator('html').getAttribute('lang'),'en');
  await page.getByLabel('Household password').fill(password);await page.getByRole('button',{name:'Open our lists'}).click();
  await page.waitForFunction(()=>document.querySelector('#sync-label').textContent==='Synced');
  assert.equal(await page.locator('#shopping-heading').textContent(),'Shopping list');
  assert.equal(await page.locator('#meals-heading').textContent(),"Next week's meals");
  assert.deepEqual(await page.locator('#shopping-unit option').allTextContents(),['pieces','liters','milliliters','kilos','grams','packs','bags','cans','bottles','bunches','trays','crates']);
  await page.locator('#shopping-input').fill('grapes');await page.locator('#shopping-quantity').fill('2');await page.locator('#shopping-unit').selectOption({label:'packs'});
  await page.getByRole('button',{name:'Add item',exact:true}).click();
  await page.waitForFunction(()=>!document.querySelector('.item.queued'));
  assert.equal(await page.locator('.amount-button').first().textContent(),'2 packs');
  await page.getByRole('button',{name:'Edit quantity and unit for grapes'}).click();await page.locator('#edit-quantity').fill('1');await page.getByRole('button',{name:'Save changes'}).click();
  await page.waitForFunction(()=>document.querySelector('.amount-button').textContent==='1 pack');
  await page.getByRole('checkbox',{name:'Purchased: grapes'}).check();await page.waitForFunction(()=>!document.querySelector('.item.busy'));
  const response=await fetch(base+'/api/v1/requirements',{headers:{Authorization:'Bearer '+bot}});const data=await response.json();
  assert.deepEqual(data.required_shopping_items,[]);assert.deepEqual(data.already_purchased_items,['1 pack grapes']);assert.equal(data.timezone,'UTC');
  await page.evaluate(()=>dispatchEvent(new CustomEvent('mad-voice-status',{detail:{running:true,visible:true,session:1,phase:'listening',text:'What should go on the list?',level:0}})));
  await page.locator('.foodie-orb.has-3d').waitFor();assert.equal(await page.locator('#foodie-title').textContent(),"I'm listening");
  await page.evaluate(()=>dispatchEvent(new CustomEvent('mad-voice-status',{detail:{running:true,visible:false,session:1,phase:'idle',text:'',level:0}})));
  await page.locator('#foodie-dialog').waitFor({state:'hidden'});
  await page.getByRole('button',{name:'Remove grapes'}).click();await page.getByRole('button',{name:'Yes, delete'}).click();await page.locator('.item').waitFor({state:'hidden'});
  assert.deepEqual(errors,[]);
  console.log('PASS: default English login, headings, all unit labels, singular/plural editing, purchased bot exclusions, 3D listening caption and deletion.');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
