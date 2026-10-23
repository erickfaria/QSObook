'use strict';
(() => {
  const container = document.getElementById('contact-map');
  if (!container) return;
  const status = document.getElementById('map-status');
  if (typeof L === 'undefined') {
    status.textContent = window.qsobookText("Não foi possível abrir o mapa. Seus contatos estão na lista de cidades.");
    return;
  }
  const map = L.map(container, {scrollWheelZoom: false}).setView([0, 0], 2);
  const tiles = L.tileLayer(container.dataset.tiles, {
    maxZoom: 18,
    // Only the application origin is sent, never a URL with operator details.
    referrerPolicy: 'origin',
    attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a>'
  });
  tiles.on('tileerror', () => {
    status.textContent = window.qsobookText("O fundo do mapa está indisponível. Você ainda pode consultar os pontos e a lista de cidades.");
  });
  const bounds = [];
  for (const city of document.querySelectorAll('[data-map-city]')) {
    const latitude = Number(city.dataset.lat);
    const longitude = Number(city.dataset.lon);
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) continue;
    const link = city.querySelector('[data-city-link]');
    const popup = document.createElement('div');
    const title = document.createElement('p');
    title.textContent = link.textContent;
    const contacts = document.createElement('a');
    contacts.href = link.getAttribute('href');
    contacts.textContent = window.qsobookText("{count} contato(s) · Abrir caderno", {count: city.dataset.count});
    popup.append(title, contacts);
    L.circleMarker([latitude, longitude], {
      radius: 7, color: '#3e6250', weight: 2, fillColor: '#c4dbcc', fillOpacity: 0.9
    }).addTo(map).bindPopup(popup);
    bounds.push([latitude, longitude]);
  }
  if (bounds.length) map.fitBounds(bounds, {padding: [28, 28], maxZoom: 7});
  // Add tiles only after selecting the final viewport; no background prefetch.
  tiles.addTo(map);
})();
