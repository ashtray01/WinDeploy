import { EmptyState, PageHeading, Panel } from './ui';
import { useState } from 'react';
import Shell, { type Page } from './Shell';
export default function App() {
  const [page, setPage] = useState<Page>('overview');
  return <Shell page={page} navigate={setPage}><PageHeading eyebrow="Подготовка рабочих станций" title="Рабочая станция" detail="Modern · режим просмотра" /><Panel title="Локальный компьютер"><EmptyState title="Источник данных ещё не подключён" detail="Интерфейс не показывает результаты до получения настоящего Inventory." /></Panel></Shell>;
}
