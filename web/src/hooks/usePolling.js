import { useEffect, useRef } from 'react';

/**
 * Runs `callback` immediately and then every `intervalMs`.
 *
 * The callback is held in a ref so the interval is not torn down and rebuilt
 * on every render, and so it never fires a stale closure — the classic bug
 * when you put a changing function straight into setInterval.
 */
export function usePolling(callback, intervalMs, enabled = true) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled) return undefined;

    const tick = () => savedCallback.current();
    tick();

    const id = setInterval(tick, intervalMs);
    return () => clearInterval(id);
  }, [intervalMs, enabled]);
}
