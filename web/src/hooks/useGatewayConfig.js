import { useCallback, useEffect, useState } from 'react';
import { gatewayApi } from '../api/client';

/** The gateway's current fail mode, read once and settable. */
export function useGatewayConfig(onError) {
  const [failMode, setFailMode] = useState('');

  useEffect(() => {
    const controller = new AbortController();

    gatewayApi
      .getConfig(controller.signal)
      .then((config) => setFailMode(config.fail_mode || ''))
      .catch(() => {
        /* the gateway may be down; the dropdown still works */
      });

    return () => controller.abort();
  }, []);

  const changeFailMode = useCallback(
    async (mode) => {
      const previous = failMode;
      setFailMode(mode); // optimistic: the explanation text updates instantly
      try {
        await gatewayApi.setConfig(mode);
        return mode;
      } catch (err) {
        setFailMode(previous); // roll back so the UI never lies about the gateway
        onError?.(err.message);
        throw err;
      }
    },
    [failMode, onError],
  );

  return { failMode, changeFailMode };
}
