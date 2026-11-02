import { StatusPill } from './StatusPill';
import { formatIDR, timeAgo } from '../lib/format';

const SETTLEABLE = new Set(['PROCESSING', 'UNKNOWN']);

function SettleButtons({ payment, onSettle, busyId }) {
  if (!SETTLEABLE.has(payment.status)) return null;

  const busy = busyId === payment.id;
  return (
    <div className="flex" style={{ justifyContent: 'flex-end' }}>
      <button className="small" disabled={busy} onClick={() => onSettle(payment.id, 'SUCCEEDED')}>
        Pay
      </button>
      <button className="small ghost" disabled={busy} onClick={() => onSettle(payment.id, 'FAILED')}>
        Fail
      </button>
    </div>
  );
}

export function PaymentsTable({ payments, selectedId, onSelect, onSettle, settlingId }) {
  return (
    <>
      <div style={{ overflowX: 'auto' }}>
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Amount</th>
              <th>Method</th>
              <th>Customer</th>
              <th>Created</th>
              <th aria-label="actions" />
            </tr>
          </thead>
          <tbody>
            {payments.map((payment) => (
              <tr
                key={payment.id}
                className={selectedId === payment.id ? 'sel' : ''}
                onClick={() => onSelect(payment.id)}
              >
                <td>
                  <StatusPill status={payment.status} />
                </td>
                <td className="amt">{formatIDR(payment.amount)}</td>
                <td className="mono">{payment.payment_method}</td>
                <td className="muted">{payment.customer_id}</td>
                <td className="muted">{timeAgo(payment.created_at)}</td>
                {/* Stop a button click from also selecting the row. */}
                <td style={{ textAlign: 'right' }} onClick={(e) => e.stopPropagation()}>
                  <SettleButtons payment={payment} onSettle={onSettle} busyId={settlingId} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {payments.length === 0 && (
        <div className="empty">No payments yet. Create one on the left.</div>
      )}
    </>
  );
}
