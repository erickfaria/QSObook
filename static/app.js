'use strict';
for (const input of document.querySelectorAll('[data-lookup]')) {
  const form = input.form;
  const status = form.querySelector('[data-lookup-status]');
  const fields = ['name', 'city', 'state', 'country'];
  if (form.getAttribute('action') === '/station') fields.push('notes');
  const qsoEntry = ['/records', '/qso/entry'].includes(form.getAttribute('action'));
  if (qsoEntry) fields.push('operator_notes');
  const revision = form.elements.radioid_revision;
  const setRevision = value => { if (revision) revision.value = value || ''; };
  const busy = value => {
    status.setAttribute?.('data-loading', String(value));
    status.setAttribute?.('aria-busy', String(value));
  };
  const draftFields = [...new Set([...fields, 'notes'])];
  const drafts = new Map();
  let current = input.value.trim().toUpperCase();
  let edited = new Set(draftFields.filter(name => form.elements[name].value !== ''));
  let timer, request, messageTimer, enterLookup;
  let sequence = 0;
  const fill = (station, onlyEmpty = false) => {
    for (const name of fields) {
      const value = station[name === 'operator_notes' ? 'notes' : name];
      const field = form.elements[name];
      if (edited.has(name) || (onlyEmpty && field.value.trim()) || typeof value !== 'string' || !value.trim()) continue;
      if (name === 'country' && field.tagName === 'SELECT' && !Array.from(field.options).some(option => option.value === value)) {
        const option = document.createElement('option'); option.value = value; option.textContent = value; field.append(option);
      }
      field.value = value;
    }
  };
  const known = station => window.qsobookText("Operador conhecido · {count} QSO(s) anteriores. Para corrigir dados salvos, use Editar.", {count: station.contacts});
  const feedback = (message, version, fallback) => {
    status.textContent = window.qsobookText(message);
    messageTimer = setTimeout(() => { if (version === sequence) status.textContent = fallback; }, 5000);
  };
  const lookup = async (callsign, version, external = false) => {
    request = new AbortController();
    const signal = request.signal;
    try {
      const response = await fetch('/lookup?callsign=' + encodeURIComponent(callsign), {signal});
      if (version !== sequence) return;
      if (!response.ok && response.status !== 404) throw new Error('lookup');
      const station = response.status === 404 ? null : await response.json();
      if (version !== sequence) return;
      if (station) fill(station);
      setRevision(station?.radioid_revision);
      let fallback = station ? known(station) : window.qsobookText("Indicativo novo no cadastro local.");
      status.textContent = fallback;
      const incomplete = !station || ['name', 'city', 'state', 'country'].some(name => !station[name]?.trim());
      if (!external || form.dataset.radioidEnabled !== 'true' || !incomplete) return;
      status.textContent = window.qsobookText("Consultando RadioID…");
      busy(true);
      const remote = await fetch('/lookup/radioid', {
        method: 'POST', signal, headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: new URLSearchParams({csrf: form.elements.csrf.value, callsign}).toString()
      });
      if (version !== sequence) return;
      if (!remote.ok) throw new Error('radioid');
      const data = await remote.json();
      if (version !== sequence) return;
      setRevision(data.radioid_revision);
      if (data.id) fallback = known(data);
      if (data.radioid_status === 'found') {
        fill(data, true);
        feedback("Dados encontrados no RadioID.", version, fallback);
      } else if (data.radioid_status === 'not_found') {
        feedback("Indicativo não encontrado no RadioID.", version, fallback);
      } else if (data.radioid_status === 'disabled' || data.radioid_status === 'local') {
        status.textContent = fallback;
      } else {
        feedback("Consulta indisponível. Os dados poderão ser atualizados posteriormente.", version, fallback);
      }
    } catch (error) {
      if (version === sequence && error.name !== 'AbortError') {
        if (external && form.dataset.radioidEnabled === 'true') {
          feedback("Consulta indisponível. Os dados poderão ser atualizados posteriormente.", version, '');
        } else {
          status.textContent = window.qsobookText("Não foi possível consultar. Você pode preencher os dados manualmente.");
        }
      }
    } finally {
      if (version === sequence) busy(false);
    }
  };
  for (const name of draftFields) {
    form.elements[name].addEventListener('input', () => edited.add(name));
  }
  input.addEventListener('input', () => {
    const version = ++sequence;
    clearTimeout(timer);
    clearTimeout(messageTimer);
    if (request) request.abort();
    enterLookup = null;
    busy(false);
    const callsign = input.value.trim().toUpperCase();
    if (callsign !== current) {
      if (current) drafts.set(current, {
        values: Object.fromEntries(draftFields.map(name => [name, form.elements[name].value])),
        edited: new Set(edited), revision: revision?.value || ''
      });
      const draft = drafts.get(callsign);
      if (draft || current) {
        for (const name of draftFields) form.elements[name].value = draft ? draft.values[name] : '';
        edited = draft ? new Set(draft.edited) : new Set();
      }
      setRevision(draft?.revision);
      current = callsign;
    }
    if (!callsign) { status.textContent = window.qsobookText("Informe um indicativo."); return; }
    status.textContent = window.qsobookText("Consultando cadastro local…");
    timer = setTimeout(() => lookup(callsign, version), 250);
  });
  input.addEventListener('keydown', event => {
    if (!qsoEntry || event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    input.value = input.value.trim().toUpperCase();
    if (!input.value || enterLookup === input.value) return;
    clearTimeout(timer);
    clearTimeout(messageTimer);
    if (request) request.abort();
    const callsign = input.value, version = ++sequence;
    enterLookup = callsign;
    status.textContent = window.qsobookText("Consultando cadastro local…");
    lookup(callsign, version, true).finally(() => { if (version === sequence) enterLookup = null; });
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
    // Attachment responses keep this page open; these read-only actions can repeat.
    if (form.getAttribute('data-download') === 'true') return;
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

// Progressive enhancement: all fields remain accessible without JavaScript.
for (const select of document.querySelectorAll('[data-other]')) {
  const field = select.form.querySelector('[data-other-field="' + select.dataset.other + '"]');
  const update = () => { field.hidden = select.value !== '__other'; };
  select.addEventListener('change', update); update();
}
for (const toggle of document.querySelectorAll('[data-toggle]')) {
  const field = document.getElementById(toggle.dataset.toggle);
  const update = () => { field.hidden = !toggle.checked; };
  toggle.addEventListener('change', update); update();
}
