'use strict';
(() => {
  const container = document.getElementById('contact-map');
  if (!container) return;
  const status = document.getElementById('map-status');
  if (typeof L === 'undefined') {
    status.textContent = window.qsobookText("Não foi possível abrir o mapa. Seus contatos estão na lista de cidades.");
    return;
  }
  const map = L.map(container, {scrollWheelZoom: false, zoomControl: false}).setView([0, 0], 2);
  L.control.zoom({zoomInTitle: window.qsobookText("Aproximar"), zoomOutTitle: window.qsobookText("Afastar")}).addTo(map);
  const tiles = L.tileLayer(container.dataset.tiles, {
    maxZoom: 18,
    // Only the application origin is sent, never a URL with operator details.
    referrerPolicy: 'origin',
    attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a>'
  });
  tiles.on('tileerror', () => {
    status.textContent = window.qsobookText("O fundo do mapa está indisponível. Você ainda pode consultar os pontos e a lista de cidades.");
  });
  const modeFilter = document.getElementById('map-mode');
  const bandFilter = document.getElementById('map-band');
  const digital = new Set(['DIGITAL', 'DMR', 'ECHOLINK', 'D-STAR', 'C4FM']);
  const known = new Set(['FM', 'AM', 'SSB', 'CW', ...digital]);
  const colors = {FM: '#487ca8', AM: '#b9893a', SSB: '#8a729d', CW: '#4c8878', digital: '#bd646c', mixed: '#847b91'};
  const category = mode => digital.has(mode.toUpperCase()) ? 'digital' : mode.toUpperCase();
  let markers = [];
  const home = [];
  if (container.dataset.homeLat !== undefined && Number.isFinite(Number(container.dataset.homeLat)) && Number.isFinite(Number(container.dataset.homeLon))) {
    home.push([Number(container.dataset.homeLat), Number(container.dataset.homeLon)]);
    const label = document.createElement('p'); label.textContent = window.qsobookText('Meu local') + ' · ' + container.dataset.homeCity;
    L.marker(home[0], {title: window.qsobookText('Meu local') + ' · ' + container.dataset.homeCity, icon: L.divIcon({className: 'home-pin', html: '<img src="/static/home.svg" alt="">', iconSize: [28,28], iconAnchor: [14,28]})}).addTo(map).bindPopup(label);
  }
  const render = () => {
    for (const marker of markers) marker.remove();
    markers = [];
    const bounds = [...home];
    const selectedMode = modeFilter?.value || '';
    const selectedBand = bandFilter?.value || '';
    for (const city of document.querySelectorAll('[data-map-city]')) {
      const latitude = Number(city.dataset.lat), longitude = Number(city.dataset.lon);
      if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) continue;
      const groups = city.dataset.groups ? JSON.parse(city.dataset.groups) : [{mode: '', band: '', count: Number(city.dataset.count)}];
      const visible = groups.filter(group => (!selectedMode || (selectedMode === 'digital' ? digital.has(group.mode.toUpperCase()) : selectedMode === 'other' ? !known.has(group.mode.toUpperCase()) : group.mode.toUpperCase() === selectedMode.toUpperCase())) && (!selectedBand || group.band === selectedBand));
      if (!visible.length) continue;
      const link = city.querySelector('[data-city-link]');
      const popup = document.createElement('div');
      const title = document.createElement('p'); title.textContent = link.textContent;
      const contacts = document.createElement('a');
      contacts.href = link.getAttribute('href') + (selectedMode ? '&mode=' + encodeURIComponent(selectedMode) : '') + (selectedBand ? '&band=' + encodeURIComponent(selectedBand) : '');
      contacts.textContent = window.qsobookText('{count} contato(s) · Abrir caderno', {count: visible.reduce((sum,group) => sum + group.count, 0)});
      const radio = document.createElement('p'); radio.textContent = visible.map(group => group.mode + (group.band ? ' · ' + group.band : '')).join(' / ');
      popup.append(title, contacts, radio);
      const categories = new Set(visible.map(group => category(group.mode)));
      const color = colors[categories.size === 1 ? [...categories][0] : 'mixed'] || colors.mixed;
      const marker = L.circleMarker([latitude, longitude], {radius: 7, color, weight: 2, fillColor: color, fillOpacity: 0.8}).addTo(map).bindPopup(popup);
      marker.on('popupopen', event => {
        const close = event.popup.getElement().querySelector('.leaflet-popup-close-button');
        if (close) close.setAttribute('aria-label', window.qsobookText('Fechar'));
      });
      markers.push(marker); bounds.push([latitude,longitude]);
    }
    if (bounds.length) map.fitBounds(bounds, {padding: [28,28], maxZoom: 7});
    if (!markers.length && (selectedMode || selectedBand)) status.textContent = window.qsobookText('Nenhum contato para estes filtros.');
    else status.textContent = '';
  };
  for (const filter of [modeFilter,bandFilter]) if (filter?.addEventListener) filter.addEventListener('change', render);
  render();
  // Add tiles only after selecting the final viewport; no background prefetch.
  tiles.addTo(map);
})();
