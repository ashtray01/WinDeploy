import { EmptyState, PageHeading, Panel } from './ui';
import { useState } from 'react';
import Shell, { type Page } from './Shell';
import { useInventory } from './useInventory';
import type { DataStatus } from './Shell';
import ScanActions from './ScanActions';
export default function App() {
  const [page, setPage] = useState<Page>('overview');
  const { state, connectionError, scan, cancel } = useInventory();
  return <Shell page={page} navigate={setPage} status={(state?.status ?? 'idle') as DataStatus} hostname={state?.report?.system?.hostname} updated={state?.report?.finished_at}>
    <PageHeading eyebrow="Подготовка рабочих станций" title="Рабочая станция" detail="Modern · сведения из настоящего Inventory Collector" action={<ScanActions state={state} scan={scan} cancel={cancel} />} />
    {(connectionError || state?.error) && <div role="alert" className="callout">{connectionError || state?.error}</div>}
    <Panel title="Источники Inventory">{state?.report ? <div className="panel-body">{state.report.sections?.map(section => <p key={section.name}>{section.name}: {section.status}</p>)}</div> : <EmptyState title="Нет данных компьютера" detail="Ожидание чтения настоящих сведений из Windows." />}</Panel>
  </Shell>;
}
