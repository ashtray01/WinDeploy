import { useState } from 'react';
import { RefreshCw } from 'lucide-react';
import DataTable, { type Column } from './DataTable';
import { type Entry, type State } from './bridge';
import { Badge, Button, PageHeading, Panel } from './ui';

const sources: Record<string, string> = { application: 'Приложение', inventory: 'Инвентаризация', profiles: 'Профили', system: 'Система', cpu: 'Процессор', memory: 'Память', volumes: 'Накопители', interfaces: 'Интерфейсы', network_config: 'Конфигурация сети', apps: 'Программы' };
const columns: Column<Entry>[] = [
  { key: 'id', label: '№', value: e => e.id },
  { key: 'time', label: 'Время', value: e => e.time, render: e => <time dateTime={e.time}>{new Date(e.time).toLocaleTimeString('ru-RU')}</time> },
  { key: 'level', label: 'Уровень', value: e => e.level === 'warning' ? 'Предупреждение' : e.level === 'error' ? 'Ошибка' : 'Информация', render: e => <Badge tone={e.level === 'warning' ? 'amber' : e.level === 'error' ? 'red' : 'blue'}>{e.level === 'warning' ? 'Предупреждение' : e.level === 'error' ? 'Ошибка' : 'Информация'}</Badge> },
  { key: 'source', label: 'Источник', value: e => sources[e.source] ?? e.source },
  { key: 'message', label: 'Сообщение', value: e => e.message },
];
export default function JournalPage({ state, query, onQuery, refresh }: { state: State | null; query: string; onQuery: (query: string) => void; refresh: () => Promise<unknown> }) {
  const [busy, setBusy] = useState(false);
  return <><PageHeading eyebrow="Диагностика приложения" title="Журнал" detail="Настоящие события текущего процесса. Последние 200 записей в памяти; журнал не сохраняется на диск." action={<Button disabled={busy} onClick={async () => { setBusy(true); try { await refresh(); } finally { setBusy(false); } }}><RefreshCw size={15} />Обновить журнал</Button>} /><Panel><DataTable rows={state?.journal ?? []} columns={columns} rowKey={e => String(e.id)} query={query} onQuery={onQuery} emptyTitle="Журнал пока пуст" /></Panel></>;
}
