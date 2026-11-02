import { useCallback, useEffect, useRef, useState } from 'react';

const VISIBLE_MS = 4200;

/** One transient message at a time, auto-dismissed. */
export function useToast() {
  const [toast, setToast] = useState(null);
  const timerRef = useRef(null);

  const show = useCallback((message, kind = '') => {
    clearTimeout(timerRef.current);
    setToast({ message, kind });
    timerRef.current = setTimeout(() => setToast(null), VISIBLE_MS);
  }, []);

  useEffect(() => () => clearTimeout(timerRef.current), []);

  return { toast, show };
}
