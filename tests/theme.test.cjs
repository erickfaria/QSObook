const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../static/theme.js'), 'utf8');

function theme(saved, unavailable = false) {
  let onReady, onChange, onSystemChange;
  const root = {dataset: {}};
  const picker = {parentElement: {hidden: true}, addEventListener(_, callback) { onChange = callback; }};
  const system = {matches: true, addEventListener(_, callback) { onSystemChange = callback; }};
  const storage = {
    getItem() { if (unavailable) throw new Error('blocked'); return saved; },
    setItem(_, value) { if (unavailable) throw new Error('blocked'); saved = value; }
  };
  vm.runInNewContext(source, {
    window: {matchMedia: () => system}, localStorage: storage,
    document: {documentElement: root, addEventListener(_, callback) { onReady = callback; }, getElementById: () => picker}
  });
  onReady();
  return {root, picker, system, saved: () => saved, change(value) { picker.value = value; onChange(); }, systemChange(value) { system.matches = value; onSystemChange(); }};
}

test('an explicit theme survives system changes and is saved for the next visit', () => {
  const h = theme('light');
  assert.equal(h.root.dataset.theme, 'light');
  h.systemChange(true);
  assert.equal(h.root.dataset.theme, 'light');
  h.change('dark');
  assert.equal(h.root.dataset.theme, 'dark');
  assert.equal(theme(h.saved()).root.dataset.theme, 'dark');
  assert.equal(h.picker.parentElement.hidden, false);
});

test('automatic mode follows the system and ignores invalid saved values', () => {
  const h = theme('invalid');
  assert.equal(h.root.dataset.theme, 'dark');
  h.systemChange(false);
  assert.equal(h.root.dataset.theme, 'light');
  h.change('dark');
  h.change('system');
  assert.equal(h.root.dataset.theme, 'light');
});

test('blocked browser storage does not prevent switching themes', () => {
  const h = theme(null, true);
  h.change('light');
  assert.equal(h.root.dataset.theme, 'light');
  h.change('dark');
  assert.equal(h.root.dataset.theme, 'dark');
});
