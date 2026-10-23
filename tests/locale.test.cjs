const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../static/locale.js'), 'utf8');

test('browser messages use the page language and preserve substituted values', () => {
  const key = 'Operador conhecido · {count} QSO(s) anteriores. Para corrigir dados salvos, use Editar.';
  for (const language of ['pt-BR', 'en', 'es', 'fr']) {
    const messages = language === 'pt-BR' ? {} : JSON.parse(fs.readFileSync(path.join(__dirname, `../locales/${language}.json`), 'utf8'));
    const window = {};
    vm.runInNewContext(source, {window, document: {documentElement: {dataset: {messages: JSON.stringify(messages)}}}});
    assert.equal(window.qsobookText(key, {count: 7}), (messages[key] || key).replace('{count}', '7'));
    assert.equal(window.qsobookText('Untranslated fallback'), 'Untranslated fallback');
    assert.equal(window.qsobookText('{value}', {value: '$& <script>'}), '$& <script>');
    assert.equal(window.qsobookText('{value}'), '{value}');
  }
});
