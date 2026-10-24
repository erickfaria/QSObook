const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const localeSource = fs.readFileSync(path.join(__dirname, '../static/locale.js'), 'utf8');
const source = fs.readFileSync(path.join(__dirname, '../static/app.js'), 'utf8');

class Element {
  constructor(value = '') {
    this.value = value;
    this.dataset = {};
    this.listeners = new Map();
    this.attributes = {};
  }
  addEventListener(name, callback) {
    if (!this.listeners.has(name)) this.listeners.set(name, []);
    this.listeners.get(name).push(callback);
  }
  emit(name, values = {}) {
    const event = {defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, ...values};
    for (const callback of this.listeners.get(name) || []) callback(event);
    return event;
  }
  getAttribute(name) { return this.attributes[name]; }
  setAttribute(name, value) { this.attributes[name] = value; }
  focus() { this.focused = true; }
}

function setup(action = '/participant') {
  const form = new Element();
  form.attributes.action = action;
  form.elements = Object.fromEntries(['callsign', 'name', 'city', 'state', 'country', 'notes', 'operator_notes'].map(name => [name, new Element()]));
  const input = form.elements.callsign;
  input.form = form;
  const status = {textContent: ''};
  const button = new Element();
  button.disabled = false;
  form.querySelector = () => status;
  form.querySelectorAll = () => [button];
  const link = new Element();
  link.attributes['aria-controls'] = 'editor';
  const editor = {hidden: true, querySelector: () => form.elements.name};
  const window = new Element();
  const requests = [];
  const timers = new Map();
  let timerID = 0;
  vm.runInNewContext(localeSource + source, {
    AbortController,
    window,
    document: {
      documentElement: {dataset: {}},
      querySelectorAll(selector) {
        if (selector === '[data-lookup]') return [input];
        if (selector === '[data-edit-participant]') return [link];
        if (selector === 'form[method="post"]') return [form];
        if (selector === '[data-submit-disabled]') return button.dataset.submitDisabled ? [button] : [];
        return [];
      },
      getElementById: () => editor
    },
    setTimeout(callback) { timers.set(++timerID, callback); return timerID; },
    clearTimeout(id) { timers.delete(id); },
    // Ignore abort intentionally: stale responses must also be checked by version.
    fetch(url, options) {
      return new Promise((resolve, reject) => requests.push({url, options, resolve, reject}));
    }
  });
  return {
    form, status, button, link, editor, window, requests,
    input(value) { input.value = value; input.emit('input'); },
    edit(name, value) { form.elements[name].value = value; form.elements[name].emit('input'); },
    async tick() {
      const callbacks = [...timers.values()];
      timers.clear();
      for (const callback of callbacks) callback();
      await flush();
    },
    async respond(index, data = {}, code = 200) {
      requests[index].resolve({status: code, ok: code === 200, json: async () => data});
      await flush();
    }
  };
}

async function flush() { await new Promise(resolve => setImmediate(resolve)); }

test('fills local operator fields without overwriting edits or conversation notes', async () => {
  const h = setup();
  h.input('zz0test');
  await h.tick();
  h.edit('name', 'Nome digitado durante a consulta');
  h.edit('notes', 'Somente nesta conversa');
  await h.respond(0, {name: 'Nome salvo', city: 'Cidade', state: 'UF', country: 'Brasil', notes: 'Permanente', contacts: 2});
  assert.equal(h.form.elements.name.value, 'Nome digitado durante a consulta');
  assert.equal(h.form.elements.city.value, 'Cidade');
  assert.equal(h.form.elements.state.value, 'UF');
  assert.equal(h.form.elements.country.value, 'Brasil');
  assert.equal(h.form.elements.notes.value, 'Somente nesta conversa');
  assert.match(h.status.textContent, /2 QSO/);
  assert.equal(h.requests[0].url, '/lookup?callsign=ZZ0TEST');
});

test('empty lookup values do not erase existing fields', async () => {
  const h = setup();
  h.input('zz0test');
  await h.tick();
  await h.respond(0, {name: 'Nome salvo', city: 'Cidade', state: 'UF', country: 'Brasil', contacts: 0});
  h.input('ZZ0TEST');
  await h.tick();
  await h.respond(1, {name: '', city: ' ', state: '', country: '', contacts: 0});
  assert.equal(h.form.elements.name.value, 'Nome salvo');
  assert.equal(h.form.elements.city.value, 'Cidade');
  assert.equal(h.form.elements.state.value, 'UF');
  assert.equal(h.form.elements.country.value, 'Brasil');
});

test('rejects stale responses even after switching A to B and back to A', async () => {
  const h = setup();
  h.input('ZZ0A');
  await h.tick();
  h.input('ZZ0B');
  await h.tick();
  h.input('ZZ0A');
  await h.tick();
  await h.respond(2, {name: 'A atual', city: 'Cidade atual', contacts: 3});
  await h.respond(0, {name: 'A antiga', city: 'Cidade antiga', contacts: 1});
  await h.respond(1, {name: 'B', contacts: 0});
  assert.equal(h.form.elements.name.value, 'A atual');
  assert.equal(h.form.elements.city.value, 'Cidade atual');
  assert.match(h.status.textContent, /3 QSO/);
  assert.equal(h.requests[0].options.signal.aborted, true);
});

test('rejects a response whose JSON finishes after the operator changes', async () => {
  const h = setup();
  let finishJSON;
  h.input('ZZ0A');
  await h.tick();
  h.requests[0].resolve({status: 200, ok: true, json: () => new Promise(resolve => { finishJSON = resolve; })});
  await flush();
  h.input('ZZ0B');
  await h.tick();
  await h.respond(1, {name: 'B atual', contacts: 0});
  finishJSON({name: 'A antiga', contacts: 1});
  await flush();
  assert.equal(h.form.elements.name.value, 'B atual');
});

test('keeps separate drafts when switching operators', async () => {
  const h = setup();
  h.input('ZZ0A');
  await h.tick();
  await h.respond(0, {name: 'Nome A', city: 'Cidade A', contacts: 0});
  h.edit('name', 'Correção A');
  h.edit('notes', 'Conversa A');
  h.input('ZZ0B');
  assert.equal(h.form.elements.name.value, '');
  assert.equal(h.form.elements.notes.value, '');
  await h.tick();
  await h.respond(1, {name: 'Nome B', city: 'Cidade B', contacts: 0});
  h.input('ZZ0A');
  await h.tick();
  await h.respond(2, {name: 'Nome A', city: 'Cidade A atualizada', contacts: 0});
  assert.equal(h.form.elements.name.value, 'Correção A');
  assert.equal(h.form.elements.city.value, 'Cidade A atualizada');
  assert.equal(h.form.elements.notes.value, 'Conversa A');
});

test('clearing a callsign does not restore unassigned fields from an earlier operator', async () => {
  const h = setup();
  h.edit('name', 'Nome informado antes do primeiro indicativo');
  h.edit('notes', 'Conversa do primeiro operador');
  h.input('ZZ0A');
  await h.tick();
  await h.respond(0, {name: 'Nome A', contacts: 0});
  h.input('');
  assert.equal(h.form.elements.name.value, '');
  assert.equal(h.form.elements.notes.value, '');
  h.input('ZZ0B');
  await h.tick();
  await h.respond(1, {name: 'Nome B', contacts: 0});
  assert.equal(h.form.elements.name.value, 'Nome B');
  assert.equal(h.form.elements.notes.value, '');
  h.input('ZZ0A');
  assert.equal(h.form.elements.name.value, 'Nome informado antes do primeiro indicativo');
  assert.equal(h.form.elements.notes.value, 'Conversa do primeiro operador');
});

test('stale errors do not replace current status and new callsigns remain editable', async () => {
  const h = setup();
  h.input('ZZ0OLD');
  await h.tick();
  h.input('ZZ0NEW');
  await h.tick();
  await h.respond(1, {}, 404);
  h.requests[0].reject(new Error('old lookup failure'));
  await flush();
  assert.match(h.status.textContent, /novo/);
  h.edit('name', 'Novo operador');
  assert.equal(h.form.elements.name.value, 'Novo operador');
  h.input('ZZ0ERROR');
  await h.tick();
  await h.respond(2, {}, 500);
  assert.match(h.status.textContent, /manualmente/);
});

test('only the operator registration form fills permanent notes', async () => {
  for (const action of ['/station', '/participant', '/contact']) {
    const h = setup(action);
    h.input('ZZ0TEST');
    await h.tick();
    await h.respond(0, {name: 'Operador', notes: 'Permanente', contacts: 0});
    assert.equal(h.form.elements.notes.value, action === '/station' ? 'Permanente' : '');
  }
});

test('prevents repeated submissions and restores controls on back navigation', () => {
  const h = setup();
  assert.equal(h.form.emit('submit').defaultPrevented, false);
  assert.equal(h.button.disabled, true);
  assert.equal(h.form.emit('submit').defaultPrevented, true);
  h.window.emit('pageshow');
  assert.equal(h.button.disabled, false);
  assert.equal(h.form.emit('submit').defaultPrevented, false);
});

test('opens the editor inline while preserving modified-click navigation', () => {
  const h = setup();
  assert.equal(h.link.emit('click', {ctrlKey: true}).defaultPrevented, false);
  assert.equal(h.editor.hidden, true);
  assert.equal(h.link.emit('click').defaultPrevented, true);
  assert.equal(h.editor.hidden, false);
  assert.equal(h.link.attributes['aria-expanded'], 'true');
  assert.equal(h.form.elements.name.focused, true);
  h.link.emit('click');
  assert.equal(h.editor.hidden, true);
});

test('attachment downloads remain usable without a page navigation', () => {
  const h = setup('/data/backup');
  h.form.setAttribute('data-download', 'true');
  for (let i = 0; i < 2; i++) {
    const event = h.form.emit('submit');
    assert.equal(event.defaultPrevented, false);
    assert.equal(h.button.disabled, false);
    assert.equal(h.form.dataset.submitting, undefined);
  }
});

test('entry forms keep permanent notes separate from listening notes', async () => {
  for (const action of ['/records', '/qso/entry']) {
    const h = setup(action);
    h.input('ZZ0QSO');
    h.edit('notes', 'Listening draft');
    await h.tick();
    await h.respond(0, {name: 'Example', city: 'Example city', notes: 'Permanent', contacts: 1});
    assert.equal(h.form.elements.operator_notes.value, 'Permanent');
    assert.equal(h.form.elements.notes.value, 'Listening draft');
  }
});
