'use strict';
const $ = (id) => document.getElementById(id);
let token = '';
try { token = sessionStorage.getItem('tmaegis-token') || ''; } catch (e) { /* storage blocked */ }

// All dynamic text is inserted with textContent, never innerHTML, so report data cannot inject markup.
function cell(text, cls) {
  const td = document.createElement('td');
  td.textContent = text == null ? '' : String(text);
  if (cls) td.className = cls;
  return td;
}
function fill(tbodySel, rows) {
  const body = document.querySelector(tbodySel);
  body.replaceChildren(...rows);
}
async function api(path) {
  const res = await fetch(path, { headers: token ? { Authorization: 'Bearer ' + token } : {} });
  if (!res.ok) {
    const j = await res.json().catch(() => ({}));
    throw new Error(j.error || res.statusText);
  }
  return res.json();
}
function scoreClass(s) { return s >= 85 ? 'good' : s >= 70 ? 'mid' : 'bad'; }

async function loadDashboard() {
  $('msg').textContent = '';
  try {
    const d = await api('/api/dashboard');
    $('avg').textContent = d.host_count ? d.average_score.toFixed(1) + '%' : '–';
    $('hosts').textContent = d.host_count;
    const sev = d.failed_by_severity || {};
    $('crit').textContent = (sev.critical || 0) + (sev.high || 0);
    fill('#host-table tbody', d.hosts.map((h) => {
      const tr = document.createElement('tr');
      tr.dataset.id = h.id;
      tr.append(cell(h.host), cell(h.score.toFixed(1) + '%', scoreClass(h.score)), cell(h.passed), cell(h.failed), cell(new Date(h.received_at).toLocaleString()));
      tr.addEventListener('click', () => showReport(h.id, h.host));
      return tr;
    }));
  } catch (e) { $('msg').textContent = e.message; }
}
async function showReport(id, host) {
  try {
    const st = await api('/api/reports/' + encodeURIComponent(id));
    $('detail-title').textContent = 'Findings for ' + host;
    $('detail-title').hidden = $('detail').hidden = false;
    const failed = (st.report.results || []).filter((r) => r.status !== 'PASS');
    fill('#detail tbody', failed.map((r) => {
      const tr = document.createElement('tr');
      tr.append(cell(r.rule.id + ' ' + r.rule.title), cell(r.rule.severity), cell(r.status, r.status), cell(r.details));
      return tr;
    }));
  } catch (e) { $('msg').textContent = e.message; }
}
async function loadRules() {
  const p = new URLSearchParams();
  for (const k of ['q', 'target', 'severity']) if ($(k).value) p.set(k, $(k).value);
  try {
    const rules = await api('/api/rules?' + p);
    if ($('target').options.length === 1) {
      [...new Set(rules.map((r) => r.target))].sort().forEach((t) => $('target').append(new Option(t, t)));
    }
    fill('#rule-table tbody', rules.map((r) => {
      const tr = document.createElement('tr');
      tr.append(cell(r.id), cell(r.title), cell(r.target), cell(r.level), cell(r.severity));
      return tr;
    }));
  } catch (e) { $('msg').textContent = e.message; }
}

$('auth').addEventListener('submit', (ev) => {
  ev.preventDefault();
  token = $('token').value.trim();
  try { sessionStorage.setItem('tmaegis-token', token); } catch (e) { /* ignore */ }
  loadDashboard();
});
document.querySelectorAll('nav button').forEach((b) => b.addEventListener('click', () => {
  document.querySelectorAll('nav button').forEach((x) => x.classList.toggle('active', x === b));
  for (const t of ['dashboard', 'rules']) $('tab-' + t).hidden = t !== b.dataset.tab;
  (b.dataset.tab === 'rules' ? loadRules : loadDashboard)();
}));
['q', 'target', 'severity'].forEach((k) => $(k).addEventListener('input', loadRules));
if (token) loadDashboard(); else $('msg').textContent = 'Enter the server token to view fleet data.';
