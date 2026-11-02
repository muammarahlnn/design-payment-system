import { useCallback, useState } from 'react';
import { TopBar } from '../components/TopBar';
import { Card } from '../components/Card';
import { Pipeline } from '../components/Pipeline';
import { CountsRow } from '../components/CountsRow';
import { PaymentsTable } from '../components/PaymentsTable';
import { CreatePaymentForm } from '../components/CreatePaymentForm';
import { GatewayConfigPanel } from '../components/GatewayConfigPanel';
import { PaymentDetail } from '../components/PaymentDetail';
import { Toast } from '../components/Toast';
import { usePayments } from '../hooks/usePayments';
import { usePaymentDetail } from '../hooks/usePaymentDetail';
import { useToast } from '../hooks/useToast';
import { gatewayApi } from '../api/client';

const NO_CHARGE_HINT =
  'The gateway has no record of this charge (it predates the persistent store)';

export function Playground() {
  const { payments, counts, error, refresh } = usePayments();
  const [selectedId, setSelectedId] = useState(null);
  const [settlingId, setSettlingId] = useState(null);
  const detail = usePaymentDetail(selectedId);
  const { toast, show } = useToast();

  const settle = useCallback(
    async (paymentId, status) => {
      setSettlingId(paymentId);
      try {
        const result = await gatewayApi.settle(paymentId, status);
        show(
          `Gateway sent a ${status} webhook — payment-service answered ${result.webhook_http_status}`,
          'good',
        );
        refresh();
      } catch (err) {
        show(err.status === 404 ? NO_CHARGE_HINT : err.message, 'bad');
      } finally {
        setSettlingId(null);
      }
    },
    [refresh, show],
  );

  const handleCreated = useCallback(
    (paymentId) => {
      setSelectedId(paymentId);
      refresh();
    },
    [refresh],
  );

  return (
    <>
      <TopBar subtitle="design-payment-system · outbox → Kafka → worker → gateway → webhook" />

      <div className="wrap">
        <div style={{ marginBottom: 20 }}>
          <Card title="The flow" hint="pick a payment below to light up where it is">
            <Pipeline status={detail?.payment?.status} />
          </Card>
        </div>

        <div className="grid">
          <div className="stack">
            <CreatePaymentForm onCreated={handleCreated} onToast={show} />
            <GatewayConfigPanel onToast={show} />
          </div>

          <div className="stack">
            <Card
              title="Payments"
              hint={error ? undefined : 'refreshing every 2s'}
              actions={
                <button className="ghost small" onClick={refresh}>
                  Refresh
                </button>
              }
              bodyless
            >
              {error && (
                <div className="body">
                  <span className="err">{error}</span>
                </div>
              )}
              <div className="body">
                <CountsRow counts={counts} />
              </div>
              <PaymentsTable
                payments={payments}
                selectedId={selectedId}
                onSelect={setSelectedId}
                onSettle={settle}
                settlingId={settlingId}
              />
            </Card>

            {selectedId && (
              <PaymentDetail
                detail={detail}
                paymentId={selectedId}
                onClose={() => setSelectedId(null)}
              />
            )}
          </div>
        </div>
      </div>

      <Toast toast={toast} />
    </>
  );
}
