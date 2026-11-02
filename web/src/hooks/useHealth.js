import { useCallback, useState } from 'react';
import { gatewayApi, paymentsApi } from '../api/client';
import { usePolling } from './usePolling';
import { HEALTH_POLL_MS } from '../lib/constants';

const SERVICES = [
  { name: 'payment-service', check: paymentsApi.health },
  { name: 'fake-gateway', check: gatewayApi.health },
];

/** Up/down for each service, as shown in the header. */
export function useHealth() {
  const [services, setServices] = useState(
    SERVICES.map(({ name }) => ({ name, up: null })),
  );

  const refresh = useCallback(async () => {
    const next = await Promise.all(
      SERVICES.map(async ({ name, check }) => {
        try {
          await check();
          return { name, up: true };
        } catch {
          return { name, up: false };
        }
      }),
    );
    setServices(next);
  }, []);

  usePolling(refresh, HEALTH_POLL_MS);

  return services;
}
