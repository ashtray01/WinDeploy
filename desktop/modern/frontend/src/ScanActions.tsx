import { LoaderCircle, RefreshCw, X } from 'lucide-react';
import type { State } from './bridge';
import { Button } from './ui';

export default function ScanActions({ state, scan, cancel }: { state: State | null; scan: () => unknown; cancel: () => unknown }) {
  return state?.scanning ? <><span className="spinner-label"><LoaderCircle size={15} className="spin" />Чтение сведений</span><Button onClick={() => cancel()} disabled={state.cancel_requested}><X size={14} />{state.cancel_requested ? 'Отмена…' : 'Отменить'}</Button></> : <Button variant="primary" onClick={() => scan()}><RefreshCw size={14} />{state?.report ? 'Обновить данные' : 'Сканировать компьютер'}</Button>;
}
