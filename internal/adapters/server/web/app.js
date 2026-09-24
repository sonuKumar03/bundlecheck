let bundleData = null;
let treemap = null;

async function init() {
  treemap = new TreemapEngine('treemap-canvas', 'tooltip');

  // When clicking a tile in treemap, filter the table to that package
  treemap.onSelect = (pkg) => {
    const searchInput = document.getElementById('search-input');
    if (searchInput) {
      searchInput.value = pkg.name;
      treemap.setHighlight(pkg.name);
      filterTable(pkg.name);
    }
  };

  try {
    const res = await fetch('/api/bundle');
    if (!res.ok) throw new Error(await res.text());
    bundleData = await res.json();
    renderBundle(bundleData);
  } catch (err) {
    console.error('Failed to load bundle data:', err);
    const badge = document.getElementById('stats-badge');
    if (badge) badge.innerText = 'No stats loaded';
  }

  // Search input
  const searchInput = document.getElementById('search-input');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      const q = e.target.value;
      treemap.setHighlight(q);
      filterTable(q);
    });
  }
}

function renderBundle(data) {
  const badge = document.getElementById('stats-badge');
  if (badge) badge.innerText = data.statsPath || 'stats.json';

  // Populate metrics
  const mainEp = data.entrypoints && (data.entrypoints['main'] || Object.values(data.entrypoints)[0]);
  if (mainEp) {
    const initEl = document.getElementById('metric-initial');
    if (initEl) initEl.innerText = formatBytes(mainEp.initialBytes);

    const asyncEl = document.getElementById('metric-async');
    if (asyncEl) asyncEl.innerText = formatBytes(mainEp.asyncBytes);
  }
  const totalEl = document.getElementById('metric-total');
  if (totalEl) totalEl.innerText = formatBytes(data.totalBytes || 0);

  // Populate Chunk Select options
  const select = document.getElementById('chunk-select');
  if (select) {
    select.innerHTML = '';

    const allOpt = document.createElement('option');
    allOpt.value = 'all';
    allOpt.innerText = `All Chunks (${formatBytes(data.totalBytes || 0)})`;
    select.appendChild(allOpt);

    const initialOpt = document.createElement('option');
    initialOpt.value = 'initial';
    initialOpt.innerText = `Initial Bootstrap (${formatBytes(mainEp ? mainEp.initialBytes : 0)})`;
    select.appendChild(initialOpt);

    for (const chunk of (data.chunks || [])) {
      if (chunk.type === 'async') {
        const opt = document.createElement('option');
        opt.value = chunk.name;
        opt.innerText = `${chunk.name} (${formatBytes(chunk.sizeBytes)})`;
        select.appendChild(opt);
      }
    }

    select.addEventListener('change', () => {
      const val = select.value;
      let filtered = data.topPackages || [];
      if (val === 'initial') {
        const initialChunkNames = (data.chunks || []).filter(c => c.type === 'initial').map(c => c.name);
        const epChunkIds = mainEp ? (mainEp.chunkIds || []) : [];
        const matches = [...new Set([...initialChunkNames, ...epChunkIds])];
        if (matches.length > 0) {
          filtered = (data.topPackages || []).filter(p => (p.chunks || []).some(c => matches.includes(c)));
        }
      } else if (val !== 'all') {
        filtered = (data.topPackages || []).filter(p => (p.chunks || []).includes(val));
      }

      const totalSize = filtered.reduce((acc, p) => acc + (p.sizeBytes || 0), 0);
      treemap.setData(filtered, totalSize);
      renderTable(filtered);

      const q = (document.getElementById('search-input')?.value || '').trim();
      if (q) {
        treemap.setHighlight(q);
        filterTable(q);
      }
    });
  }

  // Render Treemap and Table
  treemap.setData(data.topPackages || [], data.totalBytes || 0);
  renderTable(data.topPackages || []);
}

function renderTable(packages) {
  const tbody = document.getElementById('package-table-body');
  const countEl = document.getElementById('package-count');
  if (countEl) countEl.innerText = `${packages.length} packages`;
  if (!tbody) return;

  tbody.innerHTML = '';

  for (const p of packages) {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-slate-800/40 transition-colors package-row';
    tr.dataset.name = (p.name || '').toLowerCase();

    const safeName = escapeHTML(p.name);
    const safeChunks = (p.chunks || []).map(escapeHTML).join(', ');
    const safeIngress = p.ingressPath ? escapeHTML(p.ingressPath) : '';

    tr.innerHTML = `
      <td class="py-2.5 px-3 font-semibold text-white">${safeName}</td>
      <td class="py-2.5 px-3 text-amber-300 font-bold">${formatBytes(p.sizeBytes)}</td>
      <td class="py-2.5 px-3 text-slate-400">${p.gzipBytes ? '~' + formatBytes(p.gzipBytes) : '--'}</td>
      <td class="py-2.5 px-3 text-slate-400">${safeChunks}</td>
      <td class="py-2.5 px-3 text-indigo-300 text-[11px] truncate max-w-xs" title="${safeIngress}">
        ${safeIngress || '<span class="text-slate-600">--</span>'}
      </td>
    `;
    tbody.appendChild(tr);
  }
}

function escapeHTML(str) {
  return (str || '').replace(/[&<>"']/g, m => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  })[m]);
}

function filterTable(q) {
  const query = (q || '').toLowerCase().trim();
  const rows = document.querySelectorAll('.package-row');
  let matchCount = 0;
  for (const r of rows) {
    if (!query || (r.dataset.name && r.dataset.name.includes(query))) {
      r.style.display = '';
      matchCount++;
    } else {
      r.style.display = 'none';
    }
  }
  const countEl = document.getElementById('package-count');
  if (countEl) {
    countEl.innerText = query ? `${matchCount} matching` : `${rows.length} packages`;
  }
}

function formatBytes(b) {
  if (!b) return '0 B';
  if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
  if (b >= 1024) return (b / 1024).toFixed(2) + ' KB';
  return b + ' B';
}

window.addEventListener('DOMContentLoaded', init);
