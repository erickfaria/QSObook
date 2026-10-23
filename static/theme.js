'use strict';
(() => {
  const system = window.matchMedia('(prefers-color-scheme: dark)');
  let preference = 'system';
  try {
    const saved = localStorage.getItem('qsobook-theme');
    if (['light', 'dark'].includes(saved)) preference = saved;
  } catch (_) { /* The theme still works when browser storage is unavailable. */ }
  const apply = () => {
    document.documentElement.dataset.theme = preference === 'system'
      ? (system.matches ? 'dark' : 'light') : preference;
  };
  apply();
  system.addEventListener('change', apply);
  document.addEventListener('DOMContentLoaded', () => {
    const picker = document.getElementById('theme');
    picker.value = preference;
    picker.parentElement.hidden = false;
    picker.addEventListener('change', () => {
      preference = ['light', 'dark'].includes(picker.value) ? picker.value : 'system';
      apply();
      try { localStorage.setItem('qsobook-theme', preference); } catch (_) { /* Optional storage. */ }
    });
  });
})();
