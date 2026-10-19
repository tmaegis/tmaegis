// TMAegis local GUI: vanilla JS, no dependencies, no network access except this server.
// Every user-visible string comes from the gui.* catalog (internal/i18n/locales/<tag>/gui.json).
'use strict';

// ---------- helpers ----------
const SVGNS = 'http://www.w3.org/2000/svg';
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const SEVS = ['critical', 'high', 'medium', 'low'];
const SEV_ALL = [...SEVS, 'other'];
const CATS = ['os', 'server', 'database', 'containers', 'desktop', 'network', 'cloud', 'mobile', 'devsecops', 'ai', 'hardware', 'other'];
let S = null;            // /api/session
let MSG = {};            // catalog for the current language
let LANG = 'en-US';
let PR = new Intl.PluralRules('en-US');
let NF = new Intl.NumberFormat('en-US');
let PF = new Intl.NumberFormat('en-US', { style: 'percent', maximumFractionDigits: 0 });
let DF = new Intl.DateTimeFormat('en-US', { dateStyle: 'medium', timeStyle: 'short' });
let RTF = null;
let cleanup = null;      // tear-down of the current view
let viewKeys = null;     // keyboard handler of the current view
let PENDING_FIX = null;  // { report, ids } handed from Results to Fix
let uid = 0;
const nextId = (p) => p + '-' + (++uid);

function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'text') el.textContent = v;
    else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const c of kids.flat(3)) {
    if (c == null || c === false) continue;
    el.append(c.nodeType ? c : document.createTextNode(String(c)));
  }
  return el;
}
// replaceChildren that, like h(), skips null/false and flattens arrays (views pass optional parts)
const nativeReplace = Element.prototype.replaceChildren;
Element.prototype.replaceChildren = function (...kids) { return nativeReplace.apply(this, kids.flat(3).filter((x) => x != null && x !== false)); };
function put(root, ...kids) { root.append(...kids.flat(3).filter((x) => x != null && x !== false)); }
function svgEl(tag, attrs, ...kids) {
  const el = document.createElementNS(SVGNS, tag);
  for (const [k, v] of Object.entries(attrs || {})) if (v != null) el.setAttribute(k, v);
  for (const c of kids.flat(2)) if (c) el.append(c);
  return el;
}
function icon(id, cls) {
  const s = svgEl('svg', { class: 'ico' + (cls ? ' ' + cls : ''), 'aria-hidden': 'true', focusable: 'false' });
  s.append(svgEl('use', { href: '#' + (id.startsWith('c-') ? id : 'i-' + id) }));
  return s;
}
const catIcon = (c, cls) => icon('c-' + (CATS.includes(c) ? c : 'other'), cls);
const glyph = (c, small) => h('span', { class: 'glyph' + (small ? ' sm' : ''), 'aria-hidden': 'true' }, catIcon(c));
// Platform logos, monograms and glyphs from the website's icon set (assets/platform-icons.svg, one cached
// sprite; see tools/gen-platform-icons.mjs). Always decorative: the product name is the visible text beside
// the icon. Full-colour logos keep their own colours (on a contrasting chip where they would vanish); the others
// are monochrome (currentColor), and their brand colour shows on hover or for a selected item, only in a theme
// where it has 3:1 contrast. o.tile draws it on a small tile ('sm' or 'md' size), o.cat is the
// category glyph shown for a target without an icon, o.active shows the brand colour now.
const ICONS = () => (S && S.icons) || { targets: {}, brands: {}, names: {} };
const platName = (id) => ICONS().names[id] || id;
function picon(id, o = {}) {
  const ic = ICONS();
  let sym = ic.targets[String(id || '').toLowerCase()];
  const mono = !!sym && sym.startsWith('m-');
  const cls = 'picon' + (o.tile ? ' tile ' + (o.size || 'sm') : '') + (mono ? ' mono' : '') + (o.cls ? ' ' + o.cls : '');
  const box = h('span', { class: cls, 'aria-hidden': 'true', 'data-active': o.active ? '' : null });
  if (!sym) { box.append(catIcon(o.cat || 'other')); return box; }
  if (mono && !o.tile) sym = 'mf-' + sym.slice(2); // framed monogram next to text
  const b = ic.brands[sym];
  if (b && b.wide) box.classList.add('wide'); // wordmark (IBM, Cisco): more of the tile, for the same weight
  if (b && b.chip) box.dataset.chip = b.chip; // full-colour logo that would vanish on this theme's background
  if (b && !b.color) { // full-colour logos keep their own colours; single-colour marks take the brand colour on hover
    box.dataset.b = '';
    if (b.dark) box.style.setProperty('--bd', '#' + b.hex);
    if (b.light) box.style.setProperty('--bl', '#' + b.hex);
  }
  box.append(svgEl('svg', { class: 'pi', viewBox: '0 0 24 24', focusable: 'false', 'aria-hidden': 'true' }, svgEl('use', { href: ic.sprite + '#' + sym })));
  return box;
}
const hasIcon = (id) => !!id && !!ICONS().targets[String(id).toLowerCase()];
// The icons of a scan's targets ("a,b,c"), at most four, for the history list.
function scopeIcons(list) {
  const ids = String(list || '').split(',').filter(hasIcon);
  if (!ids.length) return null;
  return h('span', { class: 'picons', 'aria-hidden': 'true' }, ids.slice(0, 4).map((x) => picon(x)), ids.length > 4 ? h('span', { class: 'more', text: '+' + num(ids.length - 4) }) : null);
}
const fmt = (s, v) => (v ? s.replace(/\{(\w+)\}/g, (m, n) => (n in v ? v[n] : m)) : s);
const t = (k, v) => fmt(MSG[k] ?? k, v);
const hasKey = (k) => k in MSG;
function nn(k, c, v) {
  const s = MSG[k + '.' + PR.select(c)] ?? MSG[k + '.other'] ?? k;
  return fmt(s, { count: NF.format(c), ...v });
}
const fmtDate = (iso) => (iso ? DF.format(new Date(iso)) : '');
const num = (n) => NF.format(n);
const pct = (n) => PF.format(Math.max(0, Math.min(100, n || 0)) / 100);
function ago(iso) {
  if (!iso || !RTF) return fmtDate(iso);
  const s = (new Date(iso).getTime() - Date.now()) / 1000;
  const a = Math.abs(s);
  if (a < 60) return RTF.format(Math.round(s), 'second');
  if (a < 3600) return RTF.format(Math.round(s / 60), 'minute');
  if (a < 86400) return RTF.format(Math.round(s / 3600), 'hour');
  return RTF.format(Math.round(s / 86400), 'day');
}
function dur(sec) {
  sec = Math.max(0, Math.round(sec));
  if (sec < 60) return t('gui.time.sec', { s: num(sec) });
  if (sec < 3600) return t('gui.time.min_sec', { m: num(Math.floor(sec / 60)), s: num(sec % 60) });
  return t('gui.time.hour_min', { h: num(Math.floor(sec / 3600)), m: num(Math.floor((sec % 3600) / 60)) });
}
function bytes(n) {
  if (n < 1024) return t('gui.size.b', { n: num(n) });
  if (n < 1048576) return t('gui.size.kb', { n: NF.format(Math.round(n / 102.4) / 10) });
  return t('gui.size.mb', { n: NF.format(Math.round(n / 104857.6) / 10) });
}
const announce = (msg) => { const a = $('#announce'); a.textContent = ''; setTimeout(() => { a.textContent = msg; }, 30); };

// ---------- API, connection state ----------
class ApiError extends Error {
  constructor(status, code, message) { super(message); this.status = status; this.code = code; }
}
let offline = false, retryTimer = null;
async function api(path, body, method) {
  const opt = { method: method || (body === undefined ? 'GET' : 'POST'), headers: {}, credentials: 'same-origin' };
  if (opt.method !== 'GET') {
    opt.headers['X-TMAegis-Nonce'] = S ? S.nonce : '';
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body || {});
  }
  let res;
  try { res = await fetch(path, opt); } catch (e) { setOffline(true); throw new ApiError(0, 'offline', t('gui.err.offline')); }
  if (offline) setOffline(false);
  let data = null;
  try { data = await res.json(); } catch (e) { /* not JSON */ }
  if (!res.ok) {
    const er = (data && data.error) || {};
    throw new ApiError(res.status, er.code || 'internal', er.message || res.statusText);
  }
  return data;
}
function errText(e) {
  if (e instanceof ApiError) {
    const k = 'gui.err.' + e.code;
    return hasKey(k) ? t(k) : e.message;
  }
  return String((e && e.message) || e);
}
function setOffline(v) {
  if (v === offline) return;
  offline = v;
  const b = $('#offline');
  if (v) {
    b.replaceChildren(icon('offline'), h('span', { text: t('gui.shell.offline_banner') }),
      h('button', { class: 'btn sm', type: 'button', onclick: ping }, icon('refresh'), t('gui.state.retry')));
    b.classList.remove('hidden');
    clearInterval(retryTimer);
    retryTimer = setInterval(ping, 4000);
  } else {
    b.classList.add('hidden');
    clearInterval(retryTimer);
    toast(t('gui.shell.reconnected'), 'ok');
  }
  renderStatus();
}
async function ping() { try { await fetch('/api/session', { credentials: 'same-origin' }).then((r) => { if (r.ok) setOffline(false); }); } catch (e) { /* still offline */ } }

function toast(msg, kind) {
  kind = kind === true ? 'err' : (kind || 'ok');
  const box = $('#toasts');
  const close = h('button', { class: 'btn ghost icon sm', type: 'button', 'aria-label': t('gui.dialog.close') }, icon('x'));
  const d = h('div', { class: 'toast ' + kind, role: kind === 'err' ? 'alert' : null }, icon(kind === 'err' ? 'alert' : kind === 'info' ? 'info' : 'check'), h('div', { class: 'msg', text: msg }), close);
  close.addEventListener('click', () => d.remove());
  box.append(d);
  setTimeout(() => d.remove(), kind === 'err' ? 10000 : 5000);
}
function scoreClass(s) { return s >= 80 ? 'good' : s >= 50 ? 'fair' : 'poor'; }
function sevKey(s) { s = String(s || '').toLowerCase(); return SEVS.includes(s) ? s : 'other'; }
const sevRank = (s) => SEV_ALL.indexOf(sevKey(s));
function sevPill(s) { const k = sevKey(s); return h('span', { class: 'pill sev-' + k }, h('span', { class: 'sev-dot', 'aria-hidden': 'true' }), t('gui.sev.' + k)); }
const STATUS_ICON = { PASS: 'check', FAIL: 'x', ERROR: 'alert', SKIP: 'skip' };
function statusPill(st, compact) {
  const k = String(st || 'SKIP').toLowerCase();
  const label = t('gui.status.' + k);
  return h('span', { class: 'pill st-' + k, title: compact ? label : null }, icon(STATUS_ICON[st] || 'skip'), h('span', { class: 'lbl', text: label }));
}
function riskPill(r) { return h('span', { class: 'pill risk-' + r }, icon(r === 'high' ? 'alert' : r === 'medium' ? 'info' : 'shield'), t('gui.fx.risk_' + r)); }
function copyBtn(text, label) {
  const b = h('button', { class: 'btn sm', type: 'button', 'aria-label': label ? t('gui.copy.what', { what: label }) : null }, icon('copy'), h('span', { text: t('gui.copy.button') }));
  b.addEventListener('click', async () => {
    try { await navigator.clipboard.writeText(text); toast(t('gui.copy.done')); }
    catch (e) {
      try { const ta = h('textarea', {}); ta.value = text; document.body.append(ta); ta.select(); document.execCommand('copy'); ta.remove(); toast(t('gui.copy.done')); }
      catch (e2) { toast(t('gui.copy.failed'), 'err'); }
    }
  });
  return b;
}
function cmdBox(text, label) { return h('div', { class: 'cmd' }, h('code', { tabindex: 0, text }), copyBtn(text, label)); }
function gauge(score, small) {
  const R = 54, C = 2 * Math.PI * R, cls = scoreClass(score), gid = nextId('gg');
  const svg = svgEl('svg', { viewBox: '0 0 132 132', 'aria-hidden': 'true' },
    svgEl('defs', {}, svgEl('linearGradient', { id: gid, x1: '0', y1: '0', x2: '1', y2: '1' },
      svgEl('stop', { offset: '0', 'stop-color': cls === 'poor' ? '#f87171' : cls === 'fair' ? '#facc15' : '#00F2FE' }),
      svgEl('stop', { offset: '1', 'stop-color': cls === 'poor' ? '#fb923c' : cls === 'fair' ? '#fb923c' : '#10B981' }))),
    svgEl('circle', { class: 'bg', cx: 66, cy: 66, r: R }));
  const fg = svgEl('circle', { class: 'fg', cx: 66, cy: 66, r: R, stroke: 'url(#' + gid + ')', 'stroke-dasharray': C, 'stroke-dashoffset': C });
  svg.append(fg);
  requestAnimationFrame(() => requestAnimationFrame(() => fg.setAttribute('stroke-dashoffset', C * (1 - Math.max(0, Math.min(100, score)) / 100))));
  return h('div', { class: 'gauge ' + cls + (small ? ' sm' : ''), role: 'img', 'aria-label': t('gui.score.aria', { score: Math.round(score) }) },
    svg, h('div', { class: 'num', 'aria-hidden': 'true' }, h('div', {}, h('b', { text: pct(score) }), h('small', { text: t('gui.score.label') }))));
}
function emptyState(ico, title, text, ...actions) {
  return h('div', { class: 'card empty' }, h('div', { class: 'art', 'aria-hidden': 'true' }, icon(ico, 'xl')), h('h2', { text: title }), h('p', { text }),
    actions.length ? h('div', { class: 'actions' }, actions) : null);
}
function errorState(e, retry) {
  return h('div', { class: 'card empty', role: 'alert' }, h('div', { class: 'art', 'aria-hidden': 'true' }, icon(e && e.code === 'offline' ? 'offline' : 'alert', 'xl')),
    h('h2', { text: t('gui.state.error_title') }), h('p', { text: errText(e) }),
    retry ? h('div', { class: 'actions' }, h('button', { class: 'btn primary', type: 'button', onclick: retry }, icon('refresh'), t('gui.state.retry'))) : null);
}
function skeleton() {
  const line = (w) => { const d = h('div', { class: 'sk sk-line' }); d.style.width = w + '%'; return d; };
  return h('div', { 'aria-busy': 'true' }, h('span', { class: 'sr', text: t('gui.state.loading') }),
    h('div', { class: 'ph' }, h('div', { class: 'grow' }, line(28), line(52))),
    h('div', { class: 'grid' }, h('div', { class: 'card s8' }, h('div', { class: 'sk sk-block' }), line(70), line(45)),
      h('div', { class: 'card s4' }, line(60), line(80), line(40), line(65)),
      h('div', { class: 'card s12' }, line(90), line(75), line(85))));
}
function notice(kind, ico, ...kids) { return h('div', { class: 'notice ' + kind, role: kind === 'err' ? 'alert' : null }, icon(ico), h('div', { class: 'grow' }, kids)); }
function pageHead(titleKey, ledeText, actions, eyebrowKey) {
  return h('div', { class: 'ph' }, h('div', { class: 'grow' },
    eyebrowKey ? h('div', { class: 'eyebrow', text: t(eyebrowKey) }) : null,
    h('h1', { class: 'page', text: t(titleKey) }),
    ledeText ? h('p', { class: 'lede', text: ledeText }) : null),
  actions && actions.length ? h('div', { class: 'row' }, actions) : null);
}
function seg(label, options, value, onchange) {
  const g = h('div', { class: 'seg', role: 'group', 'aria-label': label });
  for (const [v, text, ico] of options) {
    const b = h('button', { type: 'button', 'aria-pressed': String(v === value), 'data-v': v }, ico ? icon(ico) : null, text);
    b.addEventListener('click', () => { for (const x of g.children) x.setAttribute('aria-pressed', String(x === b)); onchange(v); });
    g.append(b);
  }
  return g;
}
function toggleChip(text, on, onchange, title) {
  const b = h('button', { class: 'chip', type: 'button', 'aria-pressed': String(!!on), title }, text);
  b.addEventListener('click', () => { const v = b.getAttribute('aria-pressed') !== 'true'; b.setAttribute('aria-pressed', String(v)); onchange(v); });
  return b;
}
function setWidth(el, p) { el.style.width = Math.max(0, Math.min(100, p)) + '%'; }

// tablist with arrow-key navigation (WAI-ARIA tabs pattern)
function tablist(label, tabs, active, onselect, cls) {
  const list = h('div', { class: cls || 'cat-tabs', role: 'tablist', 'aria-label': label });
  const btns = tabs.map((tb) => h('button', { type: 'button', role: 'tab', id: nextId('tab'), 'aria-selected': String(tb.id === active), tabindex: tb.id === active ? '0' : '-1' }, tb.icon || null, h('span', { text: tb.label }), tb.count != null ? h('span', { class: 'n', text: num(tb.count) }) : null));
  const pick = (i, focus) => {
    btns.forEach((b, j) => { b.setAttribute('aria-selected', String(i === j)); b.tabIndex = i === j ? 0 : -1; });
    if (focus) btns[i].focus();
    onselect(tabs[i].id);
  };
  btns.forEach((b, i) => {
    b.addEventListener('click', () => pick(i));
    b.addEventListener('keydown', (e) => {
      const rtl = document.documentElement.dir === 'rtl';
      const fwd = rtl ? 'ArrowLeft' : 'ArrowRight', back = rtl ? 'ArrowRight' : 'ArrowLeft';
      const n = e.key === fwd ? (i + 1) % btns.length : e.key === back ? (i - 1 + btns.length) % btns.length : e.key === 'Home' ? 0 : e.key === 'End' ? btns.length - 1 : -1;
      if (n >= 0) { e.preventDefault(); pick(n, true); }
    });
  });
  list.append(...btns);
  return list;
}

// ---------- i18n + theme ----------
async function loadLang(tag) {
  const d = await api('/api/i18n/' + encodeURIComponent(tag));
  MSG = d.messages;
  LANG = tag;
  document.documentElement.lang = tag;
  try {
    PR = new Intl.PluralRules(tag); NF = new Intl.NumberFormat(tag);
    PF = new Intl.NumberFormat(tag, { style: 'percent', maximumFractionDigits: 0 });
    DF = new Intl.DateTimeFormat(tag, { dateStyle: 'medium', timeStyle: 'short' });
    RTF = new Intl.RelativeTimeFormat(tag, { numeric: 'auto' });
  } catch (e) { /* keep en-US */ }
  for (const el of $$('[data-i18n]')) el.textContent = t(el.dataset.i18n);
  for (const el of $$('[data-i18n-aria]')) el.setAttribute('aria-label', t(el.dataset.i18nAria));
  renderStatus();
  renderTheme();
}
function applyTheme(theme) {
  if (theme === 'light' || theme === 'dark') document.documentElement.setAttribute('data-theme', theme);
  else document.documentElement.removeAttribute('data-theme');
  renderTheme();
}
const THEMES = ['system', 'light', 'dark'];
const THEME_ICON = { system: 'monitor', light: 'sun', dark: 'moon' };
function renderTheme() {
  const b = $('#theme');
  if (!b || !S) return;
  const cur = S.settings.theme || 'system';
  const next = THEMES[(THEMES.indexOf(cur) + 1) % THEMES.length];
  b.replaceChildren(icon(THEME_ICON[cur]));
  b.setAttribute('aria-label', t('gui.theme.toggle', { theme: t('gui.theme.' + cur), next: t('gui.theme.' + next) }));
  b.title = t('gui.theme.toggle', { theme: t('gui.theme.' + cur), next: t('gui.theme.' + next) });
  b.setAttribute('aria-keyshortcuts', 'T');
}
async function saveSettings(patch) {
  S.settings = await api('/api/settings', { ...S.settings, ...patch });
  applyTheme(S.settings.theme);
  return S.settings;
}
async function cycleTheme() {
  const cur = S.settings.theme || 'system';
  const next = THEMES[(THEMES.indexOf(cur) + 1) % THEMES.length];
  applyTheme(next); // instant feedback, then persist
  try { await saveSettings({ theme: next }); announce(t('gui.theme.changed', { theme: t('gui.theme.' + next) })); } catch (e) { toast(errText(e), 'err'); }
  for (const r of $$('input[name=theme]')) r.checked = r.value === S.settings.theme;
}
async function changeLang(tag) {
  await saveSettings({ lang: tag });
  S.lang = tag;
  await loadLang(tag);
  for (const id of ['#lang', '#lang-m']) $(id).value = tag;
  route();
}
function renderStatus() {
  const p = $('#status');
  if (!p || !S) return;
  p.className = 'chip-status' + (offline ? ' off' : '');
  p.title = S.elevated ? t('gui.admin.ok') : t('gui.admin.missing_body');
  p.replaceChildren(h('span', { class: 'dot', 'aria-hidden': 'true' }), h('span', { class: 'lbl', text: offline ? t('gui.shell.offline') : t('gui.shell.connected') }),
    h('span', { class: 'sep', 'aria-hidden': 'true' }),
    h('span', { class: 'el ' + (S.elevated ? 'ok' : 'warn') }, icon(S.elevated ? 'shield' : 'lock'), h('span', { class: 'lbl', text: S.elevated ? t('gui.priv.elevated') : t('gui.priv.standard') })),
    h('span', { class: 'sr', text: offline ? t('gui.shell.offline') : '' }));
}

// ---------- elevation help ----------
function elevateHelp() {
  const quoted = '"' + S.exe + '" gui';
  const cmd = S.os === 'windows' ? quoted : 'sudo ' + quoted;
  const key = S.os === 'windows' ? 'gui.admin.how.windows' : S.os === 'darwin' ? 'gui.admin.how.macos' : 'gui.admin.how.linux';
  return h('div', {}, h('p', { text: t(key) }), cmdBox(cmd, t('gui.admin.command')));
}
function elevationNotice(titleKey, bodyKey) {
  return notice('warn', 'lock', h('p', {}, h('b', { text: t(titleKey) })), h('p', { text: t(bodyKey) }),
    h('details', {}, h('summary', { text: t('gui.admin.how_title') }), elevateHelp()));
}

// ---------- dialogs ----------
const dlg = $('#dlg');
const drawer = $('#drawer');
let dlgLocked = false;
dlg.addEventListener('cancel', (e) => { if (dlgLocked) e.preventDefault(); });
dlg.addEventListener('close', () => { dlgLocked = false; dlg.replaceChildren(); });
let lastFocus = null;
function showDialog(title, body, buttons, lock) {
  dlgLocked = !!lock;
  dlg.replaceChildren(
    h('div', { class: 'hd' }, h('h2', { id: 'dlg-title', text: title }),
      lock ? null : h('button', { class: 'btn ghost icon sm', type: 'button', 'aria-label': t('gui.dialog.close'), onclick: () => dlg.close() }, icon('x'))),
    h('div', { class: 'bd' }, body),
    buttons && buttons.length ? h('div', { class: 'ft' }, buttons) : null);
  if (!dlg.open) { lastFocus = document.activeElement; dlg.showModal(); }
  const af = $('[autofocus]', dlg) || $('.ft .btn', dlg);
  if (af) af.focus();
}
dlg.addEventListener('close', () => { if (lastFocus && lastFocus.isConnected) lastFocus.focus(); });
const closeBtn = (key, focus) => h('button', { class: 'btn', type: 'button', autofocus: focus ? true : null, onclick: () => dlg.close() }, t(key || 'gui.dialog.close'));

function showKeys() {
  const rows = [
    [['?'], 'gui.keys.help'], [['/'], 'gui.keys.search'], [['T'], 'gui.keys.theme'],
    [['G', 'D'], 'gui.nav.dashboard'], [['G', 'N'], 'gui.nav.new_scan'], [['G', 'R'], 'gui.nav.results'], [['G', 'F'], 'gui.nav.fix'],
    [['G', 'H'], 'gui.nav.history'], [['G', 'P'], 'gui.nav.platforms'], [['G', 'S'], 'gui.nav.settings'], [['G', 'A'], 'gui.nav.about'],
    [['J'], 'gui.keys.next_row'], [['K'], 'gui.keys.prev_row'], [['Enter'], 'gui.keys.open_row'], [['X'], 'gui.keys.select_row'], [['Esc'], 'gui.keys.close'],
  ];
  showDialog(t('gui.keys.title'), [h('p', { class: 'hint', text: t('gui.keys.intro') }),
    h('dl', { class: 'keys' }, rows.map(([ks, k]) => h('div', {}, h('dt', {}, ks.map((x) => h('kbd', { text: x }))), h('dd', { text: t(k) }))))],
  [closeBtn('gui.dialog.close', true)]);
}

// ---------- router ----------
const views = {};
const TITLES = { dashboard: 'gui.dashboard.title', scan: 'gui.nav.new_scan', results: 'gui.results.title', fix: 'gui.fix.title', history: 'gui.history.title', platforms: 'gui.nav.platforms', settings: 'gui.settings.title', about: 'gui.about.title' };
let routeSeq = 0;
async function route() {
  const seq = ++routeSeq;
  if (cleanup) { try { cleanup(); } catch (e) { /* ignore */ } cleanup = null; }
  viewKeys = null;
  if (drawer.open) drawer.close();
  closeNav();
  const raw = location.hash.replace(/^#/, '') || '/dashboard';
  const [path, qs] = raw.split('?');
  const parts = path.split('/').filter(Boolean);
  const name = views[parts[0]] ? parts[0] : 'dashboard';
  for (const a of $$('#nav a')) {
    if (a.dataset.route === name) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
  }
  document.title = t(TITLES[name]) + ' · ' + t('gui.app.title');
  const main = $('#main');
  main.replaceChildren(skeleton());
  const root = h('div', { class: 'view' });
  const query = Object.fromEntries(new URLSearchParams(qs || ''));
  try {
    const c = await views[name](root, parts[1], query, parts.slice(2));
    if (seq !== routeSeq) { if (typeof c === 'function') c(); return; }
    if (typeof c === 'function') cleanup = c;
  } catch (e) {
    if (seq !== routeSeq) return;
    root.replaceChildren(errorState(e, route));
  }
  main.replaceChildren(root);
  const h1 = $('h1', main);
  if (h1) { h1.tabIndex = -1; h1.focus({ preventScroll: true }); }
  window.scrollTo(0, 0);
}
function closeNav() { document.body.classList.remove('nav-open'); $('#menu').setAttribute('aria-expanded', 'false'); }

// ---------- keyboard shortcuts ----------
let gAt = 0;
document.addEventListener('keydown', (e) => {
  if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key === 'Escape' && document.body.classList.contains('nav-open')) { closeNav(); $('#menu').focus(); return; }
  const tg = e.target;
  if (tg.closest && tg.closest('input,textarea,select,[contenteditable=true]')) return;
  if (dlg.open) return;
  if (drawer.open) { if (drawerKeys) drawerKeys(e); return; }
  if (e.key === '?') { e.preventDefault(); showKeys(); return; }
  if (e.key === '/') { const s = $('[data-search]'); if (s) { e.preventDefault(); s.focus(); s.select(); } return; }
  const k = e.key.toLowerCase();
  if (gAt && Date.now() - gAt < 1500) {
    gAt = 0;
    const m = { d: 'dashboard', n: 'scan', r: 'results', f: 'fix', h: 'history', p: 'platforms', s: 'settings', a: 'about' }[k];
    if (m) { e.preventDefault(); location.hash = '#/' + m; }
    return;
  }
  if (k === 'g') { gAt = Date.now(); return; }
  if (k === 't') { e.preventDefault(); cycleTheme(); return; }
  if (viewKeys) viewKeys(e);
});
let drawerKeys = null;

// ---------- charts (inline SVG, never colour-only) ----------
const PATTERNS = {
  critical: (p) => p.append(svgEl('path', { d: 'M0 0L8 8M8 0L0 8', stroke: 'currentColor', 'stroke-width': '1.6' })),
  high: (p) => p.append(svgEl('path', { d: 'M-2 2L2 -2M0 8L8 0M6 10L10 6', stroke: 'currentColor', 'stroke-width': '1.8' })),
  medium: (p) => p.append(svgEl('circle', { cx: '4', cy: '4', r: '1.7', fill: 'currentColor' })),
  low: (p) => p.append(svgEl('path', { d: 'M0 4H8', stroke: 'currentColor', 'stroke-width': '1.8' })),
  other: (p) => p.append(svgEl('path', { d: 'M4 0V8', stroke: 'currentColor', 'stroke-width': '1.2' })),
};
function patternDefs(prefix) {
  const defs = svgEl('defs');
  for (const k of SEV_ALL) {
    const p = svgEl('pattern', { id: prefix + k, class: 'sev-' + k, patternUnits: 'userSpaceOnUse', width: '8', height: '8' });
    p.append(svgEl('rect', { width: '8', height: '8', fill: 'currentColor', 'fill-opacity': '.35' }));
    PATTERNS[k](p);
    defs.append(p);
  }
  return defs;
}
// donut of failing controls by severity; each segment has its own hatch pattern and the
// legend repeats the numbers as text.
function donut(counts, label) {
  const pre = nextId('pt') + '-';
  const total = SEV_ALL.reduce((n, k) => n + (counts[k] || 0), 0);
  const R = 50, C = 2 * Math.PI * R;
  const svg = svgEl('svg', { class: 'donut', viewBox: '0 0 140 140', role: 'img', 'aria-label': label });
  svg.append(patternDefs(pre), svgEl('circle', { cx: 70, cy: 70, r: R, fill: 'none', stroke: 'var(--muted-2)', 'stroke-width': 22, class: 'trk' }));
  let off = 0;
  for (const k of SEV_ALL) {
    const v = counts[k] || 0;
    if (!v) continue;
    const len = C * v / total;
    const g = svgEl('g', { class: 'sev-' + k, transform: 'rotate(-90 70 70)' });
    g.append(svgEl('circle', { cx: 70, cy: 70, r: R, fill: 'none', stroke: 'url(#' + pre + k + ')', 'stroke-width': 22, 'stroke-dasharray': Math.max(0, len - 1.5) + ' ' + C, 'stroke-dashoffset': -off }),
      svgEl('circle', { cx: 70, cy: 70, r: R + 11, fill: 'none', stroke: 'currentColor', 'stroke-width': 2, 'stroke-dasharray': Math.max(0, len - 1.5) + ' ' + C, 'stroke-dashoffset': -off * (R + 11) / R }));
    svg.append(g);
    off += len;
  }
  const tx = svgEl('text', { x: 70, y: 68, 'text-anchor': 'middle', class: 'dn-n' }); tx.textContent = num(total);
  const tl = svgEl('text', { x: 70, y: 86, 'text-anchor': 'middle', class: 'dn-l' }); tl.textContent = t('gui.dash.failing_short');
  svg.append(tx, tl);
  const legend = h('ul', { class: 'legend' }, SEV_ALL.filter((k) => counts[k]).map((k) => {
    const sw = svgEl('svg', { class: 'sw sev-' + k, viewBox: '0 0 16 16', 'aria-hidden': 'true' }, svgEl('rect', { width: '16', height: '16', fill: 'url(#' + pre + k + ')' }));
    return h('li', { class: 'sev-' + k + '-t' }, sw, h('span', { text: t('gui.sev.' + k) }), h('span', { class: 'v', text: num(counts[k]) }));
  }));
  return h('div', { class: 'donut-wrap' }, svg, legend);
}
// score trend line, redrawn to the real width so labels and dots stay round
function trendChart(trend, height) {
  const H = height || 110, box = h('div', { class: 'trend' });
  const table = h('table', { class: 'sr' }, h('caption', { text: t('gui.history.trend') }),
    h('thead', {}, h('tr', {}, h('th', { scope: 'col', text: t('gui.history.col_date') }), h('th', { scope: 'col', text: t('gui.history.col_score') }))),
    h('tbody', {}, trend.map((p) => h('tr', {}, h('td', { text: fmtDate(p.time) }), h('td', { text: pct(p.score) })))));
  const draw = () => {
    const W = Math.max(240, Math.floor(box.getBoundingClientRect().width) || 600), padL = 34, padR = 10, padT = 10, padB = 18;
    const x = (i) => padL + (trend.length === 1 ? (W - padL - padR) / 2 : i * (W - padL - padR) / (trend.length - 1));
    const y = (s) => padT + (1 - s / 100) * (H - padT - padB);
    const svg = svgEl('svg', { class: 'spark', viewBox: '0 0 ' + W + ' ' + H, width: W, height: H, role: 'img',
      'aria-label': t('gui.history.trend_aria', { first: Math.round(trend[0].score), last: Math.round(trend[trend.length - 1].score) }) });
    for (const g of [0, 50, 100]) {
      svg.append(svgEl('line', { class: 'grid-l', x1: padL, x2: W - padR, y1: y(g), y2: y(g) }));
      const tx = svgEl('text', { x: padL - 6, y: y(g) + 4, 'text-anchor': 'end' }); tx.textContent = pct(g); svg.append(tx);
    }
    const pts = trend.map((p, i) => x(i) + ',' + y(p.score));
    svg.append(svgEl('polygon', { class: 'ar', points: x(0) + ',' + y(0) + ' ' + pts.join(' ') + ' ' + x(trend.length - 1) + ',' + y(0) }),
      svgEl('polyline', { class: 'ln', points: pts.join(' ') }));
    trend.forEach((p, i) => {
      const c = svgEl('circle', { cx: x(i), cy: y(p.score), r: i === trend.length - 1 ? 4.5 : 3 });
      const tt = svgEl('title'); tt.textContent = fmtDate(p.time) + ': ' + pct(p.score); c.append(tt); svg.append(c);
    });
    const f = svgEl('text', { x: padL, y: H - 2 }); f.textContent = fmtDate(trend[0].time);
    const l = svgEl('text', { x: W - padR, y: H - 2, 'text-anchor': 'end' }); l.textContent = fmtDate(trend[trend.length - 1].time);
    if (trend.length > 1) svg.append(f, l);
    box.replaceChildren(svg, table);
  };
  const ro = new ResizeObserver(() => draw());
  ro.observe(box);
  requestAnimationFrame(draw);
  return box;
}

// ---------- Dashboard ----------
async function startAutoScan(btn) {
  if (btn) btn.disabled = true;
  try {
    const j = await api('/api/scan', { scope: 'auto', level: 1 });
    location.hash = '#/scan/' + j.id;
  } catch (e) {
    if (e.code === 'busy') location.hash = '#/scan';
    else { toast(errText(e), 'err'); if (btn) btn.disabled = false; }
  }
}
views.dashboard = async (root) => {
  const d = await api('/api/dashboard');
  const last = d.last, hs = d.host;
  const scanBtn = h('button', { class: 'btn primary lg', type: 'button' }, icon('radar'), t('gui.dashboard.scan_button'));
  scanBtn.addEventListener('click', () => startAutoScan(scanBtn));
  put(root, pageHead('gui.dashboard.title', t('gui.dashboard.lede'), null, 'gui.dash.eyebrow'));

  let hero;
  if (last) {
    const prev = d.previous;
    const delta = prev ? last.score - prev.score : null;
    const dl = delta == null ? null : h('span', { class: 'delta ' + (delta > 0.05 ? 'up' : delta < -0.05 ? 'down' : '') },
      t(delta > 0.05 ? 'gui.dash.delta_up' : delta < -0.05 ? 'gui.dash.delta_down' : 'gui.dash.delta_same', { n: NF.format(Math.abs(Math.round(delta * 10) / 10)) }));
    hero = h('section', { class: 'card s8', 'aria-labelledby': 'h-last' },
      h('div', { class: 'card-head' }, h('h2', { class: 'sec', id: 'h-last' }, icon('shield'), t('gui.dashboard.last_scan')),
        h('span', { class: 'act hint', title: fmtDate(last.created), text: ago(last.created) })),
      h('div', { class: 'hero' }, gauge(last.score),
        h('div', { class: 'info' },
          h('div', { class: 'row' }, h('p', { class: 'verdict ' + scoreClass(last.score), text: t('gui.verdict.' + scoreClass(last.score)) }), dl),
          h('p', { class: 'muted', text: t('gui.dash.last_detail', { host: last.hostname || '-', date: fmtDate(last.created), total: num(last.total) }) }),
          h('div', { class: 'kpis' },
            kpi('pass', 'check', t('gui.status.pass'), last.passed), kpi('fail', 'x', t('gui.status.fail'), last.failed),
            kpi('skip', 'skip', t('gui.status.skip'), last.skipped), kpi('total', 'list', t('gui.results.total'), last.total)),
          h('div', { class: 'actions' }, scanBtn,
            h('a', { class: 'btn', href: '#/results/' + last.id }, icon('list'), t('gui.dashboard.open_results')),
            d.fixable ? h('a', { class: 'btn', href: '#/fix/' + last.id }, icon('fix'), nn('gui.results.fix', d.fixable)) : null))));
  } else {
    hero = h('section', { class: 'card s8', 'aria-labelledby': 'h-welcome' },
      h('div', { class: 'hero' }, h('div', { class: 'scanpulse', 'aria-hidden': 'true' }, h('img', { src: '/static/logo-mark.svg', alt: '' })),
        h('div', { class: 'info' }, h('h2', { class: 'verdict', id: 'h-welcome', text: t('gui.dash.welcome_title') }),
          h('p', { class: 'muted', text: t('gui.dashboard.no_scan') }),
          h('div', { class: 'actions' }, scanBtn, h('a', { class: 'btn', href: '#/scan' }, icon('layers'), t('gui.dashboard.custom_scan'))),
          h('p', { class: 'hint' }, icon('lock'), ' ', t('gui.dashboard.scan_hint')))));
  }

  const dist = [hs.distro, hs.version].filter(Boolean).join(' ');
  const detected = h('section', { class: 'card s4', 'aria-labelledby': 'h-host' },
    h('h2', { class: 'sec', id: 'h-host' }, icon('monitor'), t('gui.dash.detected')),
    h('dl', { class: 'kv' },
      h('dt', { text: t('gui.host.os') }), h('dd', { class: 'plat' }, hasIcon(hs.os) ? picon(hs.os) : null, h('span', { text: hs.os + ' / ' + S.arch })),
      h('dt', { text: t('gui.host.distro') }), h('dd', { class: 'plat' }, hasIcon(hs.distro) ? picon(hs.distro) : null, h('span', { text: dist || t('gui.host.unknown') })),
      h('dt', { text: t('gui.host.family') }), h('dd', { class: 'mono', text: hs.family || t('gui.host.unknown') })),
    h('h3', { class: 'sub', text: t('gui.dash.benchmarks') }),
    d.benchmarks.length ? h('div', { class: 'bench' }, d.benchmarks.slice(0, 5).map((b) => h('a', { href: '#/scan?target=' + encodeURIComponent(b.id) },
      picon(b.id, { tile: true, cat: b.category }), h('span', { class: 'grow' }, h('b', { text: b.name }),
        h('span', { class: 'meta', text: b.rules ? t('gui.dash.bench_meta', { rules: num(b.rules), auto: num(b.auto) }) : t('gui.dash.bench_none') })),
      icon('chev')))) : h('p', { class: 'muted', text: t('gui.host.none') }),
    d.benchmarks.length > 5 ? h('p', {}, h('a', { href: '#/platforms?detected=1', text: nn('gui.dash.more_benchmarks', d.benchmarks.length - 5) })) : null,
    d.catalog ? h('p', { class: 'hint', text: t('gui.dash.catalog', { rules: num(d.catalog.rules), targets: num(d.catalog.targets) }) }) : null,
    d.elevated ? notice('good', 'shield', h('p', { text: t('gui.admin.ok') })) : elevationNotice('gui.admin.missing_title', 'gui.admin.missing_body'));

  const cards = [hero, detected];
  if (last && d.severity) {
    const fails = {}; let nf = 0;
    for (const k of SEV_ALL) { fails[k] = d.severity[k].fail; nf += fails[k]; }
    cards.push(h('section', { class: 'card s5', 'aria-labelledby': 'h-sev' },
      h('h2', { class: 'sec', id: 'h-sev' }, icon('alert'), t('gui.results.by_severity')),
      nf ? donut(fails, t('gui.dash.donut_aria', { n: num(nf) })) : notice('good', 'check', h('p', { text: t('gui.results.no_failures') })),
      h('div', { class: 'sevbars' }, SEV_ALL.filter((k) => d.severity[k].pass + d.severity[k].fail).map((k) => {
        const r = d.severity[k], tot = r.pass + r.fail;
        const p = h('i', { class: 'p' }), f = h('i', { class: 'f' });
        setWidth(p, 100 * r.pass / tot); setWidth(f, 100 * r.fail / tot);
        return h('div', { class: 'sevbar' }, sevPill(k), h('div', { class: 'track', 'aria-hidden': 'true' }, p, f),
          h('span', { class: 'v', text: t('gui.dash.pass_fail_short', { passed: num(r.pass), failed: num(r.fail) }) }));
      }))));
    cards.push(h('section', { class: 'card s7', 'aria-labelledby': 'h-top' },
      h('div', { class: 'card-head' }, h('h2', { class: 'sec', id: 'h-top' }, icon('layers'), t('gui.dash.top_failing')),
        h('a', { class: 'act btn sm', href: '#/results/' + last.id + '?status=FAIL' }, t('gui.dash.see_all'))),
      d.top_failing.length ? h('ol', { class: 'toplist' }, d.top_failing.map((c) => {
        const bar = h('i'); setWidth(bar, 100 * c.failed / c.total);
        return h('li', {}, h('a', { href: '#/results/' + last.id + '?status=FAIL&target=' + encodeURIComponent(c.id) }, picon(c.id, { tile: true, cat: c.category }),
          h('span', { class: 'grow' }, h('b', { text: c.name }), h('span', { class: 'bar', 'aria-hidden': 'true' }, bar)),
          h('span', { class: 'v', text: t('gui.dash.failed_of', { failed: num(c.failed), total: num(c.total) }) })));
      })) : notice('good', 'check', h('p', { text: t('gui.results.no_failures') }))));
  }
  if (d.trend.length >= 2) {
    cards.push(h('section', { class: 'card s7', 'aria-labelledby': 'h-trend' },
      h('div', { class: 'card-head' }, h('h2', { class: 'sec', id: 'h-trend' }, icon('clock'), t('gui.history.trend')),
        h('a', { class: 'act btn sm', href: '#/history' }, t('gui.dash.see_history'))),
      trendChart(d.trend, 120),
      h('p', { class: 'hint', text: nn('gui.dash.trend_summary', d.trend.length, { first: pct(d.trend[0].score), last: pct(d.trend[d.trend.length - 1].score) }) })));
  }
  const qa = (href, ico, title, desc) => h('a', { href }, icon(ico, 'lg'), h('b', { text: title }), h('span', { text: desc }));
  cards.push(h('section', { class: 'card ' + (d.trend.length >= 2 ? 's5' : 's12'), 'aria-labelledby': 'h-quick' },
    h('h2', { class: 'sec', id: 'h-quick' }, icon('zap'), t('gui.dash.quick')),
    h('div', { class: 'quick' },
      qa('#/scan', 'radar', t('gui.nav.new_scan'), t('gui.dash.q_scan')),
      qa('#/platforms', 'layers', t('gui.nav.platforms'), t('gui.dash.q_platforms')),
      qa('#/history', 'compare', t('gui.nav.history'), t('gui.dash.q_history')),
      qa('#/settings', 'gear', t('gui.nav.settings'), t('gui.dash.q_settings')))));
  put(root, h('div', { class: 'grid' }, cards));
};
function kpi(cls, ico, label, n) {
  return h('div', { class: 'kpi k-' + cls }, h('span', { class: 'l' }, icon(ico), label), h('span', { class: 'n', text: num(n || 0) }));
}

// ---------- New scan wizard ----------
views.scan = async (root, jobId, query) => {
  if (jobId) return scanProgress(root, jobId);
  const running = (await api('/api/jobs')).jobs.find((j) => j.kind === 'scan' && j.state === 'running');
  if (running) { location.replace('#/scan/' + running.id); return; }
  return scanWizard(root, query);
};

async function scanWizard(root, query) {
  const [dirsR, hostR] = await Promise.all([api('/api/rulesdirs'), api('/api/host')]);
  const dirs = dirsR.dirs;
  const st = {
    step: 1, mode: query.target ? 'pick' : 'auto', targets: new Set((query.target || '').split(',').filter(Boolean)),
    level: query.level === '2' ? 2 : 1, manual: query.manual === '1', dir: (dirs.find((x) => x.current) || dirs[0] || {}).path || '',
    custom: '', osFamily: '', device: null, cat: 'all', q: '', detectedOnly: false,
  };
  let plats = null, platsErr = null, samples = null;
  const platsFor = async () => {
    const dir = rulesDir();
    if (plats && plats.rules_dir === dir) return plats;
    try { plats = await api('/api/platforms?rules_dir=' + encodeURIComponent(dir)); platsErr = null; } catch (e) { plats = null; platsErr = e; }
    return plats;
  };
  const rulesDir = () => (st.dir === '__custom__' ? st.custom.trim() : st.dir);
  const byId = () => { const m = {}; if (plats) for (const c of plats.categories) for (const g of c.groups) for (const x of g.targets) m[x.id] = x; return m; };
  const deviceCapable = () => { const m = byId(); return st.mode === 'pick' && [...st.targets].some((id) => m[id] && m[id].device > 0); };
  const params = () => ({
    scope: st.mode === 'pick' ? 'target' : st.mode, target: st.mode === 'pick' ? [...st.targets].join(',') : '', level: st.level,
    include_manual: st.manual, os_family: st.osFamily.trim(), rules_dir: rulesDir(),
    device_config: st.device && deviceCapable() ? st.device.id : '', device_name: st.device && deviceCapable() ? st.device.name : '',
  });
  await platsFor();

  put(root, pageHead('gui.wiz.title', t('gui.wiz.lede'), null, 'gui.wiz.eyebrow'));
  const steps = [['gui.wiz.step1', 'gui.wiz.step1_desc'], ['gui.wiz.step2', 'gui.wiz.step2_desc'], ['gui.wiz.step3', 'gui.wiz.step3_desc']];
  const stepper = h('ol', { class: 'stepper', 'aria-label': t('gui.wiz.steps') });
  const panel = h('div', { class: 'card' });
  const foot = h('div', { class: 'wiz-foot' });
  put(root, stepper, panel, foot);

  const canNext = () => st.step !== 1 || st.mode !== 'pick' || st.targets.size > 0;
  function renderStepper() {
    stepper.replaceChildren(...steps.map(([k, dk], i) => {
      const n = i + 1;
      const b = h('button', { type: 'button', class: n < st.step ? 'done' : '', 'aria-current': n === st.step ? 'step' : null, disabled: n > st.step && !(n === st.step + 1 && canNext()) ? true : null },
        h('span', { class: 'n', 'aria-hidden': 'true' }, n < st.step ? icon('check') : num(n)),
        h('span', {}, h('b', { text: t(k) }), h('small', { text: t(dk) })));
      b.addEventListener('click', () => go(n));
      return h('li', {}, b);
    }));
  }
  function go(n) {
    if (n > st.step && !canNext()) return;
    st.step = n;
    render();
    const hd = $('h2', panel);
    if (hd) { hd.tabIndex = -1; hd.focus(); }
  }
  function render() {
    renderStepper();
    panel.replaceChildren();
    if (st.step === 1) stepWhat();
    else if (st.step === 2) stepOptions();
    else stepReview();
    renderFoot();
  }
  function renderFoot() {
    const back = h('button', { class: 'btn', type: 'button', onclick: () => go(st.step - 1) }, icon('chev-l'), t('gui.wiz.back'));
    const next = h('button', { class: 'btn primary', type: 'button', disabled: canNext() ? null : true, onclick: () => go(st.step + 1) }, t('gui.wiz.next'), icon('chev'));
    foot.replaceChildren(st.step > 1 ? back : null, h('span', { class: 'sp' }),
      st.step === 1 && st.mode === 'pick' ? h('span', { class: 'hint', 'aria-live': 'polite', text: nn('gui.wiz.selected', st.targets.size) }) : null,
      st.step < 3 ? next : null);
  }

  // step 1: what to scan
  function stepWhat() {
    const radio = (value, ico, title, desc, extra) => h('label', { class: 'choice' },
      h('input', { type: 'radio', name: 'mode', value, checked: st.mode === value ? true : null, onchange: () => { st.mode = value; render(); } }),
      icon(ico, 'lg'), h('div', {}, h('b', { text: title }), h('span', { text: desc }), extra || null));
    const det = h('span', {}, h('span', { class: 'chips' }, hostR.targets.map((x) => h('span', { class: 'tag plat' }, picon(x), h('span', { text: platName(x) })))));
    put(panel, h('h2', { class: 'sec', text: t('gui.wiz.what') }),
      h('fieldset', {}, h('legend', { class: 'sr', text: t('gui.scan.scope') }), h('div', { class: 'choices' },
        radio('auto', 'radar', t('gui.wiz.mode_auto'), t('gui.scan.scope_auto_desc'), det),
        radio('pick', 'layers', t('gui.wiz.mode_pick'), t('gui.wiz.mode_pick_desc')),
        radio('dir', 'list', t('gui.scan.scope_dir'), t('gui.scan.scope_dir_desc')))));
    if (st.mode === 'pick') put(panel, picker());
  }
  function picker() {
    const wrap = h('div', { class: 'picker' });
    if (!plats) { wrap.append(errorState(platsErr, async () => { await platsFor(); render(); })); return wrap; }
    const selRow = h('div', { class: 'row' });
    const body = h('div', { role: 'tabpanel' });
    const search = h('input', { class: 'input', type: 'search', 'data-search': '', placeholder: t('gui.wiz.search'), 'aria-label': t('gui.wiz.search'), value: st.q });
    search.addEventListener('input', () => { st.q = search.value.trim().toLowerCase(); drawBody(); });
    const counts = {};
    for (const c of plats.categories) counts[c.id] = c.targets;
    const tabs = [{ id: 'all', label: t('gui.cat.all'), icon: icon('c-all'), count: plats.targets }, ...plats.categories.map((c) => ({ id: c.id, label: t('gui.cat.' + c.id), icon: catIcon(c.id), count: c.targets }))];
    const tl = tablist(t('gui.wiz.categories'), tabs, st.cat, (id) => { st.cat = id; drawBody(); });
    body.setAttribute('aria-labelledby', ($('[aria-selected=true]', tl) || {}).id || '');
    function drawSel() {
      const m = byId();
      selRow.replaceChildren(h('b', { text: nn('gui.wiz.selected', st.targets.size) }),
        ...[...st.targets].map((id) => h('button', { class: 'chip', type: 'button', 'aria-label': t('gui.wiz.remove', { name: (m[id] || {}).name || id }), onclick: () => { st.targets.delete(id); drawBody(); drawSel(); renderFoot(); renderStepper(); } }, picon(id, { cat: (m[id] || {}).category, active: true }), (m[id] || {}).name || id, icon('x'))),
        st.targets.size ? h('button', { class: 'btn ghost sm', type: 'button', onclick: () => { st.targets.clear(); drawBody(); drawSel(); renderFoot(); renderStepper(); } }, t('gui.wiz.clear')) : null);
    }
    function drawBody() {
      body.setAttribute('aria-labelledby', ($('[aria-selected=true]', tl) || {}).id || '');
      const cats = plats.categories.filter((c) => st.cat === 'all' || c.id === st.cat);
      const out = [];
      for (const c of cats) {
        const groups = c.groups.map((g) => ({ id: g.id, targets: g.targets.filter((x) => (!st.q || x.name.toLowerCase().includes(st.q) || x.id.includes(st.q)) && (!st.detectedOnly || x.detected)) })).filter((g) => g.targets.length);
        if (!groups.length) continue;
        if (st.cat === 'all') out.push(h('h3', { class: 'sub row' }, glyph(c.id, true), t('gui.cat.' + c.id)));
        for (const g of groups) {
          out.push(h('section', { class: 'pgroup' }, h('h3', { text: t('gui.group.' + g.id) }),
            h('div', { class: 'tgrid' }, g.targets.map((x) => {
              const on = st.targets.has(x.id);
              const b = h('button', { class: 'tcard', type: 'button', 'aria-pressed': String(on) },
                h('span', { class: 'tick', 'aria-hidden': 'true' }, icon('check')), picon(x.id, { tile: true, size: 'md', cat: x.category }),
                h('span', { class: 'grow' }, h('span', { class: 'nm', text: x.name }), h('span', { class: 'id mono', text: x.id }),
                  h('span', { class: 'bd' }, h('span', { class: 'pill neutral', text: nn('gui.scan.target_count', x.total) }),
                    x.detected ? h('span', { class: 'pill ok' }, icon('check'), t('gui.plat.detected')) : null,
                    x.device ? h('span', { class: 'pill info' }, icon('file'), t('gui.plat.device')) : null,
                    !x.auto && !x.device ? h('span', { class: 'pill warn', text: t('gui.plat.manual_only') }) : null)));
              b.addEventListener('click', () => {
                if (st.targets.has(x.id)) st.targets.delete(x.id); else st.targets.add(x.id);
                b.setAttribute('aria-pressed', String(st.targets.has(x.id)));
                drawSel(); renderFoot(); renderStepper();
              });
              return b;
            }))));
        }
      }
      body.replaceChildren(...(out.length ? out : [h('p', { class: 'muted empty', text: t('gui.wiz.no_match') })]));
    }
    drawSel(); drawBody();
    wrap.append(h('div', { class: 'toolbar' }, h('div', { class: 'search' }, icon('scan'), search),
      toggleChip(t('gui.plat.detected_only'), st.detectedOnly, (v) => { st.detectedOnly = v; drawBody(); })), tl, selRow, body);
    return wrap;
  }

  // step 2: options
  function stepOptions() {
    const lv = (v, title, desc) => h('label', { class: 'choice' }, h('input', { type: 'radio', name: 'level', value: String(v), checked: st.level === v ? true : null, onchange: () => { st.level = v; } }),
      h('div', {}, h('b', { text: title }), h('span', { text: desc })));
    const manual = h('input', { type: 'checkbox', id: 'f-manual', checked: st.manual ? true : null, onchange: (e) => { st.manual = e.target.checked; } });
    const custom = h('input', { class: 'input mono', id: 'f-custom', value: st.custom, placeholder: t('gui.scan.dir_custom_placeholder'), spellcheck: 'false', oninput: (e) => { st.custom = e.target.value; } });
    const customField = h('div', { class: 'field' + (st.dir === '__custom__' ? '' : ' hidden') }, h('label', { for: 'f-custom', text: t('gui.scan.dir_custom') }), custom);
    const dirSel = h('select', { class: 'input', id: 'f-dir' },
      dirs.map((d) => h('option', { value: d.path, disabled: d.exists ? null : true, selected: d.path === st.dir ? true : null },
        t(!d.exists ? 'gui.scan.dir_missing' : d.kind === 'staging' ? 'gui.scan.dir_staging' : 'gui.scan.dir_option', { path: d.path }))),
      h('option', { value: '__custom__', selected: st.dir === '__custom__' ? true : null, text: t('gui.scan.dir_custom') }));
    dirSel.addEventListener('change', async () => { st.dir = dirSel.value; customField.classList.toggle('hidden', st.dir !== '__custom__'); if (st.dir !== '__custom__') await platsFor(); });
    const osFam = h('input', { class: 'input mono', id: 'f-os', value: st.osFamily, placeholder: 'windows-11, ubuntu-24.04', spellcheck: 'false', oninput: (e) => { st.osFamily = e.target.value; } });
    put(panel, h('h2', { class: 'sec', text: t('gui.wiz.options') }),
      h('fieldset', {}, h('legend', { class: 'lbl', text: t('gui.scan.level') }), h('div', { class: 'choices' },
        lv(1, t('gui.scan.level1'), t('gui.scan.level1_desc')), lv(2, t('gui.scan.level2'), t('gui.scan.level2_desc')))),
      h('div', { class: 'field' }, h('label', { class: 'choice' }, manual, h('div', {}, h('b', { text: t('gui.scan.manual') }), h('span', { text: t('gui.scan.manual_desc') })))),
      deviceCapable() ? deviceSection() : null,
      h('div', { class: 'field' }, h('label', { for: 'f-dir', text: t('gui.scan.rules_dir') }), dirSel, h('p', { class: 'hint', text: t('gui.scan.rules_dir_hint') })),
      customField,
      h('details', { class: 'field' }, h('summary', { text: t('gui.scan.advanced') }),
        h('div', { class: 'field' }, h('label', { for: 'f-os', text: t('gui.scan.os_family') }), osFam, h('p', { class: 'hint', text: t('gui.scan.os_family_hint') }))));
  }
  function deviceSection() {
    const box = h('section', { class: 'field', 'aria-labelledby': 'h-dev' });
    const m = byId();
    const devTargets = [...st.targets].filter((id) => m[id] && m[id].device > 0);
    const draw = () => {
      const input = h('input', { type: 'file', id: 'f-devfile', class: 'sr', accept: '.txt,.cfg,.conf,.xml,.json,.log,text/plain' });
      input.addEventListener('change', () => { if (input.files[0]) upload(input.files[0]); });
      const drop = h('div', { class: 'drop' }, icon('upload', 'xl'), h('p', {}, h('b', { text: t('gui.dev.drop') })), h('p', { class: 'hint', text: t('gui.dev.drop_hint') }),
        h('label', { class: 'btn', for: 'f-devfile' }, icon('file'), t('gui.dev.choose')), input);
      for (const ev of ['dragenter', 'dragover']) drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.add('over'); });
      for (const ev of ['dragleave', 'drop']) drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.remove('over'); });
      drop.addEventListener('drop', (e) => { const f = e.dataTransfer && e.dataTransfer.files[0]; if (f) upload(f); });
      const chip = st.device ? h('div', { class: 'filechip' }, icon('file', 'lg'), h('div', { class: 'grow' }, h('b', { text: st.device.name }),
        h('div', { class: 'hint', text: t('gui.dev.file_meta', { size: bytes(st.device.size), lines: num(st.device.lines) }) })),
      h('button', { class: 'btn sm', type: 'button', onclick: () => { st.device = null; draw(); } }, icon('x'), t('gui.dev.remove'))) : null;
      const sampleBox = h('div', {});
      const cli = 'tmaegis scan --target ' + (devTargets[0] || '<id>') + ' --device-config <file>';
      box.replaceChildren(h('h3', { class: 'lbl', id: 'h-dev' }, icon('c-network'), ' ', t('gui.dev.title')),
        h('p', { class: 'hint', text: nn('gui.dev.body', devTargets.length, { names: devTargets.map((id) => m[id].name).join(', ') }) }),
        chip || drop, h('p', { class: 'hint', text: t('gui.dev.only_device') }), sampleBox,
        h('p', { class: 'hint', text: t('gui.dev.cli') }), cmdBox(cli, t('gui.dev.cli_label')));
      showSamples(sampleBox, devTargets, draw);
    };
    async function upload(f) {
      if (f.size > 8 * 1048576) { toast(t('gui.err.too_large'), 'err'); return; }
      try {
        const text = await f.text();
        st.device = await api('/api/device-config', { name: f.name, content: text });
        toast(t('gui.dev.uploaded', { name: st.device.name }));
        draw();
      } catch (e) { toast(errText(e), 'err'); }
    }
    draw();
    return box;
  }
  async function showSamples(box, devTargets, redraw) {
    if (!samples) { try { samples = (await api('/api/samples')).samples; } catch (e) { samples = []; } }
    const have = samples.filter((x) => devTargets.includes(x.target));
    const generic = h('p', { class: 'hint' }, icon('info'), ' ', t('gui.dev.sample_hint'));
    if (!have.length) { box.replaceChildren(generic); return; }
    box.replaceChildren(notice('info', 'info', h('p', { text: t('gui.dev.sample_found') }),
      h('div', { class: 'chips' }, have.flatMap((x) => ['noncompliant', 'compliant'].map((v) => h('button', { class: 'chip', type: 'button', onclick: async () => {
        try { st.device = await api('/api/device-config', { sample: x.target, variant: v }); toast(t('gui.dev.uploaded', { name: st.device.name })); redraw(); } catch (e) { toast(errText(e), 'err'); }
      } }, icon('file'), t('gui.dev.sample_' + v, { target: x.target })))))));
  }

  // step 3: review (read-only) and start
  async function stepReview() {
    put(panel, h('h2', { class: 'sec', text: t('gui.wiz.review') }), h('div', { class: 'sk sk-block' }));
    const p = params();
    let plan;
    try { plan = await api('/api/scan/plan', p); } catch (e) { panel.replaceChildren(h('h2', { class: 'sec', text: t('gui.wiz.review') }), errorState(e, () => render())); return; }
    if (st.step !== 3) return;
    const err = h('div', { 'aria-live': 'polite' });
    const start = h('button', { class: 'btn primary lg', type: 'button', disabled: plan.total ? null : true }, icon('play'), t('gui.scan.start'));
    start.addEventListener('click', async () => {
      start.disabled = true; err.replaceChildren();
      try { const j = await api('/api/scan', p); location.hash = '#/scan/' + j.id; }
      catch (ex) { err.replaceChildren(notice('err', 'alert', h('p', { text: errText(ex) }))); start.disabled = false; }
    });
    const scopeText = st.mode === 'auto' ? t('gui.wiz.mode_auto') : st.mode === 'dir' ? t('gui.scan.scope_dir') : nn('gui.wiz.selected', st.targets.size);
    const sevMax = Math.max(1, ...SEV_ALL.map((k) => plan.severity[k] || 0));
    const cmds = (st.mode === 'pick' ? [...st.targets] : [null]).map((tg) => ['tmaegis scan',
      st.mode === 'auto' ? '--auto-detect' : tg ? '--target ' + tg : '', '--level ' + st.level, st.manual ? '--include-manual' : '',
      p.device_config ? '--device-config "' + p.device_name + '"' : '', p.rules_dir && p.rules_dir !== S.default_rules_dir ? '--rules-dir "' + p.rules_dir + '"' : '',
      p.os_family ? '--os-family ' + p.os_family : ''].filter(Boolean).join(' '));
    panel.replaceChildren(h('h2', { class: 'sec', text: t('gui.wiz.review') }),
      plan.total ? h('p', { class: 'verdict', text: nn('gui.wiz.will_run', plan.total) }) : notice('warn', 'alert', h('p', { text: t('gui.err.no_rules') })),
      h('div', { class: 'summary-grid' },
        kpi('pass', 'radar', t('gui.wiz.auto_rules'), plan.auto), kpi('skip', 'eye', t('gui.wiz.manual_rules'), plan.manual),
        kpi('total', 'file', t('gui.wiz.device_rules'), plan.device), kpi('total', 'fix', t('gui.wiz.fixable_rules'), plan.fixable)),
      plan.device_available && !p.device_config ? notice('info', 'info', h('p', { text: nn('gui.wiz.device_available', plan.device_available) })) : null,
      h('div', { class: 'grid' },
        h('div', { class: 's6' }, h('h3', { class: 'sub', text: t('gui.wiz.settings') }),
          h('dl', { class: 'kv' },
            h('dt', { text: t('gui.scan.scope') }), h('dd', { text: scopeText }),
            h('dt', { text: t('gui.scan.level') }), h('dd', { text: t(st.level === 2 ? 'gui.scan.level2' : 'gui.scan.level1') }),
            h('dt', { text: t('gui.scan.manual') }), h('dd', { text: t(st.manual ? 'gui.wiz.yes' : 'gui.wiz.no') }),
            p.device_config ? [h('dt', { text: t('gui.dev.title') }), h('dd', { class: 'mono', text: p.device_name })] : null,
            h('dt', { text: t('gui.scan.rules_dir') }), h('dd', { class: 'mono', text: plan.rules_dir }),
            p.os_family ? [h('dt', { text: t('gui.scan.os_family') }), h('dd', { class: 'mono', text: p.os_family })] : null),
          h('h3', { class: 'sub', text: t('gui.wiz.by_severity') }),
          h('div', { class: 'sevbars' }, SEV_ALL.filter((k) => plan.severity[k]).map((k) => {
            const f = h('i', { class: 'p' }); setWidth(f, 100 * plan.severity[k] / sevMax);
            return h('div', { class: 'sevbar' }, sevPill(k), h('div', { class: 'track', 'aria-hidden': 'true' }, f), h('span', { class: 'v', text: num(plan.severity[k]) }));
          }))),
        h('div', { class: 's6' }, h('h3', { class: 'sub', text: t('gui.wiz.targets') }),
          h('ul', { class: 'toplist' }, plan.targets.slice(0, 10).map((x) => h('li', { class: 'row' }, picon(x.id, { tile: true, cat: x.category }), h('span', { class: 'grow', text: x.name }), h('span', { class: 'pill neutral', text: nn('gui.scan.target_count', x.count) })))),
          plan.targets.length > 10 ? h('p', { class: 'hint', text: nn('gui.wiz.more_targets', plan.targets.length - 10) }) : null)),
      h('h3', { class: 'sub', text: t('gui.wiz.cli') }), cmdBox(cmds.join('\n'), t('gui.wiz.cli')),
      notice('good', 'lock', h('p', {}, h('b', { text: t('gui.scan.safe_title') }), ' ', t('gui.scan.safe_body'))),
      err, h('div', { class: 'actions' }, start));
  }
  render();
}

async function scanProgress(root, id) {
  let job;
  try { job = await api('/api/jobs/' + encodeURIComponent(id)); } catch (e) { location.replace('#/scan'); return; }
  const bar = h('i'), prog = h('div', { class: 'progress', role: 'progressbar', 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-label': t('gui.scan.progress_label') }, bar);
  const big = h('b', { class: 'verdict' });
  const phaseEl = h('p', { class: 'muted' });
  const cur = h('div', { class: 'cur', 'aria-hidden': 'true' });
  const statsEl = h('div', { class: 'stats-row' });
  const timeEl = h('div', { class: 'stats-row muted' });
  const result = h('div', { 'aria-live': 'polite' });
  const cancel = h('button', { class: 'btn danger', type: 'button' }, icon('stop'), t('gui.scan.cancel'));
  cancel.addEventListener('click', async () => {
    cancel.disabled = true;
    try { await api('/api/jobs/' + id + '/cancel', {}); announce(t('gui.prog.cancelling')); } catch (e) { toast(errText(e), 'err'); cancel.disabled = false; }
  });
  const stream = h('ul', { class: 'stream', 'aria-label': t('gui.prog.stream'), tabindex: 0 });
  let filter = 'all', lastSeq = 0, items = [];
  const drawStream = () => {
    const shown = items.filter((x) => filter === 'all' || x.status === 'FAIL' || x.status === 'ERROR');
    stream.replaceChildren(...(shown.length ? shown.slice(0, 200).map((x) => streamRow(x)) : [h('li', { class: 'muted', text: t('gui.prog.stream_empty') })]));
  };
  const streamRow = (x, isNew) => h('li', { class: isNew ? 'new' : '' }, statusPill(x.status, true), h('span', { class: 'rid', text: x.id }), h('span', { class: 'tt', text: x.title }), sevPill(x.severity));
  put(root, pageHead('gui.scan.running', null, null, 'gui.wiz.eyebrow'),
    h('section', { class: 'card', 'aria-labelledby': 'h-prog' },
      h('div', { class: 'prog-hero' }, h('div', { class: 'scanpulse', 'aria-hidden': 'true' }, h('img', { src: '/static/logo-mark.svg', alt: '' })),
        h('div', { class: 'grow' }, h('h2', { class: 'sec', id: 'h-prog', text: t('gui.prog.title') }), big, phaseEl)),
      prog, cur, statsEl, timeEl, h('div', { class: 'actions' }, cancel), result),
    h('section', { class: 'card', 'aria-labelledby': 'h-stream' }, h('div', { class: 'card-head' }, h('h2', { class: 'sec', id: 'h-stream' }, icon('list'), t('gui.prog.stream')),
      h('div', { class: 'act' }, seg(t('gui.prog.stream_filter'), [['all', t('gui.table.all')], ['fail', t('gui.prog.failed_only')]], 'all', (v) => { filter = v; drawStream(); }))), stream));
  drawStream();

  let es, timer, ended = false, lastPct = -1;
  const started = new Date(job.started).getTime();
  function render(v) {
    const el = Math.max(0, (Date.now() - started) / 1000);
    if (v.phase === 'loading' || !v.total) {
      prog.classList.add('indet'); prog.removeAttribute('aria-valuenow'); phaseEl.textContent = t('gui.scan.phase_loading'); big.textContent = t('gui.prog.preparing');
    } else {
      prog.classList.remove('indet');
      const p = Math.round(100 * v.done / v.total);
      setWidth(bar, p); prog.setAttribute('aria-valuenow', p);
      prog.setAttribute('aria-valuetext', t('gui.scan.progress_announce', { done: num(v.done), total: num(v.total) }));
      big.textContent = pct(p);
      phaseEl.textContent = v.state === 'running' ? t('gui.scan.rules_done', { done: num(v.done), total: num(v.total) }) : '';
      const step = Math.floor(p / 10);
      if (step !== lastPct && v.state === 'running') { lastPct = step; announce(t('gui.scan.progress_announce', { done: num(v.done), total: num(v.total) })); }
    }
    cur.textContent = v.current || '';
    statsEl.replaceChildren(statusPillN('PASS', v.passed), statusPillN('FAIL', v.failed), statusPillN('SKIP', v.skipped || 0));
    const rate = el > 1 ? v.done / el : 0;
    const eta = rate > 0 && v.done >= 3 && v.total > v.done ? (v.total - v.done) / rate : null;
    timeEl.replaceChildren(h('span', {}, icon('clock'), ' ', t('gui.prog.elapsed', { time: dur(el) })),
      eta != null ? h('span', { text: t('gui.prog.eta', { time: dur(eta) }) }) : null,
      rate > 0 ? h('span', { text: t('gui.prog.rate', { n: NF.format(Math.round(rate * 10) / 10) }) }) : null);
    const fresh = (v.log || []).filter((x) => x.seq > lastSeq);
    if (fresh.length) {
      lastSeq = fresh[fresh.length - 1].seq;
      items = [...fresh.reverse(), ...items].slice(0, 400);
      const shown = fresh.filter((x) => filter === 'all' || x.status === 'FAIL' || x.status === 'ERROR');
      if (stream.firstChild && stream.firstChild.classList.contains('muted')) stream.replaceChildren();
      stream.prepend(...shown.map((x) => streamRow(x, true)));
      while (stream.children.length > 200) stream.lastChild.remove();
    }
  }
  function finish(v) {
    if (ended) return; ended = true;
    if (es) es.close();
    clearInterval(timer);
    cancel.classList.add('hidden');
    prog.classList.remove('indet');
    if (v.state === 'done') {
      toast(t('gui.prog.done_toast', { passed: num(v.passed), failed: num(v.failed) }));
      location.hash = '#/results/' + v.report_id;
      return;
    }
    const msg = v.state === 'cancelled' ? t('gui.scan.cancelled') : t('gui.scan.failed', { error: hasKey('gui.err.' + v.error_code) ? t('gui.err.' + v.error_code) : (v.error || '') });
    result.replaceChildren(h('div', { class: 'outcome ' + (v.state === 'cancelled' ? 'warn' : 'err') }, h('div', { class: 'art', 'aria-hidden': 'true' }, icon(v.state === 'cancelled' ? 'stop' : 'alert', 'lg')),
      h('div', {}, h('h2', { text: v.state === 'cancelled' ? t('gui.prog.cancelled_title') : t('gui.prog.failed_title') }), h('p', { text: msg }))),
    h('div', { class: 'actions' }, h('a', { class: 'btn primary', href: '#/scan' }, icon('radar'), t('gui.scan.back'))));
    big.textContent = v.state === 'cancelled' ? t('gui.prog.cancelled_title') : t('gui.prog.failed_title');
    phaseEl.textContent = '';
    $('a.btn', result).focus();
  }
  render(job);
  if (job.state !== 'running') { finish(job); return; }
  es = new EventSource('/api/jobs/' + encodeURIComponent(id) + '/events');
  es.addEventListener('job', (ev) => { const v = JSON.parse(ev.data); render(v); if (v.state !== 'running') finish(v); });
  es.onerror = () => { if (!ended) setOffline(true); };
  timer = setInterval(() => { if (!ended) api('/api/jobs/' + id).then((v) => { render(v); if (v.state !== 'running') finish(v); }).catch(() => {}); }, 4000);
  return () => { ended = true; if (es) es.close(); clearInterval(timer); };
}
function statusPillN(st, n) { const p = statusPill(st); p.append(h('b', { text: num(n) })); return p; }

// ---------- Results ----------
const failing = (r) => r.status === 'FAIL' || r.status === 'ERROR';
const isManual = (r) => r.rule.audit && r.rule.audit.type === 'manual';
const canFix = (r) => failing(r) && r.rule.remediation && !!r.rule.remediation.type;
const STATUS_RANK = { FAIL: 0, ERROR: 0, SKIP: 1, PASS: 2 };
const ROW_H = 52;

views.results = async (root, id, query) => {
  if (!id) {
    const reps = (await api('/api/reports')).reports;
    if (!reps.length) {
      put(root, pageHead('gui.results.title', null), emptyState('list', t('gui.results.empty_title'), t('gui.results.empty_body'),
        h('a', { class: 'btn primary', href: '#/scan' }, icon('radar'), t('gui.results.go_scan'))));
      return;
    }
    location.replace('#/results/' + reps[0].id);
    return;
  }
  const [sr, all] = await Promise.all([api('/api/reports/' + encodeURIComponent(id)), api('/api/reports')]);
  const rep = sr.report, results = rep.results || [];
  const c = { PASS: 0, FAIL: 0, SKIP: 0, MANUAL: 0, FIX: 0 };
  for (const r of results) { if (failing(r)) c.FAIL++; else c[r.status] = (c[r.status] || 0) + 1; if (isManual(r)) c.MANUAL++; if (canFix(r)) c.FIX++; }
  const idx = all.reports.findIndex((r) => r.id === id);
  const older = idx >= 0 ? all.reports[idx + 1] : null;

  const picker = h('select', { class: 'input', 'aria-label': t('gui.results.pick'), onchange: (e) => { location.hash = '#/results/' + e.target.value; } },
    all.reports.map((r) => h('option', { value: r.id, selected: r.id === id ? true : null, text: t('gui.res.pick_option', { date: fmtDate(r.created), score: pct(r.score) }) })));
  put(root, pageHead('gui.results.title', t('gui.results.subtitle', { host: rep.hostname || '-', date: fmtDate(sr.created), os: rep.os + '/' + rep.arch }),
    [picker, exportMenu(id), h('a', { class: 'btn', href: '/api/report/' + id + '.html?lang=' + encodeURIComponent(LANG), target: '_blank', rel: 'noopener' }, icon('ext'), t('gui.results.open_html'))]));

  put(root, h('section', { class: 'card', 'aria-label': t('gui.results.summary') }, h('div', { class: 'res-head' }, gauge(sr.score, true),
    h('div', { class: 'meta' }, h('p', { class: 'verdict ' + scoreClass(sr.score), text: t('gui.verdict.' + scoreClass(sr.score)) }),
      h('div', { class: 'stats-row' }, statusPillN('PASS', c.PASS), statusPillN('FAIL', c.FAIL), statusPillN('SKIP', c.SKIP),
        h('span', { class: 'pill neutral' }, icon('eye'), nn('gui.res.manual_count', c.MANUAL))),
      h('p', { class: 'hint', text: t('gui.results.score_note') })),
    h('div', { class: 'row' },
      older ? h('a', { class: 'btn', href: '#/history/compare?a=' + older.id + '&b=' + id }, icon('compare'), t('gui.res.compare_prev')) : null,
      c.FIX ? h('a', { class: 'btn primary', href: '#/fix/' + id }, icon('fix'), nn('gui.results.fix', c.FIX)) : null))));

  // filters
  const targets = {};
  for (const r of results) targets[r.rule.target] = (targets[r.rule.target] || 0) + 1;
  const st = { q: (query.q || '').toLowerCase(), status: query.status || '', sevs: new Set(), target: query.target || '', fixable: false, sort: 'status', dir: 1 };
  const sel = new Set();
  const search = h('input', { class: 'input', type: 'search', 'data-search': '', 'aria-label': t('gui.table.search'), placeholder: t('gui.table.search'), value: query.q || '' });
  search.addEventListener('input', () => { st.q = search.value.toLowerCase(); refilter(); });
  const statusSeg = seg(t('gui.table.status'), [['', t('gui.table.all')], ['FAIL', t('gui.status.fail'), 'x'], ['PASS', t('gui.status.pass'), 'check'], ['SKIP', t('gui.status.skip'), 'skip'], ['MANUAL', t('gui.res.manual'), 'eye']], st.status, (v) => { st.status = v; refilter(); });
  const sevChips = h('div', { class: 'chips', role: 'group', 'aria-label': t('gui.table.severity') }, SEV_ALL.map((k) => toggleChip(t('gui.sev.' + k), false, (on) => { if (on) st.sevs.add(k); else st.sevs.delete(k); refilter(); })));
  const tSel = h('select', { class: 'input', 'aria-label': t('gui.res.benchmark') }, h('option', { value: '', text: t('gui.res.all_benchmarks') }),
    Object.keys(targets).sort().map((x) => h('option', { value: x, selected: x === st.target ? true : null, text: t('gui.res.bench_option', { name: platName(x), count: num(targets[x]) }) })));
  tSel.addEventListener('change', () => { st.target = tSel.value; refilter(); });
  const fixChip = toggleChip(t('gui.res.fixable_only'), false, (on) => { st.fixable = on; refilter(); });
  const countEl = h('span', { class: 'count', 'aria-live': 'polite' });
  put(root, h('div', { class: 'toolbar' }, h('div', { class: 'search' }, icon('scan'), search), statusSeg),
    h('div', { class: 'toolbar' }, sevChips, tSel, fixChip, countEl));

  // virtualized table
  const cols = [['sel', ''], ['status', 'gui.table.col_status'], ['sev', 'gui.table.col_severity'], ['id', 'gui.table.col_id'], ['title', 'gui.table.col_title'], ['level', 'gui.table.col_level']];
  const cls = { sel: 'c-sel', status: 'c-st', sev: 'c-sev', id: 'c-id', title: 'c-title', level: 'c-lv' };
  const tbody = h('tbody');
  const headSel = h('input', { type: 'checkbox', 'aria-label': t('gui.res.select_all_fixable') });
  headSel.addEventListener('change', () => { for (const r of rows) if (canFix(r)) { if (headSel.checked) sel.add(r.rule.id); else sel.delete(r.rule.id); } drawRows(true); drawBulk(); });
  const ths = cols.map(([k, key]) => {
    if (k === 'sel') return h('th', { scope: 'col', class: cls[k] }, headSel);
    const b = h('button', { type: 'button' }, t(key), icon('sort', 'sort'));
    const th = h('th', { scope: 'col', class: cls[k] }, b);
    b.addEventListener('click', () => { if (st.sort === k) st.dir = -st.dir; else { st.sort = k; st.dir = 1; } refilter(); });
    return th;
  });
  const table = h('table', { class: 'tbl', 'aria-rowcount': '0' }, h('caption', { class: 'sr', text: t('gui.table.caption') }),
    h('colgroup', {}, cols.map(([k]) => h('col', { class: cls[k] }))), h('thead', {}, h('tr', { 'aria-rowindex': '1' }, ths)), tbody);
  const vt = h('div', { class: 'vt', tabindex: '-1' }, table);
  const bulk = h('div', { class: 'bulk hidden', role: 'region', 'aria-label': t('gui.res.bulk') });
  put(root, vt, bulk);

  let rows = [], active = -1;
  function match(r) {
    if (st.status === 'FAIL' && !failing(r)) return false;
    if (st.status === 'MANUAL' && !isManual(r)) return false;
    if ((st.status === 'PASS' || st.status === 'SKIP') && r.status !== st.status) return false;
    if (st.sevs.size && !st.sevs.has(sevKey(r.rule.severity))) return false;
    if (st.target && r.rule.target !== st.target) return false;
    if (st.fixable && !canFix(r)) return false;
    if (st.q && !(r.rule.id + ' ' + r.rule.title + ' ' + r.rule.target + ' ' + (r.rule.reference || '')).toLowerCase().includes(st.q)) return false;
    return true;
  }
  const cmpId = (a, b) => a.rule.id.localeCompare(b.rule.id, undefined, { numeric: true });
  const sorters = {
    status: (a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status] || sevRank(a.rule.severity) - sevRank(b.rule.severity) || cmpId(a, b),
    sev: (a, b) => sevRank(a.rule.severity) - sevRank(b.rule.severity) || STATUS_RANK[a.status] - STATUS_RANK[b.status] || cmpId(a, b),
    id: cmpId, title: (a, b) => a.rule.title.localeCompare(b.rule.title) || cmpId(a, b), level: (a, b) => a.rule.level - b.rule.level || cmpId(a, b),
  };
  function refilter() {
    rows = results.filter(match).sort((a, b) => st.dir * sorters[st.sort](a, b));
    active = -1;
    ths.forEach((th, i) => { const k = cols[i][0]; if (k === st.sort) th.setAttribute('aria-sort', st.dir > 0 ? 'ascending' : 'descending'); else th.removeAttribute('aria-sort'); });
    table.setAttribute('aria-rowcount', String(rows.length + 1));
    countEl.textContent = nn('gui.table.count', rows.length, { total: num(results.length) });
    vt.scrollTop = 0;
    drawRows(true);
  }
  let lastWin = '';
  function drawRows(force) {
    const viewH = vt.clientHeight || 600;
    const first = Math.max(0, Math.floor(vt.scrollTop / ROW_H) - 8);
    const last = Math.min(rows.length, first + Math.ceil(viewH / ROW_H) + 16);
    const key = first + ':' + last + ':' + rows.length;
    if (!force && key === lastWin) return;
    lastWin = key;
    const top = h('td', { colspan: cols.length }), bot = h('td', { colspan: cols.length });
    top.style.height = (first * ROW_H) + 'px'; bot.style.height = ((rows.length - last) * ROW_H) + 'px';
    const out = [h('tr', { class: 'spacer', 'aria-hidden': 'true' }, top)];
    for (let i = first; i < last; i++) out.push(rowEl(rows[i], i));
    out.push(h('tr', { class: 'spacer', 'aria-hidden': 'true' }, bot));
    if (!rows.length) out.splice(1, 0, h('tr', {}, h('td', { colspan: cols.length, class: 'none' }, t('gui.table.none'), ' ',
      h('button', { class: 'btn sm', type: 'button', onclick: resetFilters }, t('gui.res.reset')))));
    tbody.replaceChildren(...out);
    const fixable = rows.filter(canFix);
    headSel.checked = fixable.length > 0 && fixable.every((r) => sel.has(r.rule.id));
    headSel.indeterminate = !headSel.checked && fixable.some((r) => sel.has(r.rule.id));
    headSel.disabled = !fixable.length;
  }
  function rowEl(r, i) {
    const fx = canFix(r);
    const cb = fx ? h('input', { type: 'checkbox', checked: sel.has(r.rule.id) ? true : null, 'aria-label': t('gui.fix.select_rule', { id: r.rule.id }) }) : null;
    if (cb) cb.addEventListener('change', () => { if (cb.checked) sel.add(r.rule.id); else sel.delete(r.rule.id); cb.closest('tr').classList.toggle('sel', cb.checked); drawBulk(); drawRows(true); });
    const open = h('button', { class: 'open', type: 'button', 'data-i': i, 'aria-haspopup': 'dialog' }, h('span', { class: 'rtitle', text: r.rule.title }), h('span', { class: 'rtarget', title: r.rule.target }, picon(r.rule.target), h('span', { text: platName(r.rule.target) + ' · ' + r.rule.id })));
    open.addEventListener('click', () => { active = i; openRule(rows, i); });
    open.addEventListener('focus', () => { active = i; });
    return h('tr', { class: 'r' + (sel.has(r.rule.id) ? ' sel' : '') + (i === active ? ' active' : ''), 'aria-rowindex': String(i + 2) },
      h('td', { class: 'c-sel' }, cb), h('td', { class: 'c-st' }, statusPill(r.status, true)), h('td', { class: 'c-sev' }, sevPill(r.rule.severity)),
      h('td', { class: 'c-id' }, h('span', { class: 'rid', text: r.rule.id })), h('td', { class: 'c-title' }, open), h('td', { class: 'c-lv', text: t('gui.res.level_short', { n: num(r.rule.level) }) }));
  }
  function drawBulk() {
    bulk.classList.toggle('hidden', sel.size === 0);
    bulk.replaceChildren(h('b', { text: nn('gui.fix.selected', sel.size) }),
      h('button', { class: 'btn primary', type: 'button', onclick: () => { PENDING_FIX = { report: id, ids: [...sel] }; location.hash = '#/fix/' + id + '?sel=1'; } }, icon('fix'), t('gui.res.fix_selected')),
      h('button', { class: 'btn ghost', type: 'button', onclick: () => { sel.clear(); drawBulk(); drawRows(true); } }, t('gui.wiz.clear')));
  }
  function resetFilters() {
    st.q = ''; search.value = ''; st.status = ''; st.sevs.clear(); st.target = ''; tSel.value = ''; st.fixable = false;
    for (const b of statusSeg.children) b.setAttribute('aria-pressed', String(b.dataset.v === ''));
    for (const b of sevChips.children) b.setAttribute('aria-pressed', 'false');
    fixChip.setAttribute('aria-pressed', 'false');
    refilter();
  }
  let raf = 0;
  vt.addEventListener('scroll', () => { if (!raf) raf = requestAnimationFrame(() => { raf = 0; drawRows(false); }); });
  const onResize = () => drawRows(true);
  window.addEventListener('resize', onResize);
  function focusRow(i) {
    if (!rows.length) return;
    active = Math.max(0, Math.min(rows.length - 1, i));
    const top = active * ROW_H, theadH = 42;
    if (top < vt.scrollTop) vt.scrollTop = top;
    else if (top + ROW_H > vt.scrollTop + vt.clientHeight - theadH) vt.scrollTop = top + ROW_H - vt.clientHeight + theadH;
    drawRows(true);
    const b = $('button.open[data-i="' + active + '"]', tbody);
    if (b) b.focus({ preventScroll: true });
  }
  viewKeys = (e) => {
    const k = e.key.toLowerCase();
    if (k === 'j' || k === 'arrowdown' && e.target.matches('.open')) { e.preventDefault(); focusRow(active + 1); }
    else if (k === 'k' || k === 'arrowup' && e.target.matches('.open')) { e.preventDefault(); focusRow(active - 1); }
    else if (k === 'x' && active >= 0 && canFix(rows[active])) { const rid = rows[active].rule.id; if (sel.has(rid)) sel.delete(rid); else sel.add(rid); drawBulk(); drawRows(true); focusRow(active); }
    else if (k === 'enter' && active >= 0 && !e.target.matches('button,a')) { openRule(rows, active); }
  };
  if (st.status) for (const b of statusSeg.children) b.setAttribute('aria-pressed', String(b.dataset.v === st.status));
  refilter();
  drawBulk();
  return () => window.removeEventListener('resize', onResize);
};

function exportMenu(id) {
  const fmts = (S.formats || []).slice().sort((a, b) => ['html', 'pdf', 'md', 'xlsx', 'csv', 'json', 'jsonl', 'sarif', 'asff'].indexOf(a) - ['html', 'pdf', 'md', 'xlsx', 'csv', 'json', 'jsonl', 'sarif', 'asff'].indexOf(b));
  const listId = nextId('menu');
  const btn = h('button', { class: 'btn', type: 'button', 'aria-haspopup': 'menu', 'aria-expanded': 'false', 'aria-controls': listId }, icon('down'), t('gui.res.export'), icon('chev-d'));
  const list = h('div', { class: 'menu-list hidden', role: 'menu', id: listId, 'aria-label': t('gui.res.export') },
    fmts.map((f) => h('a', { role: 'menuitem', tabindex: '-1', href: '/api/report/' + id + '.' + f + '?download=1&lang=' + encodeURIComponent(LANG), download: '' },
      icon(f === 'html' ? 'globe' : f === 'pdf' || f === 'md' || f === 'xlsx' || f === 'csv' ? 'file' : 'code'), h('span', { text: hasKey('gui.export.' + f) ? t('gui.export.' + f) : f.toUpperCase() }), h('span', { class: 'fmt', text: '.' + f }))));
  const wrap = h('div', { class: 'menu' }, btn, list);
  const items = () => $$('[role=menuitem]', list);
  const open = (focusFirst) => { list.classList.remove('hidden'); btn.setAttribute('aria-expanded', 'true'); if (focusFirst) items()[0].focus(); };
  const close = (focusBtn) => { list.classList.add('hidden'); btn.setAttribute('aria-expanded', 'false'); if (focusBtn) btn.focus(); };
  btn.addEventListener('click', () => (list.classList.contains('hidden') ? open(true) : close()));
  btn.addEventListener('keydown', (e) => { if (e.key === 'ArrowDown') { e.preventDefault(); open(true); } });
  list.addEventListener('keydown', (e) => {
    const it = items(), i = it.indexOf(document.activeElement);
    if (e.key === 'ArrowDown') { e.preventDefault(); it[(i + 1) % it.length].focus(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); it[(i - 1 + it.length) % it.length].focus(); }
    else if (e.key === 'Home') { e.preventDefault(); it[0].focus(); }
    else if (e.key === 'End') { e.preventDefault(); it[it.length - 1].focus(); }
    else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(true); }
    else if (e.key === 'Tab') close();
  });
  list.addEventListener('click', () => { close(); toast(t('gui.res.export_started'), 'info'); });
  document.addEventListener('click', (e) => { if (!wrap.contains(e.target)) close(); });
  return wrap;
}

function auditText(a) {
  return [a.type && ('type: ' + a.type), a.shell && ('shell: ' + a.shell), a.command && ('command: ' + a.command), a.check && ('check: ' + a.check), a.file_path && ('file: ' + a.file_path),
    a.pattern && ((a.must_not_match ? 'must not match: ' : 'pattern: ') + a.pattern), a.expected_output && ('expected: ' + a.expected_output)].filter(Boolean).join('\n');
}
function remText(m) {
  return [m.type && ('type: ' + m.type), m.command && ('command: ' + m.command), m.file_path && ('file: ' + m.file_path), m.match_regex && ('match: ' + m.match_regex),
    m.replace_line && ('line: ' + m.replace_line), m.verify_cmd && ('verify: ' + m.verify_cmd), m.backup_files && m.backup_files.length && ('backup: ' + m.backup_files.join(', '))].filter(Boolean).join('\n');
}
function openRule(list, i) {
  const r = list[i], rule = r.rule, a = rule.audit || {}, m = rule.remediation || {};
  const sec = (k, ico, node, copy) => h('section', { class: 'dsec' }, h('h3', {}, icon(ico), t(k), copy ? copyBtn(copy, t(k)) : null), node);
  const nav = (d) => h('button', { class: 'btn sm', type: 'button', disabled: list[i + d] ? null : true, onclick: () => openRule(list, i + d) }, icon(d < 0 ? 'chev-l' : 'chev'), t(d < 0 ? 'gui.drawer.prev' : 'gui.drawer.next'));
  const at = auditText(a), rt = remText(m);
  const ref = rule.reference || '';
  const refNode = /^https?:\/\//.test(ref) ? h('a', { href: ref, target: '_blank', rel: 'noopener noreferrer' }, ref, ' ', icon('ext')) : h('p', { text: ref });
  const manual = a.type === 'manual';
  drawer.replaceChildren(
    h('div', { class: 'hd' }, h('div', { class: 'grow' },
      h('div', { class: 'row' }, statusPill(r.status), sevPill(rule.severity), h('span', { class: 'pill neutral', text: t('gui.drawer.level', { n: num(rule.level) }) }),
        manual ? h('span', { class: 'pill info' }, icon('eye'), t('gui.res.manual')) : null),
      h('h2', { id: 'dr-title', text: rule.title }),
      h('div', { class: 'row hint' }, h('span', { class: 'plat dr-plat' }, picon(rule.target, { tile: true, active: true }), h('span', {}, h('b', { text: platName(rule.target) }), ' ', h('span', { class: 'mono', text: rule.target }))), ' · ',
        h('span', { class: 'mono', text: rule.id }), copyBtn(rule.id, t('gui.table.col_id')))),
    h('button', { class: 'btn ghost icon', type: 'button', 'aria-label': t('gui.dialog.close'), onclick: () => drawer.close() }, icon('x'))),
    h('div', { class: 'bd' },
      manual ? h('section', { class: 'explain', 'aria-labelledby': 'h-explain' }, h('h3', { id: 'h-explain', class: 'row' }, icon('help'), t('gui.explain.title')),
        h('p', { text: t('gui.explain.why') }),
        h('ol', {}, h('li', { text: t('gui.explain.step1') }), h('li', { text: t('gui.explain.step2') }), h('li', { text: t('gui.explain.step3') })),
        a.command ? sec('gui.explain.instructions', 'book', h('pre', { class: 'code', tabindex: 0, text: a.command }), a.command) : null) : null,
      rule.description ? sec('gui.detail.description', 'info', h('p', { text: rule.description })) : null,
      r.details && !manual ? sec('gui.drawer.evidence', 'eye', h('pre', { class: 'code', tabindex: 0, text: r.details }), r.details) : null,
      at && !manual ? sec('gui.detail.audit', 'scan', h('pre', { class: 'code', tabindex: 0, text: at }), at) : null,
      rt ? sec('gui.detail.remediation', 'fix', h('pre', { class: 'code', tabindex: 0, text: rt }), rt)
        : sec('gui.detail.remediation', 'fix', h('p', { class: 'muted', text: t('gui.drawer.no_remediation') })),
      ref ? sec('gui.detail.reference', 'book', refNode, ref) : null,
      rule.tags && rule.tags.length ? h('div', { class: 'chips dsec' }, rule.tags.map((x) => h('span', { class: 'tag', text: x }))) : null),
    h('div', { class: 'ft' }, nav(-1), nav(1), h('span', { class: 'grow' }),
      canFix(r) ? h('button', { class: 'btn primary', type: 'button', onclick: () => { PENDING_FIX = { report: currentReportId(), ids: [rule.id] }; drawer.close(); location.hash = '#/fix/' + currentReportId() + '?sel=1'; } }, icon('fix'), t('gui.drawer.fix_this')) : null));
  drawerKeys = (e) => {
    const k = e.key.toLowerCase();
    if (k === 'j' && list[i + 1]) { e.preventDefault(); openRule(list, i + 1); }
    if (k === 'k' && list[i - 1]) { e.preventDefault(); openRule(list, i - 1); }
  };
  if (!drawer.open) { lastFocus = document.activeElement; drawer.showModal(); }
  $('.hd .btn.icon', drawer).focus();
}
function currentReportId() { const m = location.hash.match(/#\/results\/([^?/]+)/); return m ? m[1] : ''; }
drawer.addEventListener('close', () => { drawerKeys = null; const b = lastFocus; if (b && b.isConnected) b.focus(); });
drawer.addEventListener('click', (e) => { if (e.target === drawer) drawer.close(); });

// ---------- Fix ----------
const isQuickWin = (m) => m.type === 'file_line' || (m.type !== 'iac_patch' && !!m.verify_cmd);
const kindLabel = (m) => (m.type === 'iac_patch' ? 'gui.fx.kind_suggest' : m.type === 'file_line' ? 'gui.fx.kind_file' : m.type === 'sql_exec' ? 'gui.fx.kind_sql' : 'gui.fx.kind_command');

views.fix = async (root, id, query) => {
  if (!id) {
    const reps = (await api('/api/reports')).reports;
    if (!reps.length) {
      put(root, pageHead('gui.fix.title', t('gui.fix.lede')), emptyState('fix', t('gui.results.empty_title'), t('gui.results.empty_body'),
        h('a', { class: 'btn primary', href: '#/scan' }, icon('radar'), t('gui.results.go_scan'))));
      return;
    }
    location.replace('#/fix/' + reps[0].id);
    return;
  }
  const sr = await api('/api/reports/' + encodeURIComponent(id));
  const fail = sr.report.results.filter(failing);
  const fixable = fail.filter(canFix).sort((a, b) => sevRank(a.rule.severity) - sevRank(b.rule.severity) || a.rule.id.localeCompare(b.rule.id, undefined, { numeric: true }));
  const manualOnly = fail.length - fixable.length;
  const pre = query.sel === '1' && PENDING_FIX && PENDING_FIX.report === id ? new Set(PENDING_FIX.ids) : null;
  PENDING_FIX = null;
  const sel = pre || new Set();
  const st = { step: 1, preview: null, q: '', sevs: new Set(), quick: false, limit: 200, job: null, final: null };

  put(root, pageHead('gui.fix.title', t('gui.fix.lede'), null, 'gui.fx.eyebrow'),
    h('p', { class: 'muted', text: t('gui.fix.source', { date: fmtDate(sr.created), host: sr.report.hostname || '-' }) }));
  if (!S.elevated) put(root, elevationNotice('gui.fix.not_elevated_title', 'gui.fix.not_elevated_body'));
  if (!fixable.length) {
    put(root, emptyState('check', t('gui.fix.none_title'), t('gui.fix.none_body'), h('a', { class: 'btn', href: '#/results/' + id }, icon('list'), t('gui.fix.back_results'))));
    return;
  }
  const steps = [['gui.fx.step1', 'gui.fx.step1_desc'], ['gui.fx.step2', 'gui.fx.step2_desc'], ['gui.fx.step3', 'gui.fx.step3_desc'], ['gui.fx.step4', 'gui.fx.step4_desc']];
  const stepper = h('ol', { class: 'stepper', 'aria-label': t('gui.wiz.steps') });
  const body = h('div', {});
  put(root, stepper, body);
  function renderStepper() {
    stepper.replaceChildren(...steps.map(([k, dk], i) => {
      const n = i + 1;
      const can = n < st.step && st.step < 4 && n <= 2;
      const b = h('button', { type: 'button', class: n < st.step ? 'done' : '', 'aria-current': n === st.step ? 'step' : null, disabled: can || n === st.step ? null : true },
        h('span', { class: 'n', 'aria-hidden': 'true' }, n < st.step ? icon('check') : num(n)), h('span', {}, h('b', { text: t(k) }), h('small', { text: t(dk) })));
      if (can) b.addEventListener('click', () => go(n));
      return h('li', {}, b);
    }));
  }
  function go(n) {
    st.step = n; renderStepper(); body.replaceChildren();
    if (n === 1) stepSelect(); else if (n === 2) stepReview(); else if (n === 3) stepApply(); else stepSummary();
    const hd = $('h2', body); if (hd) { hd.tabIndex = -1; hd.focus(); }
  }

  function stepSelect() {
    const card = h('section', { class: 'card', 'aria-labelledby': 'h-sel' });
    const tbody = h('tbody');
    const countEl = h('span', { class: 'count', 'aria-live': 'polite' });
    const selInfo = h('b', { 'aria-live': 'polite' });
    const previewBtn = h('button', { class: 'btn primary', type: 'button' }, icon('eye'), t('gui.fix.preview'));
    previewBtn.addEventListener('click', () => go(2));
    const more = h('button', { class: 'btn', type: 'button', onclick: () => { st.limit += 200; draw(); } }, t('gui.table.more'));
    const all = h('input', { type: 'checkbox', 'aria-label': t('gui.fix.select_shown') });
    all.addEventListener('change', () => { for (const r of shown()) { if (all.checked) sel.add(r.rule.id); else sel.delete(r.rule.id); } draw(); });
    const shown = () => fixable.filter((r) => (!st.sevs.size || st.sevs.has(sevKey(r.rule.severity))) && (!st.quick || isQuickWin(r.rule.remediation)) &&
      (!st.q || (r.rule.id + ' ' + r.rule.title + ' ' + r.rule.target).toLowerCase().includes(st.q)));
    function draw() {
      const m = shown();
      tbody.replaceChildren(...m.slice(0, st.limit).map((r) => {
        const m2 = r.rule.remediation;
        const cb = h('input', { type: 'checkbox', checked: sel.has(r.rule.id) ? true : null, 'aria-label': t('gui.fix.select_rule', { id: r.rule.id }) });
        cb.addEventListener('change', () => { if (cb.checked) sel.add(r.rule.id); else sel.delete(r.rule.id); cb.closest('tr').classList.toggle('sel', cb.checked); upd(); });
        return h('tr', { class: sel.has(r.rule.id) ? 'sel' : '' }, h('td', {}, cb), h('td', {}, sevPill(r.rule.severity)),
          h('td', { class: 'opt' }, h('span', { class: 'mono hint', text: r.rule.id })),
          h('td', {}, h('div', { text: r.rule.title }), h('div', { class: 'hint mono plat' }, picon(r.rule.target), h('span', { text: r.rule.target + (m2.file_path ? ' · ' + m2.file_path : '') }))),
          h('td', { class: 'opt' }, isQuickWin(m2) ? h('span', { class: 'pill ok' }, icon('zap'), t('gui.fix.quick_win')) : h('span', { class: 'pill neutral', text: t(kindLabel(m2)) })));
      }));
      if (!m.length) tbody.append(h('tr', {}, h('td', { colspan: 5, class: 'muted', text: t('gui.table.none') })));
      countEl.textContent = nn('gui.table.count', m.length, { total: num(fixable.length) });
      more.classList.toggle('hidden', m.length <= st.limit);
      upd();
    }
    function upd() {
      const m = shown();
      all.checked = m.length > 0 && m.every((r) => sel.has(r.rule.id));
      all.indeterminate = !all.checked && m.some((r) => sel.has(r.rule.id));
      selInfo.textContent = nn('gui.fix.selected', sel.size);
      previewBtn.disabled = sel.size === 0;
    }
    const search = h('input', { class: 'input', type: 'search', 'data-search': '', 'aria-label': t('gui.table.search'), placeholder: t('gui.table.search') });
    search.addEventListener('input', () => { st.q = search.value.toLowerCase(); st.limit = 200; draw(); });
    put(card, h('h2', { class: 'sec', id: 'h-sel' }, icon('fix'), t('gui.fx.select_title')),
      manualOnly ? h('p', { class: 'hint', text: nn('gui.fix.no_auto', manualOnly) }) : null,
      h('div', { class: 'toolbar' }, h('div', { class: 'search' }, icon('scan'), search),
        h('div', { class: 'chips', role: 'group', 'aria-label': t('gui.table.severity') }, SEV_ALL.map((k) => toggleChip(t('gui.sev.' + k), false, (on) => { if (on) st.sevs.add(k); else st.sevs.delete(k); draw(); }))),
        toggleChip(t('gui.fix.quick_filter'), false, (on) => { st.quick = on; draw(); }, t('gui.fix.quick_hint')), countEl),
      h('p', { class: 'hint' }, icon('zap'), ' ', t('gui.fix.quick_hint')),
      h('div', { class: 'tblwrap' }, h('table', { class: 'stbl' }, h('caption', { class: 'sr', text: t('gui.fix.caption') }),
        h('thead', {}, h('tr', {}, h('th', { scope: 'col' }, all), h('th', { scope: 'col', text: t('gui.table.col_severity') }), h('th', { scope: 'col', class: 'opt', text: t('gui.table.col_id') }),
          h('th', { scope: 'col', text: t('gui.table.col_title') }), h('th', { scope: 'col', class: 'opt', text: t('gui.fix.col_kind') }))), tbody)),
      h('div', { class: 'actions' }, more));
    put(body, card, h('div', { class: 'bulk' }, selInfo, previewBtn, h('span', { class: 'hint', text: t('gui.fix.preview_hint') })));
    draw();
  }

  async function stepReview() {
    const card = h('section', { class: 'card', 'aria-labelledby': 'h-rev', 'aria-busy': 'true' }, h('h2', { class: 'sec', id: 'h-rev' }, icon('eye'), t('gui.fx.review_title')),
      notice('info', 'refresh', h('p', { text: t('gui.fix.dry_running') })), h('div', { class: 'sk sk-block' }));
    put(body, card);
    const ids = [...sel];
    let p;
    try { p = await api('/api/fix/preview', { report_id: id, rule_ids: ids }); }
    catch (e) { card.replaceChildren(h('h2', { class: 'sec', id: 'h-rev', text: t('gui.fx.review_title') }), errorState(e, () => go(2))); return; }
    if (st.step !== 2) return;
    st.preview = p; st.ids = ids;
    card.removeAttribute('aria-busy');
    const risk = { high: 0, medium: 0, low: 0 };
    for (const it of p.items) risk[it.risk || 'low']++;
    card.replaceChildren(h('h2', { class: 'sec', id: 'h-rev' }, icon('eye'), t('gui.fx.review_title')),
      notice('good', 'shield', h('p', { text: t('gui.fix.dry_note') })),
      h('div', { class: 'summary-grid' }, kpi('total', 'fix', t('gui.fx.changes'), p.items.length), kpi('fail', 'alert', t('gui.fx.risk_high'), risk.high),
        kpi('skip', 'info', t('gui.fx.risk_medium'), risk.medium), kpi('pass', 'shield', t('gui.fx.risk_low'), risk.low), kpi('total', 'file', t('gui.fx.backups'), p.backup_files.length)),
      p.unsupported && p.unsupported.length ? h('p', { class: 'hint', text: nn('gui.fix.unsupported', p.unsupported.length) }) : null,
      h('details', { class: 'field', open: p.failures && p.failures.length ? true : null }, h('summary', { text: t('gui.fix.output') }), h('pre', { class: 'out', tabindex: 0, text: p.output || t('gui.fix.no_output') })),
      p.failures && p.failures.length ? notice('warn', 'alert', h('p', { text: nn('gui.fix.dry_failures', p.failures.length) })) : null,
      h('p', { class: 'hint', text: t('gui.fx.risk_note') }),
      h('div', { class: 'chg-list' }, p.items.map(changeCard)),
      h('div', { class: 'wiz-foot' }, h('button', { class: 'btn', type: 'button', onclick: () => go(1) }, icon('chev-l'), t('gui.wiz.back')), h('span', { class: 'sp' }),
        h('button', { class: 'btn primary', type: 'button', onclick: () => go(3) }, t('gui.fx.continue'), icon('chev'))));
    const hd = $('h2', card); hd.tabIndex = -1; hd.focus();
  }

  function stepApply() {
    const p = st.preview;
    const code = h('input', { class: 'input mono', id: 'fx-code', autocomplete: 'off', spellcheck: 'false', maxlength: 8, 'aria-describedby': 'fx-code-h' });
    const ack = h('input', { type: 'checkbox', id: 'fx-ack' });
    const secs = h('input', { class: 'input', id: 'fx-secs', type: 'number', min: 15, max: 900, step: 5, value: p.deadman_sec });
    const range = h('input', { type: 'range', min: 15, max: 900, step: 5, value: p.deadman_sec, 'aria-label': t('gui.fix.deadman_secs') });
    const secsText = h('span', { class: 'hint', 'aria-live': 'polite' });
    const syncSecs = (v) => { v = Math.max(15, Math.min(900, Number(v) || 60)); secs.value = v; range.value = v; secsText.textContent = t('gui.fx.deadman_human', { time: dur(v) }); };
    secs.addEventListener('change', () => syncSecs(secs.value)); range.addEventListener('input', () => syncSecs(range.value));
    syncSecs(p.deadman_sec);
    const apply = h('button', { class: 'btn danger lg', type: 'button', disabled: true }, icon('fix'), t('gui.fix.apply'));
    const why = h('p', { class: 'hint', 'aria-live': 'polite' });
    function validate() {
      const ok = S.elevated && ack.checked && code.value.trim().toUpperCase() === S.nonce;
      apply.disabled = !ok;
      why.textContent = !S.elevated ? t('gui.fix.apply_blocked') : ok ? t('gui.fx.ready') : t('gui.fx.not_ready');
    }
    code.addEventListener('input', validate); ack.addEventListener('change', validate);
    validate();
    apply.addEventListener('click', async () => {
      apply.disabled = true;
      try {
        const j = await api('/api/fix/apply', { report_id: id, rule_ids: st.ids, confirm_code: code.value.trim(), deadman_sec: Number(secs.value) || 0 });
        trackApply(j.id, (v) => { st.final = v; go(4); });
      } catch (e) { toast(errText(e), 'err'); validate(); }
    });
    put(body, h('section', { class: 'card', 'aria-labelledby': 'h-apply' }, h('h2', { class: 'sec', id: 'h-apply' }, icon('lock'), t('gui.fx.apply_title')),
      h('div', { class: 'grid' },
        h('div', { class: 's6' }, h('h3', { class: 'sub', text: t('gui.fix.snapshot') }), h('p', { text: nn('gui.fix.snapshot_body', p.backup_files.length, { dir: p.snapshot_dir }) }),
          p.backup_files.length ? h('details', {}, h('summary', { text: t('gui.fx.show_files') }), h('pre', { class: 'out', tabindex: 0, text: p.backup_files.join('\n') })) : null,
          h('h3', { class: 'sub', text: t('gui.fix.deadman') }), h('p', { text: t('gui.fix.deadman_body') }),
          h('div', { class: 'field' }, h('label', { for: 'fx-secs', text: t('gui.fix.deadman_secs') }), h('div', { class: 'row' }, h('div', { class: 'grow' }, range), h('div', {}, secs))), secsText),
        h('div', { class: 's6' }, h('div', { class: 'apply-panel' }, h('h3', { class: 'sub', text: t('gui.fix.confirm') }),
          h('p', {}, t('gui.fx.code_intro'), ' ', h('span', { class: 'code-big', text: S.nonce })),
          h('div', { class: 'field' }, h('label', { for: 'fx-code', text: t('gui.fix.code_label', { code: S.nonce }) }), code, h('p', { class: 'hint', id: 'fx-code-h', text: t('gui.fix.code_hint') })),
          h('div', { class: 'field' }, h('label', { class: 'choice' }, ack, h('div', {}, h('b', { text: t('gui.fix.ack') }), h('span', { text: nn('gui.fx.ack_detail', p.items.length) })))),
          !S.elevated ? elevationNotice('gui.fix.not_elevated_title', 'gui.fix.apply_blocked') : null,
          notice('warn', 'alert', h('p', { text: t('gui.fix.beta_caution') })),
          h('div', { class: 'actions' }, apply), why))),
      h('div', { class: 'wiz-foot' }, h('button', { class: 'btn', type: 'button', onclick: () => go(2) }, icon('chev-l'), t('gui.wiz.back')))));
  }

  function stepSummary() {
    const v = st.final || {};
    const kind = v.state === 'done' ? 'good' : v.state === 'rolled_back' ? 'warn' : 'err';
    const titleKey = v.state === 'done' ? 'gui.fx.sum_done' : v.state === 'rolled_back' ? 'gui.fx.sum_rolled' : 'gui.fx.sum_failed';
    const msg = v.state === 'done' ? t('gui.fix.result_done') : v.state === 'rolled_back' ? t('gui.fix.result_rolled_back') : t(v.rolled_back ? 'gui.fix.result_failed_rolled' : 'gui.fix.result_failed', { error: v.error || '' });
    const rescan = h('button', { class: 'btn primary', type: 'button' }, icon('radar'), t('gui.fx.rescan'));
    rescan.addEventListener('click', async () => {
      rescan.disabled = true;
      try { const j = await api('/api/scan', sr.params || { scope: 'auto', level: 1 }); location.hash = '#/scan/' + j.id; } catch (e) { toast(errText(e), 'err'); rescan.disabled = false; }
    });
    put(body, h('section', { class: 'card', 'aria-labelledby': 'h-sum' },
      h('div', { class: 'outcome ' + kind }, h('div', { class: 'art', 'aria-hidden': 'true' }, icon(kind === 'good' ? 'check' : kind === 'warn' ? 'undo' : 'alert', 'lg')),
        h('div', {}, h('h2', { id: 'h-sum', text: t(titleKey) }), h('p', { text: msg }), v.snapshot_id ? h('p', { class: 'hint mono', text: t('gui.fix.snapshot_taken', { id: v.snapshot_id }) }) : null)),
      st.preview ? [h('h3', { class: 'sub', text: nn('gui.fix.changes', st.preview.items.length) }),
        h('ul', { class: 'toplist' }, st.preview.items.map((it) => h('li', { class: 'row' }, statusPill(v.state === 'done' ? 'PASS' : 'SKIP', true), riskPill(it.risk || 'low'), h('span', { class: 'mono hint', text: it.id }), h('span', { class: 'grow', text: it.title }))))] : null,
      v.output ? h('details', { class: 'field' }, h('summary', { text: t('gui.fx.apply_output') }), h('pre', { class: 'out', tabindex: 0, text: v.output })) : null,
      h('div', { class: 'actions' }, v.state === 'done' ? rescan : null, h('a', { class: 'btn', href: '#/history' }, icon('clock'), t('gui.fix.go_history')),
        h('a', { class: 'btn', href: '#/results/' + id }, icon('list'), t('gui.fix.back_results')))));
  }
  renderStepper();
  go(pre && pre.size ? 2 : 1);
};

function changeCard(it) {
  const reasons = (it.risk_reasons || []).map((r) => h('span', { class: 'tag', text: hasKey('gui.fx.reason.' + r) ? t('gui.fx.reason.' + r) : r }));
  const d = it.diff;
  let diffNode = null;
  if (d) {
    if (d.unchanged) diffNode = notice('good', 'check', h('p', { text: t('gui.diff.unchanged', { file: d.file }) }));
    else {
      const rows = d.lines.map((l) => (l.op === '@' ? h('tr', { class: 'gap' }, h('td', { colspan: 4, text: '⋯' }))
        : h('tr', { class: l.op === '+' ? 'add' : l.op === '-' ? 'del' : '' }, h('td', { class: 'ln', text: l.old ? String(l.old) : '' }), h('td', { class: 'ln', text: l.new ? String(l.new) : '' }),
          h('td', { class: 'op', text: l.op === ' ' ? ' ' : l.op }, l.op === '+' ? h('span', { class: 'sr', text: t('gui.diff.added') }) : l.op === '-' ? h('span', { class: 'sr', text: t('gui.diff.removed') }) : null), h('td', { text: l.text }))));
      diffNode = h('div', { class: 'diff', role: 'region', 'aria-label': t('gui.diff.aria', { file: d.file }), tabindex: 0 },
        h('div', { class: 'fn' }, icon('file'), h('span', { text: d.file })),
        d.error ? h('div', { class: 'fn', text: t('gui.diff.err_' + d.error) }) : null,
        h('table', {}, h('tbody', {}, rows)), d.truncated ? h('div', { class: 'fn', text: t('gui.diff.truncated') }) : null);
    }
  }
  return h('article', { class: 'chg', 'aria-label': it.id + ' ' + it.title },
    h('header', {}, riskPill(it.risk || 'low'), sevPill(it.severity), h('span', { class: 'mono hint', text: it.id }), h('span', { class: 't', text: it.title })),
    h('div', { class: 'body' },
      h('div', { class: 'meta' }, h('span', { text: t('gui.fx.kind_label', { kind: t(kindLabel({ type: it.type })) }) }),
        it.files && it.files.length ? h('span', {}, t('gui.fix.files'), ' ', h('code', { class: 'mono', text: it.files.join(', ') })) : null),
      reasons.length ? h('div', { class: 'chips', 'aria-label': t('gui.fx.reasons') }, reasons) : null,
      diffNode ? h('div', { class: 'field' }, diffNode) : null,
      it.command ? h('div', { class: 'field' }, h('span', { class: 'lbl', text: it.applied_directly ? t('gui.fx.will_run') : t('gui.fx.suggested') }), cmdBox(it.command, t('gui.fix.command'))) : null,
      !d && it.line ? h('div', { class: 'field' }, h('span', { class: 'lbl', text: t('gui.fix.line') }), cmdBox(it.line, t('gui.fix.line'))) : null,
      it.verify ? h('p', { class: 'hint' }, t('gui.fx.verify'), ' ', h('code', { class: 'mono', text: it.verify })) : null,
      it.applied_directly ? null : notice('info', 'info', h('p', { text: t('gui.fix.suggest_note') }))));
}

function trackApply(id, onEnd) {
  const wrap = h('div', {});
  const assertive = h('div', { class: 'sr', 'aria-live': 'assertive' });
  let tick = null, es = null, ended = false, warned = {};
  const confirmBtn = h('button', { class: 'btn primary lg', type: 'button' }, icon('check'), t('gui.fix.keep'));
  const rollBtn = h('button', { class: 'btn danger lg', type: 'button' }, icon('undo'), t('gui.fix.rollback_now'));
  confirmBtn.addEventListener('click', () => decide('confirm'));
  rollBtn.addEventListener('click', () => decide('rollback'));
  async function decide(what) {
    confirmBtn.disabled = rollBtn.disabled = true;
    try { render(await api('/api/jobs/' + id + '/' + what, {})); } catch (e) { toast(errText(e), 'err'); confirmBtn.disabled = rollBtn.disabled = false; }
  }
  let shownState = '';
  function render(v) {
    if (ended) return;
    if (v.state === 'running') {
      if (shownState !== 'running') {
        shownState = 'running';
        showDialog(t('gui.fix.dlg_title'), [wrap, assertive], [], true);
      }
      wrap.replaceChildren(h('p', { class: 'verdict', text: t('gui.fix.phase_' + (v.phase || 'snapshot')) }),
        h('div', { class: 'progress indet', role: 'progressbar', 'aria-label': t('gui.fix.progress') }, h('i')),
        v.snapshot_id ? h('p', { class: 'muted', text: t('gui.fix.snapshot_taken', { id: v.snapshot_id }) }) : null);
    } else if (v.state === 'awaiting_confirm') {
      if (shownState === 'awaiting') return;
      shownState = 'awaiting';
      const total = Math.max(1, Math.round((new Date(v.deadline) - new Date(v.started)) / 1000) || 60);
      const R = 96, C = 2 * Math.PI * R;
      const fg = svgEl('circle', { class: 'fg', cx: 110, cy: 110, r: R, 'stroke-dasharray': C, 'stroke-dashoffset': 0 });
      const n = h('b', {});
      const ring = h('div', { class: 'ring', role: 'timer', 'aria-label': t('gui.fix.countdown_label'), 'aria-live': 'off' },
        svgEl('svg', { viewBox: '0 0 220 220', 'aria-hidden': 'true' }, svgEl('circle', { class: 'bg', cx: 110, cy: 110, r: R }), fg),
        h('div', { class: 'num' }, h('div', {}, n, h('small', { text: t('gui.fx.seconds_left') }))));
      const upd = () => {
        const left = Math.max(0, Math.ceil((new Date(v.deadline) - Date.now()) / 1000));
        n.textContent = num(left);
        fg.setAttribute('stroke-dashoffset', C * (1 - Math.min(1, left / Math.max(total, left))));
        ring.classList.toggle('urgent', left <= 10);
        for (const s of [30, 10, 5]) if (left <= s && !warned[s] && left > 0) { warned[s] = true; assertive.textContent = t('gui.fx.countdown_announce', { n: num(left) }); }
      };
      upd(); tick = setInterval(upd, 250);
      wrap.replaceChildren(h('div', { class: 'deadman' }, ring,
        h('h3', { class: 'verdict', text: t('gui.fix.await_title') }), h('p', { text: t('gui.fix.await_body') }),
        h('p', { class: 'muted mono', text: t('gui.fix.snapshot_taken', { id: v.snapshot_id }) })));
      showDialog(t('gui.fx.deadman_title'), [wrap, assertive], [rollBtn, confirmBtn], true);
      assertive.textContent = t('gui.fix.await_title');
      confirmBtn.focus();
    } else {
      ended = true; clearInterval(tick); if (es) es.close();
      if (dlg.open) dlg.close();
      onEnd(v);
    }
  }
  es = new EventSource('/api/jobs/' + encodeURIComponent(id) + '/events');
  es.addEventListener('job', (ev) => render(JSON.parse(ev.data)));
  es.onerror = () => { if (!ended) api('/api/jobs/' + id).then(render).catch(() => {}); };
  showDialog(t('gui.fix.dlg_title'), [wrap, assertive], [], true);
  wrap.replaceChildren(h('div', { class: 'progress indet', role: 'progressbar', 'aria-label': t('gui.fix.progress') }, h('i')));
}

// ---------- History ----------
const actionLabel = (a) => { const k = 'gui.action.' + a.replace(/\./g, '_'); return hasKey(k) ? t(k) : a; };
views.history = async (root, sub, query) => {
  if (sub === 'compare') return compareView(root, query);
  const d = await api('/api/history');
  put(root, pageHead('gui.history.title', t('gui.history.lede'), null, 'gui.hist.eyebrow'));
  put(root, h('section', { class: 'card', 'aria-labelledby': 'h-tr' }, h('h2', { class: 'sec', id: 'h-tr' }, icon('clock'), t('gui.history.trend')),
    d.trend.length >= 2 ? trendChart(d.trend, 150) : h('p', { class: 'muted', text: t('gui.history.trend_empty') })));

  const pick = [];
  const cmpBtn = h('button', { class: 'btn primary', type: 'button', disabled: true }, icon('compare'), t('gui.hist.compare'));
  cmpBtn.addEventListener('click', () => { location.hash = '#/history/compare?a=' + pick[0] + '&b=' + pick[1]; });
  const cmpHint = h('span', { class: 'hint', 'aria-live': 'polite', text: t('gui.hist.compare_hint') });
  const boxes = {};
  const updPick = () => {
    for (const [rid, cb] of Object.entries(boxes)) { cb.checked = pick.includes(rid); cb.closest('tr').classList.toggle('sel', cb.checked); }
    cmpBtn.disabled = pick.length !== 2;
    cmpHint.textContent = pick.length === 2 ? t('gui.hist.compare_ready') : t('gui.hist.compare_hint');
  };
  const repRows = d.reports.map((r) => {
    const cb = h('input', { type: 'checkbox', 'aria-label': t('gui.hist.pick', { date: fmtDate(r.created) }) });
    boxes[r.id] = cb;
    cb.addEventListener('change', () => { const i = pick.indexOf(r.id); if (cb.checked && i < 0) { pick.push(r.id); if (pick.length > 2) pick.shift(); } else if (!cb.checked && i >= 0) pick.splice(i, 1); updPick(); });
    return h('tr', {}, h('td', {}, cb), h('td', { text: fmtDate(r.created) }), h('td', { class: 'opt', text: r.hostname || '-' }),
      h('td', {}, h('span', { class: 'pill ' + (r.score >= 80 ? 'ok' : r.score >= 50 ? 'warn' : 'bad'), text: pct(r.score) })),
      h('td', { class: 'opt', text: t('gui.history.pass_fail', { passed: num(r.passed), failed: num(r.failed) }) }),
      h('td', { class: 'opt mono hint' }, scopeIcons(r.params.target), h('span', { text: (r.params.scope || '') + (r.params.target ? ':' + r.params.target : '') + ' L' + (r.params.level || 1) })),
      h('td', {}, h('a', { class: 'btn sm', href: '#/results/' + r.id }, t('gui.history.open'))));
  });
  put(root, h('section', { class: 'card', 'aria-labelledby': 'h-scans' }, h('div', { class: 'card-head' }, h('h2', { class: 'sec', id: 'h-scans' }, icon('list'), t('gui.history.scans')),
    d.reports.length > 1 ? h('div', { class: 'act row' }, cmpHint, cmpBtn) : null),
  d.reports.length ? h('div', { class: 'tblwrap' }, h('table', { class: 'stbl' }, h('caption', { class: 'sr', text: t('gui.history.scans') }), h('thead', {}, h('tr', {},
    h('th', { scope: 'col' }, h('span', { class: 'sr', text: t('gui.hist.col_pick') })), h('th', { scope: 'col', text: t('gui.history.col_date') }), h('th', { scope: 'col', class: 'opt', text: t('gui.history.col_host') }),
    h('th', { scope: 'col', text: t('gui.history.col_score') }), h('th', { scope: 'col', class: 'opt', text: t('gui.history.col_result') }), h('th', { scope: 'col', class: 'opt', text: t('gui.history.col_scope') }),
    h('th', { scope: 'col' }, h('span', { class: 'sr', text: t('gui.history.col_actions') })))), h('tbody', {}, repRows)))
    : emptyState('list', t('gui.history.no_scans'), t('gui.results.empty_body'), h('a', { class: 'btn primary', href: '#/scan' }, icon('radar'), t('gui.results.go_scan')))));

  const snapRows = d.snapshots.map((s) => h('tr', {}, h('td', { text: fmtDate(s.created) }), h('td', { class: 'mono opt', text: s.id }),
    h('td', { text: nn('gui.history.files', s.files) }),
    h('td', {}, h('button', { class: 'btn sm', type: 'button', disabled: S.elevated ? null : true, title: S.elevated ? null : t('gui.history.restore_blocked'), onclick: () => openRestore(s) }, icon('undo'), t('gui.history.restore')))));
  put(root, h('section', { class: 'card', 'aria-labelledby': 'h-snaps' }, h('h2', { class: 'sec', id: 'h-snaps' }, icon('shield'), t('gui.hist.restore_points')),
    h('p', { class: 'hint', text: t('gui.hist.restore_intro') }),
    !S.elevated && d.snapshots.length ? elevationNotice('gui.history.restore_blocked', 'gui.admin.missing_body') : null,
    d.snapshots.length ? h('div', { class: 'tblwrap' }, h('table', { class: 'stbl' }, h('caption', { class: 'sr', text: t('gui.history.snapshots') }), h('thead', {}, h('tr', {},
      h('th', { scope: 'col', text: t('gui.history.col_date') }), h('th', { scope: 'col', class: 'opt', text: t('gui.history.col_snapshot') }), h('th', { scope: 'col', text: t('gui.history.col_files') }),
      h('th', { scope: 'col' }, h('span', { class: 'sr', text: t('gui.history.col_actions') })))), h('tbody', {}, snapRows)))
      : h('p', { class: 'muted', text: t('gui.history.no_snapshots') })));

  const actRows = d.actions.map((a) => h('tr', {}, h('td', { class: 'nowrap', text: fmtDate(a.time) }), h('td', { text: actionLabel(a.action) }),
    h('td', { class: 'mono opt hint', text: a.detail || '' }),
    h('td', {}, h('span', { class: 'pill ' + (a.ok ? 'ok' : 'bad') }, icon(a.ok ? 'check' : 'x'), a.ok ? t('gui.history.ok') : t('gui.history.failed')))));
  put(root, h('section', { class: 'card', 'aria-labelledby': 'h-act' }, h('details', {}, h('summary', {}, h('span', { class: 'sec', id: 'h-act', text: t('gui.history.actions') }), ' ', h('span', { class: 'pill neutral', text: num(d.actions.length) })),
    d.actions.length ? h('div', { class: 'tblwrap field' }, h('table', { class: 'stbl' }, h('caption', { class: 'sr', text: t('gui.history.actions') }), h('thead', {}, h('tr', {},
      h('th', { scope: 'col', text: t('gui.history.col_date') }), h('th', { scope: 'col', text: t('gui.history.col_action') }), h('th', { scope: 'col', class: 'opt', text: t('gui.history.col_detail') }), h('th', { scope: 'col', text: t('gui.history.col_outcome') }))), h('tbody', {}, actRows)))
      : h('p', { class: 'muted', text: t('gui.history.no_actions') }))));
};

async function compareView(root, q) {
  const d = await api('/api/compare?a=' + encodeURIComponent(q.a || '') + '&b=' + encodeURIComponent(q.b || ''));
  const delta = d.delta;
  put(root, pageHead('gui.cmp.title', t('gui.cmp.lede'), [h('a', { class: 'btn', href: '#/history' }, icon('chev-l'), t('gui.cmp.back'))], 'gui.hist.eyebrow'));
  const side = (r, k) => h('div', { class: 'cmp-side' }, gauge(r.score, true), h('div', {}, h('div', { class: 'eyebrow', text: t(k) }), h('b', { text: fmtDate(r.created) }),
    h('div', { class: 'hint', text: t('gui.history.pass_fail', { passed: num(r.passed), failed: num(r.failed) }) }), h('a', { href: '#/results/' + r.id, text: t('gui.history.open') })));
  put(root, h('section', { class: 'card' }, h('div', { class: 'cmp-head' }, side(d.before, 'gui.cmp.before'),
    h('div', { class: 'cmp-arrow' }, icon('chev', 'xl'), h('span', { class: 'delta ' + (delta > 0.05 ? 'up' : delta < -0.05 ? 'down' : '') },
      t(delta > 0.05 ? 'gui.dash.delta_up' : delta < -0.05 ? 'gui.dash.delta_down' : 'gui.dash.delta_same', { n: NF.format(Math.abs(Math.round(delta * 10) / 10)) }))),
    side(d.after, 'gui.cmp.after'))));
  const kinds = [['regressed', 'x', 'fail'], ['fixed', 'check', 'pass'], ['added', 'plus', 'total'], ['removed', 'skip', 'skip']];
  put(root, h('div', { class: 'kpis field' }, kinds.map(([k, ico, cls]) => kpi(cls, ico, t('gui.cmp.' + k), d.counts[k]))),
    h('p', { class: 'hint', text: nn('gui.cmp.unchanged', d.counts.unchanged) }));
  const listBox = h('div', { role: 'tabpanel' });
  const draw = (k) => {
    const l = d.lists[k];
    listBox.replaceChildren(l.length ? h('div', { class: 'tblwrap' }, h('table', { class: 'stbl' }, h('caption', { class: 'sr', text: t('gui.cmp.' + k) }),
      h('thead', {}, h('tr', {}, h('th', { scope: 'col', text: t('gui.table.col_severity') }), h('th', { scope: 'col', class: 'opt', text: t('gui.table.col_id') }),
        h('th', { scope: 'col', text: t('gui.table.col_title') }), h('th', { scope: 'col', text: t('gui.cmp.change') }))),
      h('tbody', {}, l.map((r) => h('tr', {}, h('td', {}, sevPill(r.severity)), h('td', { class: 'opt mono hint', text: r.id }),
        h('td', {}, h('div', { text: r.title }), h('div', { class: 'hint plat' }, picon(r.target), h('span', { text: platName(r.target) }))),
        h('td', { class: 'row' }, r.before ? statusPill(r.before, true) : h('span', { class: 'pill neutral', text: t('gui.cmp.none') }), icon('chev'),
          r.after ? statusPill(r.after, true) : h('span', { class: 'pill neutral', text: t('gui.cmp.none') })))))))
      : h('p', { class: 'muted', text: t('gui.cmp.empty') }),
    d.counts[k] > l.length ? h('p', { class: 'hint', text: nn('gui.cmp.truncated', d.counts[k] - l.length) }) : null);
  };
  const first = kinds.find(([k]) => d.counts[k]) || kinds[0];
  put(root, h('section', { class: 'card' }, tablist(t('gui.cmp.title'), kinds.map(([k, ico]) => ({ id: k, label: t('gui.cmp.' + k), icon: icon(ico), count: d.counts[k] })), first[0], draw), listBox));
  draw(first[0]);
}

function openRestore(s) {
  const code = h('input', { class: 'input mono', id: 'rs-code', autocomplete: 'off', spellcheck: 'false', maxlength: 8 });
  const go = h('button', { class: 'btn danger', type: 'button', disabled: true }, icon('undo'), t('gui.restore.confirm'));
  go.addEventListener('click', async () => {
    go.disabled = true;
    try {
      const r = await api('/api/snapshots/' + encodeURIComponent(s.id) + '/restore', { confirm_code: code.value.trim() });
      showDialog(t('gui.restore.title'), [notice('good', 'check', h('p', { text: t('gui.restore.done') })), h('pre', { class: 'out', text: r.output || '' })], [closeBtn('gui.dialog.close', true)]);
      toast(t('gui.restore.done'));
    } catch (e) { showDialog(t('gui.restore.title'), notice('err', 'alert', h('p', { text: errText(e) })), [closeBtn('gui.dialog.close', true)]); }
  });
  code.addEventListener('input', () => { go.disabled = code.value.trim().toUpperCase() !== S.nonce; });
  showDialog(t('gui.restore.title'), h('div', {},
    notice('warn', 'alert', h('p', { text: t('gui.restore.warning', { id: s.id, date: fmtDate(s.created) }) }), h('p', { text: nn('gui.restore.files', s.files) })),
    h('p', {}, t('gui.fx.code_intro'), ' ', h('span', { class: 'code-big', text: S.nonce })),
    h('div', { class: 'field' }, h('label', { for: 'rs-code', text: t('gui.fix.code_label', { code: S.nonce }) }), code)),
  [closeBtn('gui.dialog.cancel', true), go]);
}

// ---------- Platforms ----------
views.platforms = async (root, sub, query) => {
  const d = await api('/api/platforms');
  const st = { cat: CATS.includes(query.cat) ? query.cat : 'all', q: (query.q || '').toLowerCase(), detected: query.detected === '1' };
  put(root, pageHead('gui.nav.platforms', t('gui.plat.lede', { rules: num(d.rules), targets: num(d.targets), categories: num(d.categories.length) }), null, 'gui.plat.eyebrow'));
  const overview = h('div', { class: 'quick' }, d.categories.map((c) => {
    const b = h('button', { type: 'button' }, glyph(c.id), h('b', { text: t('gui.cat.' + c.id) }), h('span', { text: t('gui.plat.cat_meta', { targets: num(c.targets), rules: num(c.rules) }) }));
    b.addEventListener('click', () => { st.cat = c.id; tabs.replaceWith(tabs = mkTabs()); draw(); $('[aria-selected=true]', tabs).focus(); });
    return b;
  }));
  const search = h('input', { class: 'input', type: 'search', 'data-search': '', placeholder: t('gui.wiz.search'), 'aria-label': t('gui.wiz.search'), value: query.q || '' });
  search.addEventListener('input', () => { st.q = search.value.trim().toLowerCase(); draw(); });
  const mkTabs = () => tablist(t('gui.wiz.categories'), [{ id: 'all', label: t('gui.cat.all'), icon: icon('c-all'), count: d.targets }, ...d.categories.map((c) => ({ id: c.id, label: t('gui.cat.' + c.id), icon: catIcon(c.id), count: c.targets }))], st.cat, (id) => { st.cat = id; draw(); });
  let tabs = mkTabs();
  const body = h('div', { role: 'tabpanel' });
  const countEl = h('span', { class: 'count', 'aria-live': 'polite' });
  function draw() {
    body.setAttribute('aria-labelledby', ($('[aria-selected=true]', tabs) || {}).id || '');
    const out = []; let n = 0;
    for (const c of d.categories) {
      if (st.cat !== 'all' && c.id !== st.cat) continue;
      const groups = c.groups.map((g) => ({ id: g.id, targets: g.targets.filter((x) => (!st.q || x.name.toLowerCase().includes(st.q) || x.id.includes(st.q)) && (!st.detected || x.detected)) })).filter((g) => g.targets.length);
      if (!groups.length) continue;
      out.push(h('h2', { class: 'sec cat-h' }, glyph(c.id, true), t('gui.cat.' + c.id)));
      for (const g of groups) {
        n += g.targets.length;
        out.push(h('section', { class: 'pgroup' }, h('h3', { text: t('gui.group.' + g.id) }), h('div', { class: 'tgrid' }, g.targets.map((x) => h('div', { class: 'tcard' }, picon(x.id, { tile: true, size: 'md', cat: x.category }),
          h('span', { class: 'grow' }, h('span', { class: 'nm', text: x.name }), h('span', { class: 'id mono', text: x.id }),
            h('span', { class: 'bd' }, h('span', { class: 'pill neutral', text: nn('gui.scan.target_count', x.total) }),
              x.auto ? h('span', { class: 'pill info', text: nn('gui.plat.auto', x.auto) }) : null,
              x.manual ? h('span', { class: 'pill neutral', text: nn('gui.plat.manual', x.manual) }) : null,
              x.device ? h('span', { class: 'pill info' }, icon('file'), t('gui.plat.device')) : null,
              x.detected ? h('span', { class: 'pill ok' }, icon('check'), t('gui.plat.detected')) : null),
            h('span', { class: 'row field' }, h('a', { class: 'btn sm', href: '#/scan?target=' + encodeURIComponent(x.id), 'aria-label': t('gui.plat.scan_aria', { name: x.name }) }, icon('radar'), t('gui.plat.scan')))))))));
      }
    }
    countEl.textContent = nn('gui.plat.count', n);
    body.replaceChildren(...(out.length ? out : [h('p', { class: 'muted empty', text: t('gui.wiz.no_match') })]));
  }
  put(root, h('section', { class: 'card' }, h('h2', { class: 'sec' }, icon('layers'), t('gui.plat.categories')), overview),
    h('div', { class: 'toolbar' }, h('div', { class: 'search' }, icon('scan'), search), toggleChip(t('gui.plat.detected_only'), st.detected, (v) => { st.detected = v; draw(); }), countEl),
    tabs, body);
  draw();
};

// ---------- Settings ----------
views.settings = async (root) => {
  put(root, pageHead('gui.settings.title', t('gui.settings.lede')));
  const st = S.settings;
  const langSel = h('select', { class: 'input', id: 's-lang', onchange: (e) => changeLang(e.target.value).then(() => toast(t('gui.settings.saved'))).catch((x) => toast(errText(x), 'err')) },
    S.languages.map((l) => h('option', { value: l.tag, selected: l.tag === S.lang ? true : null, text: l.name })));
  const themeRadio = (v) => h('label', { class: 'choice' }, h('input', { type: 'radio', name: 'theme', value: v, checked: st.theme === v ? true : null,
    onchange: () => saveSettings({ theme: v }).then(() => toast(t('gui.settings.saved'))).catch((x) => toast(errText(x), 'err')) }), icon(THEME_ICON[v], 'lg'), h('div', {}, h('b', { text: t('gui.theme.' + v) }), h('span', { text: t('gui.set.theme_' + v) })));
  const dir = h('input', { class: 'input mono', id: 's-dir', value: st.rules_dir, spellcheck: 'false' });
  const dirMsg = h('div', { 'aria-live': 'polite' });
  const ai = S.ai || {};
  const beta = (S.beta || []).includes(S.lang);
  put(root, h('div', { class: 'grid' },
    h('section', { class: 'card s6', 'aria-labelledby': 'h-app' }, h('h2', { class: 'sec', id: 'h-app' }, icon('globe'), t('gui.settings.appearance')),
      h('div', { class: 'field' }, h('label', { for: 's-lang', text: t('gui.lang.label') }), langSel, h('p', { class: 'hint', text: t('gui.set.beta_note') })),
      beta ? notice('info', 'info', h('p', { text: t('gui.set.beta_current') })) : null,
      h('fieldset', {}, h('legend', { class: 'lbl', text: t('gui.settings.theme') }), h('div', { class: 'choices' }, THEMES.map(themeRadio)))),
    h('section', { class: 'card s6', 'aria-labelledby': 'h-rules' }, h('h2', { class: 'sec', id: 'h-rules' }, icon('book'), t('gui.settings.rules')),
      h('form', { onsubmit: async (e) => {
        e.preventDefault();
        try { await saveSettings({ rules_dir: dir.value.trim() }); dirMsg.replaceChildren(notice('good', 'check', h('p', { text: t('gui.settings.saved') }))); }
        catch (x) { dirMsg.replaceChildren(notice('err', 'alert', h('p', { text: errText(x) }))); }
      } }, h('div', { class: 'field' }, h('label', { for: 's-dir', text: t('gui.scan.rules_dir') }), dir, h('p', { class: 'hint', text: t('gui.settings.rules_hint', { dir: S.default_rules_dir }) })),
      dirMsg, h('div', { class: 'actions' }, h('button', { class: 'btn primary', type: 'submit' }, icon('check'), t('gui.settings.save')))),
      h('h3', { class: 'sub', text: t('gui.keys.title') }), h('p', { class: 'hint', text: t('gui.keys.intro') }),
      h('button', { class: 'btn', type: 'button', onclick: showKeys }, icon('keyboard'), t('gui.keys.open'))),
    h('section', { class: 'card s6', 'aria-labelledby': 'h-ai' }, h('h2', { class: 'sec', id: 'h-ai' }, icon('c-ai'), t('gui.settings.ai')),
      h('dl', { class: 'kv' }, h('dt', { text: t('gui.settings.ai_provider') }), h('dd', { text: ai.provider || '-' }),
        h('dt', { text: t('gui.settings.ai_model') }), h('dd', { text: ai.model || '-' }),
        h('dt', { text: t('gui.settings.ai_endpoint') }), h('dd', { class: 'mono', text: ai.endpoint || '-' }),
        h('dt', { text: t('gui.settings.ai_key') }), h('dd', { class: 'mono', text: ai.key_env ? t('gui.settings.ai_key_env', { name: ai.key_env }) : '-' })),
      h('p', { class: 'hint', text: t('gui.settings.ai_note') })),
    h('section', { class: 'card s6', 'aria-labelledby': 'h-paths' }, h('h2', { class: 'sec', id: 'h-paths' }, icon('lock'), t('gui.settings.paths')),
      h('dl', { class: 'kv' }, h('dt', { text: t('gui.settings.data_dir') }), h('dd', { class: 'mono', text: S.data_dir }),
        h('dt', { text: t('gui.settings.snapshot_dir') }), h('dd', { class: 'mono', text: S.snapshot_dir }),
        h('dt', { text: t('gui.settings.binary') }), h('dd', { class: 'mono', text: S.exe })),
      notice('good', 'shield', h('p', { text: t('gui.settings.privacy') })))));
};

// ---------- About ----------
const SITE = 'https://www.tmaegis.dev';
views.about = async (root) => {
  put(root, pageHead('gui.about.title', null));
  const ext = (href, ico, label) => h('a', { class: 'btn', href, target: '_blank', rel: 'noopener noreferrer' }, icon(ico), label, h('span', { class: 'sr', text: t('gui.about.new_tab') }));
  put(root, h('div', { class: 'grid' },
    h('section', { class: 'card s12' }, h('div', { class: 'about-hero' }, h('img', { src: '/static/logo-mark.svg', alt: '' }),
      h('div', { class: 'grow' }, h('h2', { class: 'gradient-text', text: 'TMAegis™' }), h('p', { class: 'lede', text: t('gui.about.tagline') }),
        h('div', { class: 'row' }, h('span', { class: 'pill info', text: t('gui.about.version', { version: S.version || 'dev' }) }), h('span', { class: 'pill warn', text: t('gui.beta.label') }), h('span', { class: 'pill neutral mono', text: S.os + '/' + S.arch })),
        h('div', { class: 'links' }, ext(SITE, 'globe', t('gui.about.website')), ext(SITE + '/docs', 'book', t('gui.about.docs')), ext('https://github.com/tmaegis/tmaegis', 'code', t('gui.about.source')))))),
    h('section', { class: 'card s6', 'aria-labelledby': 'h-lic' }, h('h2', { class: 'sec', id: 'h-lic' }, icon('book'), t('gui.about.license_title')),
      h('p', { text: t('gui.about.license') }), h('p', { class: 'hint', text: t('gui.about.license_note') }),
      h('p', {}, h('a', { href: 'https://mozilla.org/MPL/2.0/', target: '_blank', rel: 'noopener noreferrer' }, t('gui.about.license_link'), ' ', icon('ext'))),
      h('p', { class: 'hint', text: t('gui.about.fonts') }), h('p', { class: 'hint', text: t('gui.about.brands') })),
    h('section', { class: 'card s6', 'aria-labelledby': 'h-auth' }, h('h2', { class: 'sec', id: 'h-auth' }, icon('shield'), t('gui.about.authors_title')),
      h('p', { text: t('gui.about.founders') }), h('p', { class: 'hint', text: t('gui.about.trademark') }), h('p', { class: 'hint', text: t('gui.about.copyright') })),
    h('section', { class: 'card s12', 'aria-labelledby': 'h-priv' }, h('h2', { class: 'sec', id: 'h-priv' }, icon('lock'), t('gui.about.privacy_title')),
      h('p', { text: t('gui.settings.privacy') }), h('p', { class: 'hint', text: t('gui.about.security') }))));
};

// ---------- boot ----------
async function quit() {
  showDialog(t('gui.quit.title'), h('p', { text: t('gui.quit.body') }), [closeBtn('gui.dialog.cancel', true),
    h('button', { class: 'btn danger', type: 'button', onclick: async () => {
      try { await api('/api/quit', {}); } catch (e) { /* the server is going away */ }
      dlg.close();
      clearInterval(retryTimer);
      document.body.classList.add('stopped');
      $('#main').replaceChildren(emptyState('power', t('gui.quit.stopped_title'), t('gui.quit.stopped_body')));
      $('.side').classList.add('hidden'); $('.top-tools').classList.add('hidden'); $('#offline').classList.add('hidden');
    } }, icon('power'), t('gui.quit.confirm'))]);
}

async function boot() {
  try {
    S = await api('/api/session');
  } catch (e) {
    document.body.replaceChildren(h('main', { class: 'main' }, h('h1', { class: 'page', text: 'TMAegis' }), h('p', { text: e.message })));
    return;
  }
  applyTheme(S.settings.theme);
  $('#ver').textContent = 'v' + (S.version || 'dev');
  for (const id of ['#lang', '#lang-m']) {
    const sel = $(id);
    sel.replaceChildren(...S.languages.map((l) => h('option', { value: l.tag, selected: l.tag === S.lang ? true : null, text: l.name })));
    sel.addEventListener('change', (e) => changeLang(e.target.value).catch((x) => toast(errText(x), 'err')));
  }
  $('#quit').addEventListener('click', quit);
  $('#theme').addEventListener('click', cycleTheme);
  $('#keys').addEventListener('click', showKeys);
  $('#menu').addEventListener('click', () => {
    const open = !document.body.classList.contains('nav-open');
    document.body.classList.toggle('nav-open', open);
    $('#menu').setAttribute('aria-expanded', String(open));
    if (open) $('#nav a').focus();
  });
  $('#scrim').addEventListener('click', closeNav);
  window.addEventListener('offline', () => setOffline(true));
  window.addEventListener('online', ping);
  await loadLang(S.lang);
  window.addEventListener('hashchange', route);
  route();
}
boot();
