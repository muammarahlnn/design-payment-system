import { STATUS_COLOR_VAR, STATUS_ORDER } from '../lib/constants';

export function CountsRow({ counts }) {
  return (
    <div className="counts">
      {STATUS_ORDER.map((status) => (
        <div key={status} className="count">
          <div className="n" style={{ color: `var(--${STATUS_COLOR_VAR[status]})` }}>
            {counts[status] ?? 0}
          </div>
          <div className="k">{status}</div>
        </div>
      ))}
    </div>
  );
}
