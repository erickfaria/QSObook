'use strict';
(() => {
  // JSON is escaped by html/template in a data attribute, never executed as code.
  const messages = JSON.parse(document.documentElement.dataset.messages || '{}');
  window.qsobookText = (key, values = {}) => (messages[key] || key).replace(/\{(\w+)\}/g,
    (match, name) => Object.hasOwn(values, name) ? String(values[name]) : match);
})();
