import { useState } from 'react';
import { Card } from './Card';
import { paymentsApi } from '../api/client';
import { formatIDR, newIdempotencyKey } from '../lib/format';
import { PAYMENT_METHODS } from '../lib/constants';

const REPLAY_NOTE =
  'replay. Same key, so you got the <b>same payment back</b> and nothing new was created.';
const CREATED_NOTE =
  'created. The payment, its idempotency key and its outbox event were written in one transaction.';

export function CreatePaymentForm({ onCreated, onToast }) {
  const [form, setForm] = useState({
    customerId: 'cust_demo',
    amount: '275000',
    paymentMethod: 'va_bca',
    idempotencyKey: newIdempotencyKey(),
  });
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState(null);

  const update = (field) => (event) =>
    setForm((prev) => ({ ...prev, [field]: event.target.value }));

  async function handleSubmit(event) {
    event.preventDefault();
    setSubmitting(true);
    setResult(null);

    try {
      const { body, status } = await paymentsApi.create(form);
      const replayed = status === 200;

      setResult({ ok: true, status, note: replayed ? REPLAY_NOTE : CREATED_NOTE });
      onToast(replayed ? 'Replayed the existing payment' : 'Payment created', replayed ? '' : 'good');
      onCreated(body.id);
    } catch (err) {
      setResult({ ok: false, note: err.message });
      onToast(err.message, 'bad');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card title="Create a payment">
      <form onSubmit={handleSubmit}>
        <div className="field">
          <label htmlFor="customer">Customer</label>
          <input id="customer" value={form.customerId} onChange={update('customerId')} required />
        </div>

        <div className="field">
          <label htmlFor="amount">Amount (whole rupiah)</label>
          <input
            id="amount"
            type="number"
            min="1"
            value={form.amount}
            onChange={update('amount')}
            required
          />
          <div className="note">
            {formatIDR(form.amount)} — IDR is zero-decimal, so this is whole rupiah.
          </div>
        </div>

        <div className="field">
          <label htmlFor="method">Method</label>
          <select id="method" value={form.paymentMethod} onChange={update('paymentMethod')}>
            {PAYMENT_METHODS.map(({ value, label }) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>

        <div className="field">
          <label htmlFor="idem">Idempotency key</label>
          <div className="row">
            <input
              id="idem"
              className="mono"
              value={form.idempotencyKey}
              onChange={update('idempotencyKey')}
              required
            />
            <button
              type="button"
              className="ghost small"
              style={{ flex: 'none' }}
              onClick={() => setForm((prev) => ({ ...prev, idempotencyKey: newIdempotencyKey() }))}
            >
              New
            </button>
          </div>
          <div className="note">
            Send the same key twice &rarr; one payment, HTTP 200 instead of 201.
          </div>
        </div>

        <div className="field">
          <button type="submit" className="wide" disabled={submitting}>
            {submitting ? 'Creating…' : 'Create payment'}
          </button>
        </div>
      </form>

      {result && (
        <div
          className="explain"
          style={
            result.ok
              ? undefined
              : { background: '#fff2ef', borderColor: '#f6cfc3', color: '#8a3517' }
          }
        >
          {result.ok ? (
            <span dangerouslySetInnerHTML={{ __html: `<b>HTTP ${result.status}</b> — ${result.note}` }} />
          ) : (
            result.note
          )}
        </div>
      )}
    </Card>
  );
}
