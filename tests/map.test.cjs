const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const locale = fs.readFileSync(path.join(__dirname, '../static/locale.js'), 'utf8');
const source = fs.readFileSync(path.join(__dirname, '../static/map.js'), 'utf8');

function setup(language, available = true) {
  const messages = language === 'pt-BR' ? {} : JSON.parse(fs.readFileSync(path.join(__dirname, `../locales/${language}.json`), 'utf8'));
  const status = {textContent: ''};
  const container = {dataset: {tiles: 'https://tiles.example.org/{z}/{x}/{y}.png'}};
  const cityLink = {textContent: '<Example city>', getAttribute: () => '/history?city=Example'};
  const city = {dataset: {lat: '1', lon: '2', count: '3'}, querySelector: () => cityLink};
  const close = {setAttribute(name, value) { this[name] = value; }};
  let zoomOptions, popup, popupOpen, tileError, tileOptions;
  const map = {setView() { return this; }, fitBounds() {}};
  const context = {
    window: {},
    document: {
      documentElement: {dataset: {messages: JSON.stringify(messages)}},
      getElementById: id => id === 'contact-map' ? container : status,
      querySelectorAll: () => [city],
      createElement: () => ({append(...children) { this.children = children; }})
    }
  };
  if (available) context.L = {
    map: () => map,
    control: {zoom(options) { zoomOptions = options; return {addTo() {}}; }},
    tileLayer(url, options) {
      tileOptions = options;
      return {on(event, callback) { tileError = callback; }, addTo() {}};
    },
    circleMarker() {
      return {addTo() { return this; }, bindPopup(element) { popup = element; return this; }, on(event, callback) { popupOpen = callback; }};
    }
  };
  vm.runInNewContext(locale + source, context);
  return {status, close, popup, zoomOptions, tileOptions, messages,
    openPopup() { popupOpen({popup: {getElement: () => ({querySelector: () => close})}}); },
    tileError() { tileError(); }};
}

test('map controls, popup counts and offline feedback follow the page language', () => {
  for (const language of ['pt-BR', 'en', 'es', 'fr']) {
    const h = setup(language);
    const text = key => h.messages[key] || key;
    assert.equal(h.zoomOptions.zoomInTitle, text('Aproximar'));
    assert.equal(h.zoomOptions.zoomOutTitle, text('Afastar'));
    assert.equal(h.popup.children[0].textContent, '<Example city>');
    assert.equal(h.popup.children[1].textContent, text('{count} contato(s) · Abrir caderno').replace('{count}', '3'));
    assert.equal(h.popup.children[1].href, '/history?city=Example');
    assert.equal(h.tileOptions.referrerPolicy, 'origin');
    h.openPopup();
    assert.equal(h.close['aria-label'], text('Fechar'));
    h.tileError();
    assert.equal(h.status.textContent, text('O fundo do mapa está indisponível. Você ainda pode consultar os pontos e a lista de cidades.'));
    const failed = setup(language, false);
    assert.equal(failed.status.textContent, text('Não foi possível abrir o mapa. Seus contatos estão na lista de cidades.'));
  }
});
