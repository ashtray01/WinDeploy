import { useCallback, useEffect, useRef, useState } from 'react';
import { api, type State } from './bridge';

export function useInventory() {
  const [state, setState] = useState<State | null>(null);
  const [connectionError, setConnectionError] = useState('');
  const mounted = useRef(false);
  const request = useRef(0);
  const invoke = useCallback(async (call: () => Promise<State>) => {
    const id = ++request.current;
    try {
      const next = await call();
      if (mounted.current && id === request.current) { setState(next); setConnectionError(''); }
      return next;
    } catch {
      if (mounted.current && id === request.current) setConnectionError('Не удалось связаться с Go. Данные компьютера не подменяются. Повторите запрос.');
      return null;
    }
  }, []);
  const refresh = useCallback(() => invoke(api.state), [invoke]);
  const scan = useCallback(() => invoke(api.startScan), [invoke]);
  const cancel = useCallback(() => invoke(api.cancelScan), [invoke]);
  useEffect(() => {
    mounted.current = true;
    let live = true;
    void refresh().then(next => { if (live && next?.generation === 0) void scan(); });
    return () => { live = false; mounted.current = false; };
  }, [refresh, scan]);
  useEffect(() => {
    if (!state?.scanning) return;
    const timer = setTimeout(() => void refresh(), 300);
    return () => clearTimeout(timer);
  }, [state, refresh]);
  return { state, connectionError, refresh, scan, cancel };
}
