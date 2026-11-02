import { Card } from './Card';
import { StatusPill } from './StatusPill';
import { formatIDR, clockTime } from '../lib/format';
import { STATUS_HINT } from '../lib/constants';

function Timeline({ steps }) {
  return (
    <ul className="tl">
      {steps.map((step) => (
        <li key={step.stage} className={step.done ? 'done' : ''}>
          <span className="bullet" />
          <span>
            <div className="s">{step.stage}</div>
            <div className="d">{step.detail}</div>
            {step.at ? (
              <div className="at">{clockTime(step.at)}</div>
            ) : (
              <div className="at muted">not yet</div>
            )}
          </span>
        </li>
      ))}
    </ul>
  );
}

export function PaymentDetail({ detail, paymentId, onClose }) {
  const payment = detail?.payment;

  return (
    <Card
      title="Payment detail"
      hint={paymentId}
      actions={
        <button className="ghost small" onClick={onClose}>
          Close
        </button>
      }
    >
      {!payment ? (
        <div className="muted">Loading…</div>
      ) : (
        <div className="stack">
          <div className="flex">
            <StatusPill status={payment.status} />
            <span className="amt">{formatIDR(payment.amount)}</span>
            <span className="muted">
              {payment.payment_method} &middot; {payment.customer_id}
            </span>
          </div>

          <div>
            <label>Gateway reference</label>
            {payment.gateway_reference ? (
              <span className="mono">{payment.gateway_reference}</span>
            ) : (
              <span className="muted">none — we never got one</span>
            )}
          </div>

          {payment.last_error && (
            <div>
              <label>Last error</label>
              <div className="err">{payment.last_error}</div>
            </div>
          )}

          <div>
            <label>Timeline</label>
            <Timeline steps={detail.timeline} />
          </div>

          {STATUS_HINT[payment.status] && (
            <div
              className="explain"
              dangerouslySetInnerHTML={{ __html: STATUS_HINT[payment.status] }}
            />
          )}
        </div>
      )}
    </Card>
  );
}
