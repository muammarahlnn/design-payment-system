import { useCallback, useRef, useState } from 'react';
import { paymentsApi } from '../api/client';
import { usePolling } from './usePolling';
import { POLL_MS } from '../lib/constants';

/**
 * The payments list and its status counts, refreshed on an interval.
 *
 * Each refresh aborts the previous one. Without that, a slow response can land
 * after a newer one and overwrite fresher data with stale rows.
 */
export function usePayments() {
  const [payments, setPayments] = useState([]);
  const [counts, setCounts] = useState({});
  const [error, setError] = useState(null);
  const [loaded, setLoaded] = useState(false);
  const abortRef = useRef(null);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const [list, summary] = await Promise.all([
        paymentsApi.list(60, controller.signal),
        paymentsApi.summary(controller.signal),
      ]);
      setPayments(list);
      setCounts(summary);
      setError(null);
    } catch (err) {
      if (err.name === 'AbortError') return;
      setError(err.message);
    } finally {
      if (!controller.signal.aborted) setLoaded(true);
    }
  }, []);

  usePolling(refresh, POLL_MS);

  return { payments, counts, error, loaded, refresh };
}
