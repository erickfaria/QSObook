'use strict';
for (const input of document.querySelectorAll('[data-lookup]')) {
  const form = input.form;
  const status = form.querySelector('[data-lookup-status]');
  const lookupButton = form.querySelector('[data-lookup-button]');
  const fields = ['name', 'city', 'state', 'country'];
  if (form.getAttribute('action') === '/station') fields.push('notes');
  const qsoEntry = ['/records', '/qso/entry'].includes(form.getAttribute('action'));
  if (qsoEntry) fields.push('operator_notes');
  const revision = form.elements.radioid_revision;
  const manualInput = form.elements.radioid_edited;
  let manual = Number(manualInput?.value) || 0;
  const setManual = value => { manual = value; if (manualInput) manualInput.value = String(value); };
  const setRevision = value => { if (revision) revision.value = value || ''; };
  const busy = value => {
    status.setAttribute?.('data-loading', String(value));
    status.setAttribute?.('aria-busy', String(value));
    if (lookupButton) lookupButton.disabled = value;
  };
  const draftFields = [...new Set([...fields, 'notes'])];
  const drafts = new Map();
  let current = input.value.trim().toUpperCase();
  let edited = new Set(draftFields.filter(name => form.elements[name].value !== ''));
  let timer, request, explicitLookup;
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
  const feedback = message => {
    status.textContent = window.qsobookText(message);
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
      const fallback = station ? known(station) : window.qsobookText("Indicativo novo no cadastro local.");
      status.textContent = fallback;
      const incomplete = !station || ['name', 'city', 'state', 'country'].some(name => !station[name]?.trim());
      if (!external) return;
      if (form.dataset.radioidEnabled !== 'true') {
        status.textContent = fallback + ' ' + window.qsobookText("O RadioID está desativado em Preferências.");
        return;
      }
      if (!incomplete) return;
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
      if (data.radioid_status === 'found') {
        fill(data, true);
        const missing = ['name', 'city', 'state', 'country'].some(name => !form.elements[name].value.trim());
        feedback(missing ? "O RadioID retornou dados incompletos. Complete os campos manualmente." : "Dados encontrados no RadioID.");
      } else if (data.radioid_status === 'not_found') {
        feedback("Indicativo não encontrado no RadioID.");
      } else if (data.radioid_status === 'disabled') {
        feedback("O RadioID está desativado em Preferências.");
      } else if (data.radioid_status === 'local') {
        fill(data, true);
        status.textContent = data.id ? known(data) : fallback;
      } else if (data.radioid_status === 'pending') {
        feedback("O RadioID está ocupado ou em pausa. Tente novamente mais tarde.");
      } else {
        feedback("Consulta indisponível. Os dados poderão ser atualizados posteriormente.");
      }
    } catch (error) {
      if (version === sequence && error.name !== 'AbortError') {
        if (external && form.dataset.radioidEnabled === 'true') {
          feedback("Consulta indisponível. Os dados poderão ser atualizados posteriormente.");
        } else {
          status.textContent = window.qsobookText("Não foi possível consultar. Você pode preencher os dados manualmente.");
        }
      }
    } finally {
      if (version === sequence) busy(false);
    }
  };
  for (const name of draftFields) {
    form.elements[name].addEventListener('input', () => {
      edited.add(name);
      const field = ['name', 'city', 'state', 'country'].indexOf(name);
      if (field !== -1) setManual(manual | (1 << field));
    });
  }
  input.addEventListener('input', () => {
    const version = ++sequence;
    clearTimeout(timer);
    if (request) request.abort();
    explicitLookup = null;
    busy(false);
    const callsign = input.value.trim().toUpperCase();
    if (callsign !== current) {
      if (current) drafts.set(current, {
        values: Object.fromEntries(draftFields.map(name => [name, form.elements[name].value])),
        edited: new Set(edited), revision: revision?.value || '', manual
      });
      const draft = drafts.get(callsign);
      if (draft || current) {
        for (const name of draftFields) form.elements[name].value = draft ? draft.values[name] : '';
        edited = draft ? new Set(draft.edited) : new Set();
        setManual(draft?.manual || 0);
      }
      setRevision(draft?.revision);
      current = callsign;
    }
    if (!callsign) { status.textContent = window.qsobookText("Informe um indicativo."); return; }
    status.textContent = window.qsobookText("Consultando cadastro local…");
    timer = setTimeout(() => lookup(callsign, version), 250);
  });
  const consult = () => {
    input.value = input.value.trim().toUpperCase();
    if (!input.value) {
      feedback("Informe um indicativo.");
      input.focus();
      return;
    }
    if (explicitLookup === input.value) return;
    clearTimeout(timer);
    if (request) request.abort();
    const callsign = input.value, version = ++sequence;
    explicitLookup = callsign;
    status.textContent = window.qsobookText("Consultando cadastro local…");
    busy(true);
    lookup(callsign, version, true).finally(() => { if (version === sequence) explicitLookup = null; });
  };
  input.addEventListener('keydown', event => {
    if (!qsoEntry || event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    consult();
  });
  if (lookupButton) {
    lookupButton.hidden = false;
    lookupButton.addEventListener('click', consult);
  }
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
