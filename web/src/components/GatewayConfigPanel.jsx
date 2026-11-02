import { Card } from './Card';
import { useGatewayConfig } from '../hooks/useGatewayConfig';
import { FAIL_MODES, FAIL_MODE_TEXT } from '../lib/constants';

export function GatewayConfigPanel({ onToast }) {
  const { failMode, changeFailMode } = useGatewayConfig((message) => onToast(message, 'bad'));

  async function handleChange(event) {
    const mode = event.target.value;
    try {
      await changeFailMode(mode);
      onToast(`Gateway mode: ${mode || 'normal'}`, 'good');
    } catch {
      /* the hook already rolled the value back and reported it */
    }
  }

  return (
    <Card title="Gateway behaviour" hint="applies to the next charge">
      <div className="field">
        <label htmlFor="failMode">Fail mode</label>
        <select id="failMode" value={failMode} onChange={handleChange}>
          {FAIL_MODES.map(({ value, label }) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </div>
      <div className="explain" dangerouslySetInnerHTML={{ __html: FAIL_MODE_TEXT[failMode] }} />
    </Card>
  );
}
