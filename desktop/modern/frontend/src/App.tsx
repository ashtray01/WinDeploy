import { EmptyState, PageHeading, Panel } from './ui';
import { useState } from 'react';
import Shell, { type Page } from './Shell';
import { useInventory } from './useInventory';
import type { DataStatus } from './Shell';
import ScanActions from './ScanActions';
import Dashboard from './Dashboard';
export default function App() {
  const [page, setPage] = useState<Page>('overview');
  const { state, connectionError, scan, cancel } = useInventory();
  return <Shell page={page} navigate={setPage} status={(state?.status ?? 'idle') as DataStatus} hostname={state?.report?.system?.hostname} updated={state?.report?.finished_at}>
    {(connectionError || state?.error) && <div role="alert" className="callout">{connectionError || state?.error}</div>}
    {page === 'overview' ? <Dashboard state={state} navigate={setPage} scan={scan} cancel={cancel} /> : <><PageHeading eyebrow="Инвентаризация" title="Сведения компьютера" detail="Источники настоящего Inventory Collector" action={<ScanActions state={state} scan={scan} cancel={cancel} />} /><Panel title="Источники Inventory">{state?.report ? <div className="panel-body">{state.report.sections?.map(section => <p key={section.name}>{section.name}: {section.status}</p>)}</div> : <EmptyState title="Нет данных компьютера" detail="Ожидание чтения настоящих сведений из Windows." />}</Panel></>}
  </Shell>;
}
