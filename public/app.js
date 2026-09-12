'use strict';
const languagePack = window.FOODIE_I18N;
const t = message => languagePack?.messages[message] ?? message;
const $ = id => document.getElementById(id);
const storage = {
  get(key) { try { return localStorage.getItem('mad.' + key); } catch { return null; } },
  set(key, value) { try { localStorage.setItem('mad.' + key, value); return true; } catch { return false; } },
  remove(key) { try { localStorage.removeItem('mad.' + key); } catch {} }
};
let token = storage.get('token');
let current = null, deferredState = null, streamController, retryTimer, toastTimer;
let connected = false, wakeLock, wakeEnabled = false, editing;
let updateCheckRunning = false, posting = false, postRetryTimer;
let saveErrorShown = false;
let downloadPreparation, downloadExpires = 0, downloadFailure = '', downloadRequested = false;
const additions = new Map(), mutations = new Map();
try { for (const item of JSON.parse(storage.get('outbox') || '[]')) if (item.request_id && ['shopping', 'meals'].includes(item.kind) && Array.isArray(item.texts)) { item.unit = normalizeLegacyUnit(item.unit); additions.set(item.request_id, item); } } catch {}
const nativeVersion = Number(navigator.userAgent.match(/MadTablet\/(\d+)/)?.[1] || 0);
const installedAndroidCode = Number(navigator.userAgent.match(/FoodieAndroid\/(\d+)/)?.[1] || 0);
let androidUpdateAvailable = false, androidUpdateChecking = false;
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');
const numberFormat = new Intl.NumberFormat(languagePack?.locale || 'en-US', { maximumFractionDigits: 2 });
function formatAmount(quantity, unit) {
  const forms=languagePack?.units[unit];
  return numberFormat.format(quantity)+' '+(forms?forms[Number(quantity)===1?0:1]:unit);
}
for(const option of document.querySelectorAll('select option')) {
 const forms=languagePack?.units[option.value];if(forms)option.textContent=forms[1];
}
const loadedVersion = document.querySelector('meta[name=app-version]')?.content;
function toast(message, success = false) {
  const notice = $('toast'); notice.textContent = message; notice.classList.toggle('success', success); notice.hidden = false;
  if (!reducedMotion.matches) {
    notice.getAnimations().forEach(animation => animation.cancel());
    notice.animate([{ opacity: 0, transform: 'translateX(-50%) translateY(10px) scale(.95)' }, { opacity: 1, transform: 'translateX(-50%) translateY(0) scale(1)' }], { duration: 240, easing: 'cubic-bezier(.16,1,.3,1)' });
  }
  clearTimeout(toastTimer); toastTimer = setTimeout(() => notice.hidden = true, success ? 1800 : 5500);
}
function status(online = connected) {
  connected = online;
  const busy = additions.size + mutations.size > 0;
  $('sync-dot').classList.toggle('waiting', !online || busy);
  $('sync-label').textContent = busy ? (online ? t('Saving …') : t('Waiting for connection')) : online ? t('Synced') : t('Offline');
  $('sync-control').title = online ? t('Your lists are up to date. Tap to sync again.') : t('No connection. Tap to try again.');
}
function showApp() {
  document.body.classList.add('authenticated');
  $('login-view').hidden = true; $('app-view').hidden = false;
  $('sync-control').hidden = false;
  $('native-settings').hidden = nativeVersion < 2;
  $('voice').hidden = nativeVersion < 3;
  $('wake').hidden = !!nativeVersion || !('wakeLock' in navigator);
  $('install').hidden = !!nativeVersion || matchMedia('(display-mode: standalone)').matches || !!navigator.standalone;
  if (nativeVersion) checkAndroidUpdate(); else prepareAndroidDownload();
  if (nativeVersion >= 6) prepareFoodieAvatar();
}
function showLogin() {
  androidUpdateAvailable = false;
  downloadExpires = 0; $('install').href = '#';
  if (nativeVersion >= 3) location.href = 'mad-app://voice/logout';
  token = null; current = null; deferredState = null; storage.remove('token'); storage.remove('state');
  streamController?.abort(); clearTimeout(retryTimer); clearTimeout(postRetryTimer);
  hideFoodie(true);
  foodieRenderer?.dispose(); foodieRenderer = null;
  document.querySelectorAll('dialog[open]').forEach(dialog => dialog.close());
  document.body.classList.remove('authenticated');
  $('app-view').hidden = true; $('login-view').hidden = false;
  for (const id of ['install', 'wake', 'voice', 'native-settings', 'sync-control']) $(id).hidden = true;
  for (const kind of ['shopping', 'meals']) $(kind + '-list').replaceChildren();
}
async function api(endpoint, options = {}) {
  let response;
  try {
    response = await fetch(endpoint, { ...options, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}), ...options.headers }, cache: 'no-store' });
  } catch { status(false); throw new Error(t('No connection. Your text is saved here. Try again when you\'re online.')); }
  const data = await response.json();
  if (!response.ok) {
    if (response.status === 401 && endpoint !== '/api/login') showLogin();
    const error = new Error(data.error || t('Couldn\'t save. Please try again.')); error.status = response.status; throw error;
  }
  return data;
}
function icon(name) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', '#' + name); svg.setAttribute('aria-hidden', 'true'); svg.append(use); return svg;
}
function animate(element) {
  if (!reducedMotion.matches && element.animate) element.animate([
    { opacity: 0, transform: 'translateY(6px)' }, { opacity: 1, transform: 'translateY(0)' }
  ], { duration: 160, easing: 'cubic-bezier(.2,.7,.2,1)' });
}
function makeRow(item) {
  const row = document.createElement('li'); row.className = 'item'; row.dataset.id = item.id;
  if (item.kind === 'shopping') {
    const check = document.createElement('input'); check.type = 'checkbox'; check.className = 'item-check';
    check.addEventListener('change', () => mutate(row.item, 'PATCH', { checked: check.checked })); row.append(check);
  } else { const number = document.createElement('span'); number.className = 'meal-number'; row.append(number); }
  const name = document.createElement('button'); name.type = 'button'; name.className = 'item-name';
  name.addEventListener('click', () => openEditor(row.item)); row.append(name);
  if (item.kind === 'shopping') {
    const amount = document.createElement('button'); amount.type = 'button'; amount.className = 'amount-button';
    amount.addEventListener('click', () => openEditor(row.item, true)); row.append(amount);
  }
  const remove = document.createElement('button'); remove.type = 'button'; remove.className = 'remove-button'; remove.append(icon('trash'));
  remove.addEventListener('click', () => confirmRemoval(row.item)); row.append(remove);
  return row;
}
function acceptState(state, confirmed = false) {
  if (!token || (current && state.revision < current.revision)) return;
  // A live event can arrive before the POST response. Keep its snapshot until
  // optimistic additions are reconciled, so the same item never appears twice.
  if (additions.size && !confirmed) {
    if (!deferredState || state.revision >= deferredState.revision) deferredState = state;
    return;
  }
  if (deferredState && deferredState.revision > state.revision) state = deferredState;
  deferredState = null; current = state; storage.set('state', JSON.stringify(state)); draw();
}
function draw() {
  if (!current || !token) return;
  for (const kind of ['shopping', 'meals']) {
    const list = $(kind + '-list');
    const previous = new Map([...list.children].map(row => [row.dataset.id, row.getBoundingClientRect()]));
    const rows = current[kind].filter(item => mutations.get(item.id)?.method !== 'DELETE').map(item => ({ ...item, ...(mutations.get(item.id)?.data || {}) }));
    for (const pending of additions.values()) if (pending.kind === kind) pending.texts.forEach((text, i) => rows.push({ id: 'pending-' + pending.request_id + '-' + i, kind, text, quantity: pending.quantity || 1, unit: pending.unit || 'piece', queued: true }));
    rows.reverse(); // API order is oldest first; the tablet always shows the newest entry at the top.
    const known = new Map([...list.children].map(row => [row.dataset.id, row]));
    rows.forEach((item, index) => {
      let row = known.get(item.id), fresh = false;
      if (!row) { row = makeRow(item); fresh = true; }
      known.delete(item.id); row.item = item;
      row.classList.toggle('checked', !!item.checked); row.classList.toggle('queued', !!item.queued);
      row.classList.toggle('busy', mutations.has(item.id));
      const name = row.querySelector('.item-name'); name.textContent = item.text; name.setAttribute('aria-label', t('Edit ') + item.text);
      const check = row.querySelector('.item-check');
      if (check) { check.checked = !!item.checked; check.setAttribute('aria-label', t('Purchased: ') + item.text); }
      const number = row.querySelector('.meal-number'); if (number) number.textContent = String(index + 1).padStart(2, '0');
      const amount = row.querySelector('.amount-button');
      if (amount) { amount.textContent = formatAmount(item.quantity ?? 1, item.unit || 'piece'); amount.setAttribute('aria-label', t('Edit quantity and unit for ') + item.text); }
      row.querySelector('.remove-button').setAttribute('aria-label', t('Remove ') + item.text);
      row.querySelectorAll('button,input').forEach(control => control.disabled = !!item.queued || mutations.has(item.id));
      if (list.children[index] !== row) list.insertBefore(row, list.children[index] || null);
      if (fresh) animate(row);
    });
    for (const row of known.values()) {
      // Pending rows are replaced by committed rows without a deletion effect.
      if (!row.item.queued) animateRemoval(row, previous.get(row.dataset.id));
      row.remove();
    }
    if (!reducedMotion.matches) for (const row of list.children) {
      const before = previous.get(row.dataset.id);
      if (!before) continue;
      const delta = before.top - row.getBoundingClientRect().top;
      if (Math.abs(delta) > 1) {
        row.getAnimations().forEach(animation => animation.cancel());
        row.animate([{ transform: 'translateY(' + delta + 'px)' }, { transform: 'translateY(0)' }], { duration: 300, easing: 'cubic-bezier(.2,.8,.2,1)' });
      }
    }
    $(kind + '-empty').hidden = rows.length > 0;
    const count = kind === 'shopping' ? rows.filter(item => !item.checked).length : rows.length;
    $(kind + '-count').textContent = count + (kind === 'shopping' ? count === 1 ? t(' item') : t(' items') : count === 1 ? t(' request') : t(' requests'));
  }
}
function motionLayer() {
  let layer = $('motion-layer');
  if (!layer) { layer = document.createElement('div'); layer.id = 'motion-layer'; layer.setAttribute('aria-hidden', 'true'); document.body.append(layer); }
  return layer;
}
function animateRemoval(row, originalRect) {
  if (reducedMotion.matches) return;
  const rect = originalRect || row.getBoundingClientRect(), panel = row.closest('.list-area').getBoundingClientRect();
  if (rect.bottom <= panel.top || rect.top >= panel.bottom) return;
  const ghost = row.cloneNode(true); ghost.classList.add('removal-clone'); ghost.inert = true; ghost.setAttribute('aria-hidden', 'true');
  Object.assign(ghost.style, { left: rect.left + 'px', top: rect.top + 'px', width: rect.width + 'px', height: rect.height + 'px', background: getComputedStyle(row.closest('.board')).backgroundColor });
  motionLayer().append(ghost);
  ghost.animate([{ opacity: 1, transform: 'translateX(0) scale(1)' }, { opacity: 0, transform: 'translateX(70px) scale(.93)' }], { duration: 280, easing: 'cubic-bezier(.4,0,1,1)' }).finished.finally(() => ghost.remove());
}
function flyAddition(kind, text, origin) {
  if (reducedMotion.matches) return;
  requestAnimationFrame(() => {
    const target = $(kind + '-list').firstElementChild?.querySelector('.item-name'); if (!target) return;
    const end = target.getBoundingClientRect(), flyer = document.createElement('div');
    flyer.className = 'addition-flight ' + kind; flyer.textContent = text; flyer.setAttribute('aria-hidden', 'true');
    Object.assign(flyer.style, { left: end.left + 'px', top: end.top + 'px', width: Math.min(end.width, 280) + 'px' });
    motionLayer().append(flyer);
    const x = origin.left - end.left, y = origin.top - end.top;
    flyer.animate([
      { opacity: .85, transform: 'translate(' + x + 'px,' + y + 'px) scale(.92)' },
      { opacity: 1, offset: .72 },
      { opacity: 0, transform: 'translate(0,0) scale(1)' }
    ], { duration: 440, easing: 'cubic-bezier(.2,.75,.2,1)' }).finished.finally(() => flyer.remove());
  });
}
let removingItem;
function confirmRemoval(item) {
  if (item.queued || mutations.has(item.id)) return;
  removingItem = { ...item };
  $('delete-title').textContent = item.kind === 'shopping' ? t('Delete this item?') : t('Delete this meal request?');
  $('delete-name').textContent = item.text;
  $('delete-dialog').showModal(); $('cancel-delete').focus(); animate($('delete-dialog'));
}
$('cancel-delete').addEventListener('click', () => $('delete-dialog').close());
$('confirm-delete').addEventListener('click', () => {
  const item = removingItem; $('delete-dialog').close();
  if (item) mutate(item, 'DELETE', {});
});
async function mutate(item, method, data) {
  if (mutations.has(item.id) || item.queued) return;
  mutations.set(item.id, { method, data }); draw(); status();
  try {
    const state = await api('/api/items/' + item.id, { method, body: JSON.stringify({ ...data, version: item.version }) });
    mutations.delete(item.id); acceptState(state); status(true);
    if (method === 'DELETE') toast('Slettet', true);
  } catch (error) {
    mutations.delete(item.id); draw(); toast(error.message); status(false);
    if (error.status === 409) { try { acceptState(await api('/api/state')); status(true); } catch {} }
  }
}
function openEditor(item, amountOnly = false) {
  if (item.queued || mutations.has(item.id)) return;
  editing = { id: item.id, version: item.version, kind: item.kind };
  $('amount-title').textContent = item.kind === 'shopping' ? t('Edit item') : t('Edit meal request');
  $('edit-name').value = item.text; $('edit-quantity').value = item.quantity ?? 1; $('edit-unit').value = item.unit || 'piece';
  $('edit-amount-fields').hidden = item.kind !== 'shopping';
  $('edit-quantity').disabled = $('edit-unit').disabled = item.kind !== 'shopping';
  $('amount-error').textContent = ''; $('amount-dialog').showModal();
  (amountOnly ? $('edit-quantity') : $('edit-name')).focus(); animate($('amount-dialog'));
}
$('cancel-amount').addEventListener('click', () => $('amount-dialog').close());
$('amount-form').addEventListener('submit', async event => {
  event.preventDefault(); const button = event.currentTarget.querySelector('[type=submit]'); if (button.disabled) return;
  const text = $('edit-name').value.trim(); if (!text) { $('edit-name').focus(); return; }
  button.disabled = true;
  const edit = { ...editing };
  const changes = { text, version: edit.version, ...(edit.kind === 'shopping' ? { quantity: Number($('edit-quantity').value), unit: $('edit-unit').value } : {}) };
  try {
    acceptState(await api('/api/items/' + edit.id, { method: 'PATCH', body: JSON.stringify(changes) })); status(true); $('amount-dialog').close();
  } catch (error) {
    $('amount-error').textContent = error.message;
    if (error.status === 409) {
      try {
        const state = await api('/api/state'); acceptState(state);
        const latest = state[edit.kind].find(item => item.id === edit.id);
        if (latest) editing.version = latest.version;
        $('amount-error').textContent = latest ? t('This item changed on another device. Check your changes and save again.') : t('This item was removed on another device.');
      } catch {}
    }
  } finally { button.disabled = false; }
});
function fitField(field) { field.style.height = '51px'; field.style.height = Math.min(94, Math.max(51, field.scrollHeight)) + 'px'; }
function saveAmount() { storage.set('draft.amount', JSON.stringify({ quantity: $('shopping-quantity').value, unit: $('shopping-unit').value })); }
try { const draft = JSON.parse(storage.get('draft.amount')); if (draft) { $('shopping-quantity').value = draft.quantity; $('shopping-unit').value = normalizeLegacyUnit(draft.unit); } } catch {}
for (const id of ['shopping-quantity', 'shopping-unit']) $(id).addEventListener('input', saveAmount);
for (const button of document.querySelectorAll('[data-step]')) {
  button.addEventListener('click', () => {
    $('shopping-quantity').value = Math.min(9999, Math.max(1, Math.round(((Number($('shopping-quantity').value) || 1) + Number(button.dataset.step)) * 100) / 100)); saveAmount();
  });
}
function persistOutbox() { return storage.set('outbox', JSON.stringify([...additions.values()])); }
async function flushAdditions() {
  clearTimeout(postRetryTimer);
  if (posting || !token || !additions.size || !navigator.onLine) return;
  posting = true;
  try {
    while (token && additions.size && navigator.onLine) {
      const [id, item] = additions.entries().next().value;
      try {
        const state = await api('/api/items', { method: 'POST', body: JSON.stringify(item) });
        additions.delete(id); persistOutbox(); acceptState(state, true); status(true); saveErrorShown = false;
      } catch (error) {
        status(false);
        if (!saveErrorShown) { toast(error.status === 401 ? t('Log in again to save your pending items.') : t('Your items are saved on this device and will sync when you\'re back online.')); saveErrorShown = true; }
        if (error.status && error.status !== 401 && error.status < 500 && error.status !== 429) {
          // A rejected payload must remain editable, rather than blocking later saves.
          const field = $(item.kind + '-input');
          field.value = item.texts.join('\n') + (field.value.trim() ? '\n' + field.value : '');
          storage.set('draft.' + item.kind, field.value); fitField(field);
          if (item.kind === 'shopping') { $('shopping-quantity').value = item.quantity; $('shopping-unit').value = item.unit; saveAmount(); }
          additions.delete(id); persistOutbox(); draw(); toast(error.message);
        }
        break;
      }
    }
  } finally {
    posting = false; draw();
    if (token && additions.size) postRetryTimer = setTimeout(flushAdditions, 5000);
    else if (deferredState) acceptState(deferredState);
  }
}
for (const form of document.querySelectorAll('.composer')) {
  const kind = form.dataset.kind, input = form.querySelector('textarea'), key = 'draft.' + kind;
  input.value = storage.get(key) || ''; requestAnimationFrame(() => fitField(input));
  input.addEventListener('input', () => { storage.set(key, input.value); fitField(input); });
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); form.requestSubmit(); }
  });
  form.addEventListener('submit', event => {
    event.preventDefault();
    const texts = input.value.split(/\r?\n/).map(text => text.trim()).filter(Boolean);
    if (!texts.length) { input.focus(); return; }
    if (texts.length > 50 || texts.some(text => text.length > 300)) { toast(t('Use up to 50 lines at a time and 300 characters per line.')); return; }
    const amount = kind === 'shopping' ? { quantity: Number($('shopping-quantity').value), unit: $('shopping-unit').value } : {};
    const payload = JSON.stringify({ texts, ...amount });
    let pending; try { pending = normalizeLegacyPending(JSON.parse(storage.get('pending.' + kind))); } catch {}
    if (pending?.payload === JSON.stringify(texts) && (kind !== 'shopping' || (amount.quantity === 1 && amount.unit === 'piece'))) pending.payload = payload;
    if (!pending || pending.payload !== payload) pending = { payload, id: crypto.randomUUID ? crypto.randomUUID() : Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('') };
    additions.set(pending.id, { kind, texts, ...amount, request_id: pending.id });
    // Persist before clearing the composer. Retrying the same request ID is safe,
    // including after closing the app or losing a successful server response.
    if (!persistOutbox()) { additions.delete(pending.id); toast(t('This device couldn\'t save locally. Your text is still in the field.')); return; }
    const origin = input.getBoundingClientRect();
    input.value = ''; storage.remove(key); storage.remove('pending.' + kind); fitField(input);
    if (kind === 'shopping') { $('shopping-quantity').value = '1'; $('shopping-unit').value = 'piece'; storage.remove('draft.amount'); }
    document.activeElement?.blur(); input.blur(); draw(); status();
    $(kind + '-scroll').scrollTop = 0;
    flyAddition(kind, texts[texts.length - 1], origin);
    const receipt = texts.length === 1 ? t('Added') : texts.length + (kind === 'shopping' ? t(' items added') : t(' meal requests added'));
    toast(receipt + (navigator.onLine ? '' : ' · afventer net'), true);
    flushAdditions();
  });
}

async function connect() {
  clearTimeout(retryTimer); streamController?.abort();
  if (!token || document.hidden) return;
  const controller = new AbortController(); streamController = controller;
  try {
    const response = await fetch('/api/events', { headers: { Authorization: 'Bearer ' + token }, signal: controller.signal, cache: 'no-store' });
    if (response.status === 401) { showLogin(); return; }
    if (!response.ok || !response.body) throw Error('stream');
    const reader = response.body.getReader(), decoder = new TextDecoder(); let buffer = ''; status(true);
    while (true) {
      const { value, done } = await reader.read(); if (done) throw Error('closed'); buffer += decoder.decode(value, { stream: true });
      let pos;
      while ((pos = buffer.indexOf('\n\n')) >= 0) {
        const event = buffer.slice(0, pos); buffer = buffer.slice(pos + 2);
        if (event.startsWith('data: ')) { acceptState(JSON.parse(event.slice(6))); status(true); }
      }
    }
  } catch {
    if (controller.signal.aborted) return;
    status(false); retryTimer = setTimeout(connect, 2500);
  }
}
async function enter() {
  if (!token) return;
  showApp();
  if (!current) { current = { revision: -1, shopping: [], meals: [] }; draw(); }
  try { acceptState(await api('/api/state')); status(true); }
  catch (error) { if (error.status === 401) return; status(false); }
  connect(); flushAdditions();
}
$('login-form').addEventListener('submit', async event => {
  event.preventDefault(); const button = event.currentTarget.querySelector('[type=submit]'); button.disabled = true; $('login-error').textContent = '';
  try {
    const data = await api('/api/login', { method: 'POST', body: JSON.stringify({ password: $('password').value }) });
    token = data.token;
    if (!storage.set('token', token)) toast(t('This device doesn\'t allow storage. Your login can\'t be remembered.'));
    $('password').value = ''; $('password').blur(); navigator.storage?.persist?.().catch(() => {}); await enter();
  } catch (error) { $('login-error').textContent = error.message; }
  finally { button.disabled = false; }
});
// Native speech callbacks are the source of truth. Queued TTS never animates a mouth.
const foodieDialog = $('foodie-dialog');
const foodiePhases = new Set(['waking', 'speaking', 'listening', 'hearing', 'thinking']);
const foodieTitles = { waking: t('Hello there'), speaking: 'Foodie', listening: t('I\'m listening'), hearing: t('I\'m listening'), thinking: t('One moment …') };
let foodieSession = 0, foodieDismissed = 0, foodieCloseTimer, foodieNativeClosures = 0;
let foodieRenderer, foodieRendererPromise, foodieRendererFailed = false;
let foodieFrame = { visible: false, phase: 'idle', level: 0 };
function prepareFoodieAvatar() {
  if (!token || nativeVersion < 6 || foodieRenderer || foodieRendererPromise || foodieRendererFailed) return;
  const identity = token;
  foodieRendererPromise = import('/foodie-3d.js?v=' + encodeURIComponent(loadedVersion || '130')).then(module => {
    if (!token || token !== identity) return;
    foodieRenderer = module.createFoodie(document.querySelector('.foodie-orb'), reducedMotion);
    foodieRenderer.update(foodieFrame);
  }).catch(() => {
    foodieRendererFailed = true;
    document.querySelectorAll('.foodie-canvas').forEach(canvas => canvas.remove());
    // Keep the animated vector if WebGL is unavailable on this device.
  }).finally(() => { foodieRendererPromise = null; });
}
function pauseFoodieAvatar() {
  foodieFrame = { visible: false, phase: 'idle', level: 0 }; foodieRenderer?.update(foodieFrame);
}
function hideFoodie(immediate = false) {
  pauseFoodieAvatar();
  if (!foodieDialog.open) return;
  if (foodieCloseTimer && !immediate) return;
  clearTimeout(foodieCloseTimer);
  const close = () => {
    foodieCloseTimer = undefined;
    if (!foodieDialog.open) return;
    foodieNativeClosures++; foodieDialog.close(); foodieDialog.classList.remove('is-leaving');
  };
  if (immediate || reducedMotion.matches) close();
  else { foodieDialog.classList.add('is-leaving'); foodieCloseTimer = setTimeout(close, 200); }
}
function dismissFoodie() {
  pauseFoodieAvatar();
  foodieDismissed = Math.max(foodieDismissed, foodieSession);
  clearTimeout(foodieCloseTimer); foodieCloseTimer = undefined;
  if (foodieDialog.open) foodieDialog.close();
}
foodieDialog.addEventListener('close', () => {
  if (foodieNativeClosures) { foodieNativeClosures--; return; }
  pauseFoodieAvatar();
  foodieDismissed = Math.max(foodieDismissed, foodieSession);
  if (token && nativeVersion >= 6) location.href = 'mad-app://voice/dismiss';
});
foodieDialog.addEventListener('cancel', event => { event.preventDefault(); dismissFoodie(); });
$('foodie-close').addEventListener('click', dismissFoodie);
document.addEventListener('visibilitychange', () => foodieDialog.classList.toggle('is-paused', document.hidden));
function presentFoodie(detail) {
  if (nativeVersion < 6 || !token || $('app-view').hidden) { hideFoodie(true); return; }
  const session = Number(detail.session);
  if (!Number.isSafeInteger(session) || session < foodieSession) return;
  if (!detail.running || !detail.visible) {
    foodieDismissed = Math.max(foodieDismissed, session); hideFoodie(); return;
  }
  if (session < 1 || session <= foodieDismissed || !foodiePhases.has(detail.phase)) return;
  foodieSession = session;
  clearTimeout(foodieCloseTimer); foodieCloseTimer = undefined; foodieDialog.classList.remove('is-leaving');
  if (foodieDialog.dataset.phase !== detail.phase) foodieDialog.dataset.phase = detail.phase;
  const title = foodieTitles[detail.phase];
  if ($('foodie-title').textContent !== title) $('foodie-title').textContent = title;
  const caption = typeof detail.text === 'string' ? detail.text : '';
  if ($('foodie-caption').textContent !== caption) $('foodie-caption').textContent = caption;
  const level = ['listening', 'hearing'].includes(detail.phase) ? Math.max(0, Math.min(1, Number(detail.level) || 0)) : 0;
  foodieDialog.style.setProperty('--voice-level', level.toFixed(2));
  if (!foodieDialog.open) {
    // Blur before showModal so closing it cannot restore focus and reopen the keyboard.
    document.activeElement?.blur(); foodieDialog.showModal();
  }
  foodieFrame = { visible: true, phase: detail.phase, level };
  prepareFoodieAvatar(); foodieRenderer?.update(foodieFrame);
}

$('sync-control').addEventListener('click', enter);
$('voice').addEventListener('click', () => { if (token && nativeVersion >= 3) location.href = 'mad-app://voice'; });
window.addEventListener('mad-voice-status', event => {
  if (nativeVersion < 3 || !event.detail) return;
  $('voice').setAttribute('aria-pressed', event.detail.running ? 'true' : 'false');
  $('voice').title = 'Hey Foodie · ' + (event.detail.status || t('Off'));
  presentFoodie(event.detail);
});
$('native-settings').addEventListener('click', () => { if (token && nativeVersion >= 2) location.href = 'mad-app://settings'; });
window.addEventListener('mad-back', () => {
  if (foodieDialog.open) { dismissFoodie(); return; }
  const dialog = document.querySelector('dialog[open]');
  if (dialog) dialog.close(); else document.activeElement?.blur();
});
for (const dialog of document.querySelectorAll('dialog')) {
  dialog.addEventListener('click', event => {
    if (event.target !== dialog) return;
    const rect = dialog.getBoundingClientRect();
    if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) dialog.close();
  });
}
async function requestWake() {
  if (!wakeEnabled || document.hidden) return;
  try {
    wakeLock = await navigator.wakeLock.request('screen'); $('wake').setAttribute('aria-pressed', 'true');
    wakeLock.addEventListener('release', () => $('wake').setAttribute('aria-pressed', 'false'));
  } catch { toast(t('This device can\'t keep the screen on right now.')); }
}
$('wake').addEventListener('click', async () => { wakeEnabled = !wakeEnabled; if (wakeEnabled) await requestWake(); else await wakeLock?.release(); });
window.addEventListener('beforeinstallprompt', event => event.preventDefault());
function prepareAndroidDownload() {
  if (!token || $('install').hidden) return Promise.resolve(false);
  if (downloadExpires > Date.now() + 60000) return Promise.resolve(true);
  if (downloadPreparation) return downloadPreparation;
  const link = $('install'), deviceToken = token;
  const controller = new AbortController(), timeout = setTimeout(() => controller.abort(), 15000);
  link.setAttribute('aria-busy', 'true');
  downloadPreparation = (async () => {
    try {
      const response = await fetch('/api/download/android/session', {
        method: 'POST', headers: { Authorization: 'Bearer ' + deviceToken },
        credentials: 'omit', cache: 'no-store', signal: controller.signal
      });
      if (!response.ok) { if (response.status === 401) showLogin(); throw Error(t('Couldn\'t get the download link. Tap to try again.')); }
      const data = await response.json();
      if (token !== deviceToken) return false;
      const url = new URL(data.url, location.origin);
      if (url.origin !== location.origin || url.pathname !== '/api/download/android' || !url.searchParams.has('grant')) throw Error(t('Couldn\'t get the download link. Tap to try again.'));
      url.searchParams.set('t', Date.now());
      downloadExpires = Date.now() + data.expires_in * 1000 - 5000;
      link.download = data.filename; link.href = url.href;
      link.title = t(androidUpdateAvailable ? 'Update Foodie' : 'Download Android app'); downloadFailure = '';
      return true;
    } catch (error) {
      downloadFailure = error.name === 'AbortError' || error instanceof TypeError ? t('No connection. Try again when you\'re online.') : error.message;
      link.title = downloadFailure;
      return false;
    } finally {
      clearTimeout(timeout); downloadPreparation = null; link.removeAttribute('aria-busy');
    }
  })();
  return downloadPreparation;
}
$('install').addEventListener('click', event => {
  if (!token) { event.preventDefault(); return; }
  if (nativeVersion && !installedAndroidCode) {
    event.preventDefault(); $('android-update-dialog').showModal(); return;
  }
  if (Date.now() >= downloadExpires) {
    event.preventDefault();
    if (downloadRequested) return;
    downloadRequested = true; toast(t('Preparing download …'));
    prepareAndroidDownload().then(ready => {
      downloadRequested = false;
      if (ready && token) $('install').click(); else if (token) toast(downloadFailure);
    });
    return;
  }
  // The header is the real download link. Its APK-only grant also works when
  // Android's download manager has no access to the browser's cookies.
  const url = new URL($('install').href); url.searchParams.set('t', Date.now());
  $('install').href = url.href;
  toast(t(installedAndroidCode ? 'Downloading update …' : 'Download started. Open the APK from your browser\'s downloads.'));
});
window.addEventListener('appinstalled', () => $('install').hidden = true);
async function checkAndroidUpdate() {
  if (!nativeVersion || !token || document.hidden || androidUpdateChecking || !navigator.onLine) return;
  androidUpdateChecking = true;
  const deviceToken = token;
  const controller = new AbortController(), timeout = setTimeout(() => controller.abort(), 10000);
  try {
    const response = await fetch('/api/android/release', {headers: {Authorization: 'Bearer ' + deviceToken}, cache: 'no-store', signal: controller.signal});
    if (token !== deviceToken) return;
    if (response.status === 401) { showLogin(); return; }
    if (response.status === 404) { androidUpdateAvailable = false; $('install').hidden = true; return; }
    if (!response.ok) return;
    const release = await response.json();
    if (token !== deviceToken || !Number.isSafeInteger(release.version_code) || release.version_code < 1) return;
    androidUpdateAvailable = release.version_code > installedAndroidCode;
    const link = $('install'); link.hidden = !androidUpdateAvailable;
    if (androidUpdateAvailable) {
      link.querySelector('span').textContent = t('Update Foodie');
      link.setAttribute('aria-label', t('Update Foodie'));
      link.title = t('Update Foodie') + ' ' + release.version_name;
      if (installedAndroidCode) prepareAndroidDownload();
    }
  } catch {} finally { clearTimeout(timeout); androidUpdateChecking = false; }
}
async function checkWebUpdate() {
  if (updateCheckRunning || document.hidden || !navigator.onLine || !loadedVersion || additions.size || mutations.size) return;
  if (document.activeElement?.matches('input,textarea,select') || document.querySelector('dialog[open]')) return;
  updateCheckRunning = true;
  try {
    const response = await fetch('/api/app-version', { cache: 'no-store' });
    if (response.ok && (await response.json()).version !== loadedVersion) location.reload();
  } catch {} finally { updateCheckRunning = false; }
}
let resizeFrame;
function fitViewport() {
  cancelAnimationFrame(resizeFrame);
  resizeFrame = requestAnimationFrame(() => {
    const viewport = window.visualViewport;
    if (!viewport || viewport.scale === 1) document.documentElement.style.setProperty('--app-height', Math.round(viewport?.height || innerHeight) + 'px');
    for (const field of document.querySelectorAll('.composer textarea')) fitField(field);
  });
}
window.addEventListener('resize', fitViewport); window.visualViewport?.addEventListener('resize', fitViewport); fitViewport();
document.addEventListener('visibilitychange', () => {
  if (document.hidden) { streamController?.abort(); clearTimeout(retryTimer); }
  else { fitViewport(); enter(); requestWake(); checkWebUpdate(); checkAndroidUpdate(); }
});
window.addEventListener('online', () => { enter(); checkWebUpdate(); checkAndroidUpdate(); });
window.addEventListener('offline', () => status(false));
window.addEventListener('pageshow', event => { if (event.persisted) enter(); });
setInterval(() => { checkWebUpdate(); checkAndroidUpdate(); if (!document.hidden) prepareAndroidDownload(); }, 60000);
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});
if (token) {
  showApp();
  current = { revision: -1, shopping: [], meals: [] };
  try { const state = JSON.parse(storage.get('state')); if (state && Array.isArray(state.shopping) && Array.isArray(state.meals)) { for (const item of [...state.shopping, ...state.meals]) item.unit = normalizeLegacyUnit(item.unit); current = state; draw(); } } catch {}
  draw(); enter();
}
