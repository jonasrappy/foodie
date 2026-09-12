'use strict';
// Migrate saved drafts and offline additions from releases before 1.4.
// These old wire IDs are never used as a speech recognition vocabulary.
function normalizeLegacyUnit(unit) {
  const legacy = { 'stk.': 'piece', 'kilo': 'kilogram', 'pakker': 'pack', 'poser': 'bag', 'dåser': 'can', 'flasker': 'bottle', 'bundter': 'bunch', 'bakker': 'tray', 'kasser': 'crate' };
  return Object.hasOwn(legacy, unit) ? legacy[unit] : unit;
}

function normalizeLegacyPending(pending) {
  try {
    const payload = JSON.parse(pending.payload);
    if (payload && !Array.isArray(payload) && payload.unit) {
      payload.unit = normalizeLegacyUnit(payload.unit);
      pending.payload = JSON.stringify(payload);
    }
  } catch {}
  return pending;
}
