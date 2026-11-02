import { STAGES, STAGES_REACHED } from '../lib/constants';

/** The flow strip. Lights up however far the selected payment has reached. */
export function Pipeline({ status }) {
  const reached = status ? STAGES_REACHED[status] ?? 0 : 0;

  return (
    <div className="pipe">
      {STAGES.map((stage, index) => {
        const active = index < reached;
        const className = `node${active ? ' on' : status ? ' wait' : ''}`;
        return (
          <div key={stage.name} className={className}>
            <div className="t">{stage.name}</div>
            <div className="d">{stage.detail}</div>
          </div>
        );
      })}
    </div>
  );
}
