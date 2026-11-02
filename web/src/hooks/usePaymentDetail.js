import { useCallback, useEffect, useRef, useState } from 'react';
import { paymentsApi } from '../api/client';
import { usePolling } from './usePolling';
import { POLL_MS } from '../lib/constants';

/** The selected payment's detail and timeline, polled while one is selected. */
export function usePaymentDetail(paymentId) {
  const [detail, setDetail] = useState(null);
  const abortRef = useRef(null);

  // Drop the previous payment's data immediately, so the panel never shows
  // one payment's status beside another's timeline while the fetch is in
  // flight.
  useEffect(() => {
    setDetail(null);
  }, [paymentId]);

  const refresh = useCallback(async () => {
    if (!paymentId) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      setDetail(await paymentsApi.detail(paymentId, controller.signal));
    } catch (err) {
      if (err.name !== 'AbortError') setDetail(null);
    }
  }, [paymentId]);

  usePolling(refresh, POLL_MS, Boolean(paymentId));

  useEffect(() => () => abortRef.current?.abort(), []);

  return detail;
}
