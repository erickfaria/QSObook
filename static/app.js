'use strict';
for (const input of document.querySelectorAll('[data-lookup]')) {
  const form = input.form;
  const status = form.querySelector('#lookup-status');
  const fields = ['name', 'city', 'state', 'country', 'notes'];
  let timer, request;
  const pending = new Set();
  const automatic = new Set();
  for (const name of fields) form.elements[name].addEventListener('input', () => pending.add(name));
  input.addEventListener('input', () => {
    clearTimeout(timer);
    if (request) request.abort();
    for (const name of automatic) {
      if (!pending.has(name)) form.elements[name].value = "";
    }
    automatic.clear();
    const callsign = input.value.trim().toUpperCase();
    status.textContent = 'Consultando cadastro local…';
    timer = setTimeout(async () => {
      if (!callsign) { status.textContent = 'Informe um indicativo.'; return; }
      request = new AbortController();
      try {
        const response = await fetch('/lookup?callsign=' + encodeURIComponent(callsign), {signal: request.signal});
        if (input.value.trim().toUpperCase() !== callsign) return;
        if (response.status === 404) { status.textContent = 'Indicativo novo no cadastro local.'; return; }
        if (!response.ok) throw new Error('lookup');
        const station = await response.json();
        if (input.value.trim().toUpperCase() !== callsign) return;
        for (const name of fields) {
          if (!pending.has(name)) {
            form.elements[name].value = station[name];
            automatic.add(name);
          }
        }
        status.textContent = `Estação conhecida · ${station.contacts} QSO(s) anteriores. Revise os dados antes de salvar.`;
      } catch (error) {
        if (error.name !== 'AbortError') status.textContent = 'Não foi possível consultar. Você pode preencher os dados manualmente.';
      }
    }, 250);
  });
}
