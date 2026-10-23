'use strict';
for (const input of document.querySelectorAll('[data-lookup]')) {
  const form = input.form;
  const status = form.querySelector('[data-lookup-status]');
  const fields = ['name', 'city', 'state', 'country'];
  if (form.getAttribute('action') === '/station') fields.push('notes');
  const draftFields = [...new Set([...fields, 'notes'])];
  const drafts = new Map();
  let current = input.value.trim().toUpperCase();
  let edited = new Set(draftFields.filter(name => form.elements[name].value !== ''));
  let timer, request;
  let sequence = 0;
  for (const name of draftFields) {
    form.elements[name].addEventListener('input', () => edited.add(name));
  }
  input.addEventListener('input', () => {
    const version = ++sequence;
    clearTimeout(timer);
    if (request) request.abort();
    const callsign = input.value.trim().toUpperCase();
    if (callsign !== current) {
      drafts.set(current, {
        values: Object.fromEntries(draftFields.map(name => [name, form.elements[name].value])),
        edited: new Set(edited)
      });
      const draft = drafts.get(callsign);
      if (draft || current) {
        for (const name of draftFields) form.elements[name].value = draft ? draft.values[name] : '';
        edited = draft ? new Set(draft.edited) : new Set();
      }
      current = callsign;
    }
    if (!callsign) { status.textContent = 'Informe um indicativo.'; return; }
    status.textContent = 'Consultando cadastro local…';
    timer = setTimeout(async () => {
      request = new AbortController();
      try {
        const response = await fetch('/lookup?callsign=' + encodeURIComponent(callsign), {signal: request.signal});
        if (version !== sequence) return;
        if (response.status === 404) { status.textContent = 'Indicativo novo no cadastro local.'; return; }
        if (!response.ok) throw new Error('lookup');
        const station = await response.json();
        if (version !== sequence) return;
        for (const name of fields) {
          if (!edited.has(name) && typeof station[name] === 'string' && station[name].trim()) {
            form.elements[name].value = station[name];
          }
        }
        status.textContent = `Operador conhecido · ${station.contacts} QSO(s) anteriores.`;
      } catch (error) {
        if (version === sequence && error.name !== 'AbortError') {
          status.textContent = 'Não foi possível consultar. Você pode preencher os dados manualmente.';
        }
      }
    }, 250);
  });
}

// The links still open the existing edit page when JavaScript is unavailable.
for (const link of document.querySelectorAll('[data-edit-participant]')) {
  link.addEventListener('click', event => {
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    const editor = document.getElementById(link.getAttribute('aria-controls'));
    editor.hidden = !editor.hidden;
    link.setAttribute('aria-expanded', String(!editor.hidden));
    if (!editor.hidden) editor.querySelector('[name="name"]').focus();
  });
}

for (const form of document.querySelectorAll('form[method="post"]')) {
  form.addEventListener('submit', event => {
    if (form.dataset.submitting) { event.preventDefault(); return; }
    form.dataset.submitting = 'true';
    for (const button of form.querySelectorAll('button:not([type]), button[type="submit"]')) {
      if (!button.disabled) {
        button.dataset.submitDisabled = 'true';
        button.disabled = true;
      }
    }
  });
}

window.addEventListener('pageshow', () => {
  for (const form of document.querySelectorAll('form[method="post"]')) delete form.dataset.submitting;
  for (const button of document.querySelectorAll('[data-submit-disabled]')) {
    button.disabled = false;
    delete button.dataset.submitDisabled;
  }
});
