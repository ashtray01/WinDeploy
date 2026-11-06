import { useEffect, useState } from 'react';
import { Globe, Plus, Trash2 } from 'lucide-react';
import { api, BrowserKind, BrowserScope, type BrowserBookmark, type BrowserEnvironmentView, type BrowserPolicy, type BrowserResult, type BrowserWebApp } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Spinner } from './ui';
import './settings.css';

const browserNames: Record<string, string> = { chrome: 'Google Chrome', edge: 'Microsoft Edge', firefox: 'Mozilla Firefox' };

export default function BrowsersPage() {
  const [env, setEnv] = useState<BrowserEnvironmentView | null>(null);
  const [browser, setBrowser] = useState(BrowserKind.Chrome);
  const [scope, setScope] = useState(BrowserScope.ScopeUser);
  const [homepage, setHomepage] = useState('');
  const [startup, setStartup] = useState('');
  const [bookmarks, setBookmarks] = useState<BrowserBookmark[]>([]);
  const [webapps, setWebapps] = useState<BrowserWebApp[]>([]);
  const [result, setResult] = useState<BrowserResult | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => { void api.getBrowserEnvironment().then(setEnv).catch(() => setEnv(null)); }, []);

  async function apply(dryRun: boolean) {
    setBusy(true);
    setError('');
    const policy: BrowserPolicy = {
      browser, scope, homepage: homepage.trim() || undefined,
      startup_urls: startup.split('\n').map(line => line.trim()).filter(Boolean),
      bookmarks: bookmarks.filter(bookmark => bookmark.name.trim() && bookmark.url.trim()),
      webapps: webapps.filter(app => app.name.trim() && app.url.trim()),
    };
    try {
      setResult(await api.applyBrowserPolicy(policy, dryRun));
      setConfirming(false);
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось применить политику браузера.');
    } finally {
      setBusy(false);
    }
  }

  function updateBookmark(index: number, key: 'name' | 'url', value: string) {
    setBookmarks(previous => previous.map((bookmark, current) => current === index ? { ...bookmark, [key]: value } : bookmark));
  }
  function updateWebApp(index: number, key: 'name' | 'url', value: string) {
    setWebapps(previous => previous.map((app, current) => current === index ? { ...app, [key]: value } : app));
  }

  return <>
    <PageHeading eyebrow="Браузеры" title="Политики браузеров" detail="Стартовые страницы, набор вкладок, управляемые закладки и ярлыки веб-приложений через поддерживаемые политики. Существующие профили браузеров не изменяются." action={<Button onClick={() => void apply(true)} disabled={busy}>Проверить</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {env && <div className="environment-banner"><div className="environment-item"><Globe size={15} /><span>Chrome</span><strong>{env.chrome ? 'найден' : 'не найден'}</strong></div><div className="environment-item"><Globe size={15} /><span>Edge</span><strong>{env.edge ? 'найден' : 'не найден'}</strong></div><div className="environment-item"><Globe size={15} /><span>Firefox</span><strong>{env.firefox ? 'найден' : 'не найден'}</strong></div><p>Политики записываются в реестр Windows ({scope === 'machine' ? 'HKLM, нужны права администратора' : 'HKCU'}). Профили, пароли и базы браузеров не затрагиваются.</p></div>}
    {result && <div className="callout blue" role="status">{result.dry_run ? 'Проверка выполнена: изменения не применялись.' : 'Политика применена.'}<ul>{result.applied?.map((change, index) => <li key={index}>{change.name} · {change.action}</li>)}{result.warnings?.map((warning, index) => <li key={`w-${index}`}>{warning}</li>)}{result.shortcuts?.map(shortcut => <li key={shortcut} className="mono">{shortcut}</li>)}</ul></div>}
    <div className="settings-layout">
      <Panel title={<><Globe size={16} /> Политика</>}>
        <div className="settings-form">
          <label><span>Браузер</span><select value={browser} onChange={event => setBrowser(event.target.value as BrowserKind)} aria-label="Браузер">{Object.entries(browserNames).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
          <label><span>Область</span><select value={scope} onChange={event => setScope(event.target.value as BrowserScope)} aria-label="Область политики"><option value="user">Текущий пользователь</option><option value="machine">Компьютер</option></select></label>
          <label className="wide"><span>Домашняя страница</span><input value={homepage} onChange={event => setHomepage(event.target.value)} placeholder="https://intranet.example/" aria-label="Домашняя страница" /></label>
          <label className="wide"><span>Стартовые вкладки (по одной в строке)</span><textarea value={startup} onChange={event => setStartup(event.target.value)} rows={3} placeholder="https://intranet.example/" aria-label="Стартовые вкладки" /></label>
          <fieldset className="wide"><legend>Управляемые закладки</legend>{bookmarks.map((bookmark, index) => <div className="pair" key={index}><input value={bookmark.name} onChange={event => updateBookmark(index, 'name', event.target.value)} placeholder="Название" aria-label={`Название закладки ${index + 1}`} /><input value={bookmark.url} onChange={event => updateBookmark(index, 'url', event.target.value)} placeholder="https://…" aria-label={`URL закладки ${index + 1}`} /><Button variant="ghost" className="small" onClick={() => setBookmarks(previous => previous.filter((_, current) => current !== index))}><Trash2 size={13} />Удалить</Button></div>)}<Button className="small" onClick={() => setBookmarks(previous => [...previous, { name: '', url: '' }])}><Plus size={13} />Добавить закладку</Button></fieldset>
          <fieldset className="wide"><legend>Ярлыки веб-приложений</legend>{browser === BrowserKind.Firefox ? <p className="settings-note">Firefox не поддерживает ярлыки веб-приложений в этой реализации.</p> : <>{webapps.map((app, index) => <div className="pair" key={index}><input value={app.name} onChange={event => updateWebApp(index, 'name', event.target.value)} placeholder="Название" aria-label={`Название веб-приложения ${index + 1}`} /><input value={app.url} onChange={event => updateWebApp(index, 'url', event.target.value)} placeholder="https://…" aria-label={`URL веб-приложения ${index + 1}`} /><Button variant="ghost" className="small" onClick={() => setWebapps(previous => previous.filter((_, current) => current !== index))}><Trash2 size={13} />Удалить</Button></div>)}<Button className="small" onClick={() => setWebapps(previous => [...previous, { name: '', url: '' }])}><Plus size={13} />Добавить веб-приложение</Button></>}</fieldset>
        </div>
        <div className="settings-actions">{!confirming ? <Button variant="primary" disabled={busy} onClick={() => setConfirming(true)}>Применить политику</Button> : <><span className="settings-confirm">Изменить политики {browserNames[browser]} для {scope === 'machine' ? 'компьютера' : 'пользователя'}?</span><Button variant="danger" disabled={busy} onClick={() => void apply(false)}>Подтвердить</Button><Button variant="ghost" onClick={() => setConfirming(false)} disabled={busy}>Отмена</Button></>}</div>
      </Panel>
      <Panel title="Что будет изменено"><ul className="settings-list"><li>HomepageLocation / политика домашней страницы</li><li>RestoreOnStartup + список стартовых URL (Chromium)</li><li>ManagedBookmarks / Bookmarks</li><li>Ярлыки на рабочем столе для Chrome/Edge</li></ul><p className="settings-note">Firefox не поддерживает стартовый список и веб-приложения; поддерживаемые значения будут применены, остальные отмечены предупреждением.</p></Panel>
    </div>
  </>;
}
