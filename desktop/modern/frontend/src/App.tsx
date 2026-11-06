import { EmptyState, PageHeading, Panel } from './ui';
import { useState } from 'react';
import Shell, { type Page } from './Shell';
import { useInventory } from './useInventory';
import type { DataStatus } from './Shell';
import ProfilesPage from './ProfilesPage';
import Dashboard from './Dashboard';
import InventoryPages from './InventoryPages';
export default function App() {
  const [page, setPage] = useState<Page>('overview');
  const [query, setQuery] = useState('');
  const navigate = (next: Page) => { setQuery(''); setPage(next); };
  const { state, connectionError, scan, cancel, refresh } = useInventory();
  return <Shell page={page} navigate={navigate} status={(state?.status ?? 'idle') as DataStatus} hostname={state?.report?.system?.hostname} updated={state?.report?.finished_at} query={query} onQuery={page === 'overview' ? undefined : setQuery}>
    {(connectionError || state?.error) && <div role="alert" className="callout">{connectionError || state?.error}</div>}
    {page === 'overview' ? <Dashboard state={state} navigate={navigate} scan={scan} cancel={cancel} /> : page === 'apps' || page === 'network' || page === 'volumes' || page === 'system' ? <InventoryPages page={page} state={state} query={query} onQuery={setQuery} scan={scan} cancel={cancel} /> : page === 'profiles' ? <ProfilesPage query={query} onActivity={refresh} /> : <><PageHeading eyebrow="Рабочее пространство" title="Журнал" detail="Раздел подключается к настоящим данным приложения." /><Panel><EmptyState title="Источник раздела ещё не подключён" detail="Здесь не отображаются фиктивные результаты." /></Panel></>}
  </Shell>;
}
