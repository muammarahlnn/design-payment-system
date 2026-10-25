'use strict';

const API = '/api/payments';
const GW  = '/api/gateway';

const $ = (id) => document.getElementById(id);
const fmtIDR = (n) => 'Rp ' + Number(n).toLocaleString('id-ID');
const ago = (iso) => {
  const s = Math.max(0, (Date.now() - new Date(iso)) / 1000);
  if (s < 60) return Math.floor(s) + 's ago';
  if (s < 3600) return Math.floor(s / 60) + 'm ago';
  if (s < 86400) return Math.floor(s / 3600) + 'h ago';
  return Math.floor(s / 86400) + 'd ago';
};
const clock = (iso) => new Date(iso).toLocaleTimeString('en-GB');

let selected = null;
let lastRows = [];

/* ---------------- toast ---------------- */
let toastTimer;
function toast(msg, kind) {
  const t = $('toast');
  t.textContent = msg;
  t.className = 'toast show' + (kind ? ' ' + kind : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.className = 'toast'), 4200);
}

/* ---------------- http ---------------- */
async function req(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = { raw: text }; }
  if (!res.ok) {
    const msg = (body && (body.error || body.raw)) || ('HTTP ' + res.status);
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return body;
}

/* ---------------- pipeline ---------------- */
const STAGES = [
  ['API',        'payment + outbox, one transaction'],
  ['Outbox',     'event queued, waiting for the relay'],
  ['Kafka',      'payment.created, keyed by payment id'],
  ['Worker',     'consumes it, charges the gateway'],
  ['Gateway',    'accepts — customer has not paid yet'],
  ['Webhook',    'settles it SUCCEEDED or FAILED'],
];

// How far through the pipeline a status implies we are.
const REACHED = { PENDING: 2, PROCESSING: 5, UNKNOWN: 4, SUCCEEDED: 6, FAILED: 6 };

function drawPipe(status) {
  const upto = status ? (REACHED[status] || 0) : 0;
  $('pipe').innerHTML = STAGES.map((s, i) => {
    const on = i < upto;
    const cls = 'node' + (on ? ' on' : (status ? ' wait' : ''));
    return `<div class="${cls}"><div class="t">${s[0]}</div><div class="d">${s[1]}</div></div>`;
  }).join('');
}

/* ---------------- health ---------------- */
async function health() {
  const items = [
    ['payment-service', API + '/health'],
    ['fake-gateway',    GW + '/health'],
  ];
  const out = [];
  for (const [name, url] of items) {
    let ok = false;
    try { await req(url); ok = true; } catch { ok = false; }
    out.push(`<span class="dot ${ok ? 'up' : 'down'}"><i></i>${name}</span>`);
  }
  $('health').innerHTML = out.join('');
}

/* ---------------- list ---------------- */
async function loadPayments() {
  let data;
  try {
    data = await req(API + '/payments?limit=60');
  } catch (e) {
    $('liveHint').innerHTML = '<span class="err">' + e.message + '</span>';
    return;
  }
  $('liveHint').textContent = 'refreshing every 2s';

  const rows = data.payments || [];
  lastRows = rows;
  $('empty').hidden = rows.length > 0;

  $('rows').innerHTML = rows.map((p) => `
    <tr data-id="${p.id}" class="${selected === p.id ? 'sel' : ''}">
      <td><span class="pill ${p.status}">${p.status}</span></td>
      <td class="amt">${fmtIDR(p.amount)}</td>
      <td class="mono">${p.payment_method}</td>
      <td class="muted">${p.customer_id}</td>
      <td class="muted">${ago(p.created_at)}</td>
      <td style="text-align:right">${settleButtons(p)}</td>
    </tr>`).join('');

  for (const tr of $('rows').querySelectorAll('tr')) {
    tr.onclick = (ev) => {
      if (ev.target.closest('button')) return;
      select(tr.dataset.id);
    };
  }
  for (const b of $('rows').querySelectorAll('button[data-settle]')) {
    b.onclick = () => settle(b.dataset.settle, b.dataset.status, b);
  }

  // counts
  try {
    const { counts } = await req(API + '/payments/summary');
    const order = ['PENDING', 'PROCESSING', 'UNKNOWN', 'SUCCEEDED', 'FAILED'];
    $('counts').innerHTML = order.map((k) =>
      `<div class="count"><div class="n" style="color:var(--${colorVar(k)})">${counts[k] || 0}</div><div class="k">${k}</div></div>`
    ).join('');
  } catch { /* counts are cosmetic */ }

  if (selected) refreshDetail();
}

const colorVar = (s) => ({
  PENDING: 'yellow', PROCESSING: 'turquoise', UNKNOWN: 'navy',
  SUCCEEDED: 'dark-green', FAILED: 'orange',
}[s] || 'ink');

function settleButtons(p) {
  if (p.status !== 'PROCESSING' && p.status !== 'UNKNOWN') return '';
  return `<div class="flex" style="justify-content:flex-end">
    <button class="small" data-settle="${p.id}" data-status="SUCCEEDED">Pay</button>
    <button class="small ghost" data-settle="${p.id}" data-status="FAILED">Fail</button>
  </div>`;
}

/* ---------------- actions ---------------- */
async function create() {
  const btn = $('create');
  btn.disabled = true;
  $('createOut').innerHTML = '';
  try {
    const body = {
      customer_id: $('customer').value.trim(),
      amount: Number($('amount').value),
      payment_method: $('method').value,
    };
    const res = await fetch(API + '/payments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': $('idem').value.trim() },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'HTTP ' + res.status);

    const replay = res.status === 200;
    $('createOut').innerHTML =
      `<div class="explain"><b>HTTP ${res.status}</b> &mdash; ${replay
        ? 'replay. Same key, so you got the <b>same payment back</b> and nothing new was created.'
        : 'created. The payment, its idempotency key and its outbox event were written in one transaction.'}</div>`;
    toast(replay ? 'Replayed the existing payment' : 'Payment created', replay ? '' : 'good');
    select(data.id);
    loadPayments();
  } catch (e) {
    $('createOut').innerHTML = `<div class="explain" style="background:#fff2ef;border-color:#f6cfc3;color:#8a3517">${e.message}</div>`;
    toast(e.message, 'bad');
  } finally {
    btn.disabled = false;
  }
}

async function settle(id, status, btn) {
  btn.disabled = true;
  try {
    const r = await req(GW + '/charges/' + id + '/settle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    });
    toast(`Gateway sent a ${status} webhook — payment-service answered ${r.webhook_http_status}`, 'good');
    loadPayments();
  } catch (e) {
    toast(e.status === 404
      ? 'The gateway has no record of this charge (it predates the persistent store)'
      : e.message, 'bad');
  } finally {
    btn.disabled = false;
  }
}

/* ---------------- detail ---------------- */
function select(id) {
  selected = id;
  $('detailCard').hidden = false;
  refreshDetail();
  loadPayments();
}

async function refreshDetail() {
  if (!selected) return;
  let d;
  try { d = await req(API + '/payments/' + selected); }
  catch { return; }

  const p = d.payment;
  drawPipe(p.status);
  $('detailId').textContent = p.id;

  const gwRef = p.gateway_reference
    ? `<span class="mono">${p.gateway_reference}</span>`
    : '<span class="muted">none — we never got one</span>';

  $('detailBody').innerHTML = `
    <div class="flex">
      <span class="pill ${p.status}">${p.status}</span>
      <span class="amt">${fmtIDR(p.amount)}</span>
      <span class="muted">${p.payment_method} &middot; ${p.customer_id}</span>
    </div>
    <div><label>Gateway reference</label>${gwRef}</div>
    ${p.last_error ? `<div><label>Last error</label><div class="err">${p.last_error}</div></div>` : ''}
    <div>
      <label>Timeline</label>
      <ul class="tl">
        ${d.timeline.map((s) => `
          <li class="${s.done ? 'done' : ''}">
            <span class="bullet"></span>
            <span>
              <div class="s">${s.stage}</div>
              <div class="d">${s.detail}</div>
              ${s.at ? `<div class="at">${clock(s.at)}</div>` : '<div class="at muted">not yet</div>'}
            </span>
          </li>`).join('')}
      </ul>
    </div>
    ${hintFor(p)}`;
}

function hintFor(p) {
  const h = {
    PENDING:    'The relay ships the outbox event within a second, then the worker charges the gateway.',
    PROCESSING: 'The gateway has the charge. <b>Nothing moves until the customer pays</b> — press Pay or Fail on the row.',
    UNKNOWN:    'The gateway never answered, so we do not know if the money moved. Reconciliation sweeps this and asks.',
    SUCCEEDED:  'Terminal. A later webhook for this payment is accepted with 200 but ignored.',
    FAILED:     'Terminal. Either the gateway declined it or the customer never paid.',
  }[p.status];
  return h ? `<div class="explain">${h}</div>` : '';
}

/* ---------------- gateway config ---------------- */
const MODE_TEXT = {
  '':        'Charges are accepted. The payment parks at <b>PROCESSING</b> until you settle it.',
  decline:   'The gateway answers <b>402</b>. A refusal is an answer, so there is <b>no retry</b> — straight to FAILED.',
  error:     'The gateway answers <b>500</b>. That failure is known not to have charged, so the worker <b>retries 3&times;</b>, then gives up to UNKNOWN.',
  timeout:   'The gateway hangs. We never learn if money moved, so it is <b>never retried</b> — straight to UNKNOWN.',
};

async function loadConfig() {
  try {
    const c = await req(GW + '/config');
    $('failMode').value = c.fail_mode || '';
  } catch { /* gateway may be down */ }
  $('modeExplain').innerHTML = MODE_TEXT[$('failMode').value];
}

async function setConfig() {
  const mode = $('failMode').value;
  $('modeExplain').innerHTML = MODE_TEXT[mode];
  try {
    await req(GW + '/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fail_mode: mode }),
    });
    toast('Gateway mode: ' + (mode || 'normal'), 'good');
  } catch (e) { toast(e.message, 'bad'); }
}

/* ---------------- boot ---------------- */
function newKey() {
  $('idem').value = 'order-' + Math.random().toString(36).slice(2, 8) + '-' + Date.now().toString().slice(-5);
}

$('create').onclick = create;
$('newKey').onclick = newKey;
$('refresh').onclick = loadPayments;
$('failMode').onchange = setConfig;
$('closeDetail').onclick = () => { selected = null; $('detailCard').hidden = true; drawPipe(null); loadPayments(); };
$('amount').oninput = () => { $('amountHint').textContent = fmtIDR($('amount').value || 0) + ' — IDR is zero-decimal, so this is whole rupiah.'; };

newKey();
drawPipe(null);
$('amount').oninput();
loadConfig();
health();
loadPayments();
setInterval(loadPayments, 2000);
setInterval(health, 10000);
