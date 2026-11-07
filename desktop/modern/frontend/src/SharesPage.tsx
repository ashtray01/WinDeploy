import { useCallback, useEffect, useState } from 'react';
import { FolderOpen, Network, RefreshCw, ShieldCheck } from 'lucide-react';
import { api, type ShareConnectionView, type ShareResultView } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel } from './ui';
import './shares.css';

type ShareForm = { id: string; unc: string; name: string; username: string; password: string; persistent: boolean; shortcut: boolean };

export default function SharesPage() {
  const [form, setForm] = useState<ShareForm>({ id: '', unc: '', name: '', username: '', password: '', persistent: false, shortcut: true });
  const [connections, setConnections] = useState<ShareConnectionView[]>([]);
  const [result, setResult] = useState<ShareResultView | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const loadConnections = useCallback(async () => {
    try {
      setConnections((await api.listShareConnections()) ?? []);
    } catch {
      setConnections([]);
    }
  }, []);
  useEffect(() => { void loadConnections(); }, [loadConnections]);

  function update<K extends keyof ShareForm>(key: K, value: ShareForm[K]) {
    setForm(previous => ({ ...previous, [key]: value }));
    setResult(null);
  }

  async function submit(dryRun: boolean, resolveConflict: boolean) {
    setBusy(true);
    setError('');
    setResult(null);
    try {
      const view = await api.addShare({ ...form, id: form.id.trim(), unc: form.unc.trim(), dry_run: dryRun, resolve_conflict: resolveConflict });
      setResult(view);
      if (view.connected || view.credential_saved) {
        setForm(previous => ({ ...previous, password: '' }));
      }
      if (view.connected || view.conflict) void loadConnections();
    } catch {
      setError('Не удалось выполнить операцию. Проверьте UNC-путь, сервер и учётные данные.');
    } finally {
      setBusy(false);
      setConfirming(false);
    }
  }

  return <>
    <PageHeading eyebrow="Сеть" title="Сетевые папки" detail="Подключение UNC-ресурсов, хранение паролей в диспетчере учётных данных Windows и ярлыки на рабочем столе." action={<Button onClick={() => void loadConnections()} disabled={busy}><RefreshCw size={15} />Обновить список</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {result?.conflict && <div className="callout red share-conflict" role="alert"><div><strong>Конфликт учётных данных (Windows Error 1219).</strong><p>{result.message}</p></div><Button variant="danger" disabled={busy} onClick={() => void submit(false, true)}>Закрыть конфликтующие сеансы и подключить</Button></div>}
    {result && !result.conflict && <div className={`callout ${result.connected ? 'blue' : ''}`} role="status">{result.message}{(result.warnings ?? []).length > 0 && <ul>{(result.warnings ?? []).map((warning, index) => <li key={index}>{warning}</li>)}</ul>}{result.shortcut_path && <p className="share-path">{result.shortcut_path}</p>}</div>}
    <div className="shares-layout">
      <Panel title={<><FolderOpen size={16} /> Новое подключение</>}>
        <div className="share-form">
          <label><span>UNC-путь</span><input value={form.unc} onChange={event => update('unc', event.target.value)} placeholder="\\\\server\\share" aria-label="UNC-путь" /></label>
          <label><span>Название</span><input value={form.name} onChange={event => update('name', event.target.value)} placeholder="Отдел продаж" aria-label="Название подключения" /></label>
          <label><span>Пользователь</span><input value={form.username} onChange={event => update('username', event.target.value)} placeholder="DOMAIN\\user" aria-label="Пользователь" autoComplete="off" /></label>
          <label><span>Пароль</span><input type="password" value={form.password} onChange={event => update('password', event.target.value)} placeholder="Необязательно" aria-label="Пароль" autoComplete="new-password" /></label>
          <label className="share-check"><input type="checkbox" checked={form.persistent} onChange={event => update('persistent', event.target.checked)} />Восстанавливать при входе</label>
          <label className="share-check"><input type="checkbox" checked={form.shortcut} onChange={event => update('shortcut', event.target.checked)} />Создать ярлык на рабочем столе</label>
        </div>
        <p className="share-note"><ShieldCheck size={13} /> Пароль передаётся только в Windows Credential Manager и не сохраняется в профилях и журналах.</p>
        {!confirming ? <div className="share-actions"><Button disabled={busy || !form.unc.trim()} onClick={() => void submit(true, false)}>Проверить</Button><Button variant="primary" disabled={busy || !form.unc.trim()} onClick={() => setConfirming(true)}>Подключить</Button></div>
          : <div className="share-confirm" role="group" aria-label="Подтверждение подключения"><p>Подключить <strong>{form.unc}</strong>{form.shortcut ? ' и создать ярлык на рабочем столе' : ''}? Пароль будет сохранён в диспетчере учётных данных Windows.</p><div className="share-actions"><Button onClick={() => setConfirming(false)} disabled={busy}>Отмена</Button><Button variant="primary" disabled={busy} onClick={() => void submit(false, false)}>{busy ? 'Подключение…' : 'Подтвердить'}</Button></div></div>}
      </Panel>
      <Panel title={<><Network size={16} /> Текущие подключения</>} action={<Badge tone="muted">{connections.length}</Badge>}>
        {connections.length === 0 ? <EmptyState title="Нет активных подключений" detail="Windows не сообщает о подключённых сетевых ресурсах." /> : <div className="network-table-wrap"><table className="network-table"><thead><tr><th scope="col">Ресурс</th><th scope="col">Локальное имя</th></tr></thead><tbody>{connections.map(connection => <tr key={connection.remote_name}><td>{connection.remote_name}</td><td>{connection.local_name || '—'}</td></tr>)}</tbody></table></div>}
      </Panel>
    </div>
  </>;
}
