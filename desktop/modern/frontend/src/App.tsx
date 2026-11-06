import { useEffect, useState } from 'react';
import Shell, { type Page } from './Shell';
import { useInventory } from './useInventory';
import type { DataStatus } from './Shell';
import ProfilesPage from './ProfilesPage';
import Dashboard from './Dashboard';
import InventoryPages from './InventoryPages';
import JournalPage from './JournalPage';
import PackagesPage from './PackagesPage';
import NetworkPage from './NetworkPage';
import SharesPage from './SharesPage';
import DriversPage from './DriversPage';
import PrintersPage from './PrintersPage';
import BrowsersPage from './BrowsersPage';
import WindowsSettingsPage from './WindowsSettingsPage';
import ExecutionPage from './ExecutionPage';
export default function App() {
  const [page, setPage] = useState<Page>('overview');
  const [query, setQuery] = useState('');
  const navigate = (next: Page) => { setQuery(''); setPage(next); };
  const { state, connectionError, scan, cancel, refresh } = useInventory();
  useEffect(() => { if (page === 'journal') void refresh(); }, [page, refresh]);
  const inventory = page === 'apps' || page === 'network' || page === 'volumes' || page === 'system';
  return <Shell page={page} navigate={navigate} status={(state?.status ?? 'idle') as DataStatus} hostname={state?.report?.system?.hostname} updated={state?.report?.finished_at} query={query} onQuery={page === 'overview' ? undefined : setQuery}>
    {(connectionError || state?.error) && <div role="alert" className="callout">{connectionError || state?.error}</div>}
    {page === 'overview' ? <Dashboard state={state} navigate={navigate} scan={scan} cancel={cancel} /> : inventory ? <InventoryPages key={page} page={page} state={state} query={query} onQuery={setQuery} scan={scan} cancel={cancel} /> : page === 'profiles' ? <ProfilesPage query={query} onActivity={refresh} /> : page === 'packages' ? <PackagesPage query={query} /> : page === 'networkdiag' ? <NetworkPage /> : page === 'shares' ? <SharesPage /> : page === 'drivers' ? <DriversPage query={query} /> : page === 'printers' ? <PrintersPage /> : page === 'browsers' ? <BrowsersPage /> : page === 'winsettings' ? <WindowsSettingsPage /> : page === 'deploy' ? <ExecutionPage /> : <JournalPage state={state} query={query} onQuery={setQuery} refresh={refresh} />}
  </Shell>;
}
