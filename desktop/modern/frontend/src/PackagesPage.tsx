import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { DownloadCloud, PackagePlus, RefreshCw, ShieldAlert, ShieldCheck, X } from 'lucide-react';
import { api, type PackageCatalogView, type PackageEnvironment, type PackageInstallView, type PackageView } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Progress, Spinner } from './ui';
import './packages.css';

const categoryTitles: Record<string, string> = {
  browsers: 'Браузеры', messengers: 'Мессенджеры', media: 'Медиа', utilities: 'Утилиты', printing: 'Печать', office: 'Офис', other: 'Другое',
};

function actionTone(action: string): 'blue' | 'green' | 'amber' | 'red' | 'muted' {
  switch (action) {
    case 'install': return 'blue';
    case 'update': return 'amber';
    case 'up-to-date': return 'green';
    case 'incompatible': case 'unavailable': return 'red';
    default: return 'muted';
  }
}

function statusTone(status: string): 'blue' | 'green' | 'amber' | 'red' | 'muted' {
  switch (status) {
    case 'success': case 'up-to-date': return 'green';
    case 'reboot-required': return 'amber';
    case 'failed': case 'unavailable': case 'incompatible': return 'red';
    case 'cancelled': return 'muted';
    default: return 'blue';
  }
}

function sourceLabel(pkg: PackageView): string {
  if (!pkg.source_kind) return 'Источник не найден';
  if (pkg.source_kind === 'winget') return 'WinGet';
  if (pkg.source_kind === 'http') return 'HTTPS, официальный сайт';
  return 'Локальный файл';
}

function integrityLabel(pkg: PackageView): string {
  if (pkg.integrity === 'signature') return 'Подпись издателя';
  if (pkg.integrity === 'sha256') return 'SHA-256';
  return 'Без проверки';
}

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '—';
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} КБ`;
  return `${(bytes / (1024 * 1024)).toFixed(0)} МБ`;
}

function EnvironmentBanner({ env, blocked }: { env: PackageEnvironment; blocked: boolean }) {
  return <div className={`environment-banner ${blocked ? 'blocked' : ''}`}>
    <div className="environment-item"><ShieldCheck size={15} /><span>Windows</span><strong>{env.windows} · {env.architecture}</strong></div>
    <div className="environment-item"><DownloadCloud size={15} /><span>WinGet</span><strong>{env.winget ? env.winget_version || 'доступен' : 'не найден'}</strong></div>
    <div className="environment-item"><ShieldAlert size={15} /><span>Права</span><strong>{env.elevated ? 'администратор' : 'обычный пользователь'}</strong></div>
    {blocked && <p>Для выбранных программ нужны права администратора. Перезапустите WinDeploy от имени администратора или снимите отметки с системных установщиков.</p>}
    {!env.elevated && !blocked && <p>WinDeploy запущен без прав администратора: доступна установка только пользовательских программ.</p>}
    {!env.winget && <p>WinGet не найден: используются только локальные файлы и HTTPS-источники с проверкой SHA-256.</p>}
  </div>;
}

export default function PackagesPage({ query }: { query: string }) {
  const [catalog, setCatalog] = useState<PackageCatalogView | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [install, setInstall] = useState<PackageInstallView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionError, setActionError] = useState('');
  const runningRef = useRef(false);
  const busy = install?.running ?? false;

  const reload = useCallback(async (keepSelection = false) => {
    setLoading(true);
    setError('');
    try {
      const data = await api.getPackages();
      setCatalog(data);
      const available = (data.packages ?? []).filter(p => p.action === 'install' || p.action === 'update').map(p => p.id);
      setSelected(previous => keepSelection ? previous.filter(id => available.includes(id)) : available);
    } catch {
      setCatalog(null);
      setError('Не удалось прочитать каталог программ. Повторите попытку.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void reload(); }, [reload]);

  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const next = await api.getPackageInstall();
        if (!active) return;
        setInstall(next);
        if (runningRef.current && !next.running) {
          runningRef.current = false;
          void reload(true);
        }
        runningRef.current = next.running;
      } catch {
        // A transient bridge failure must not erase the progress already shown.
      }
    };
    void poll();
    const timer = window.setInterval(poll, 700);
    return () => { active = false; window.clearInterval(timer); };
  }, [reload]);

  const packages = useMemo(() => catalog?.packages ?? [], [catalog]);
  const byId = useMemo(() => new Map(packages.map(pkg => [pkg.id, pkg])), [packages]);
  const selectedSet = useMemo(() => new Set(selected), [selected]);
  const selectable = packages.filter(pkg => pkg.action === 'install' || pkg.action === 'update');
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visible = packages.filter(pkg => !normalizedQuery || `${pkg.name} ${pkg.publisher} ${pkg.description ?? ''} ${pkg.group ?? ''}`.toLocaleLowerCase().includes(normalizedQuery));
  const groups: { title: string; items: PackageView[] }[] = [];
  visible.forEach(pkg => {
    const title = pkg.group || categoryTitles[pkg.category] || 'Другое';
    const group = groups.find(item => item.title === title);
    if (group) group.items.push(pkg); else groups.push({ title, items: [pkg] });
  });
  const blocked = !!catalog && !catalog.environment.elevated && selected.some(id => byId.get(id)?.requires_admin);
  const allSelected = selectable.length > 0 && selectable.every(pkg => selectedSet.has(pkg.id));
  const progress = install && install.total > 0 ? Math.round((install.completed / install.total) * 100) : 0;

  function toggle(id: string) {
    setSelected(previous => previous.includes(id) ? previous.filter(item => item !== id) : [...previous, id]);
  }

  async function startInstall() {
    setActionError('');
    try {
      const view = await api.startPackageInstall(selected);
      setInstall(view);
      runningRef.current = view.running;
    } catch {
      setActionError('Не удалось запустить установку. Проверьте выбранные программы и права администратора.');
    }
  }

  async function cancelInstall() {
    try {
      const view = await api.cancelPackageInstall();
      setInstall(view);
    } catch {
      setActionError('Не удалось запросить отмену установки.');
    }
  }

  return <>
    <PageHeading eyebrow="Установка программ" title="Каталог программ" detail="Проверенный каталог WinGet, официальных HTTPS-источников и локальных установщиков. Перед запуском проверяются SHA-256, подпись издателя и состояние системы." action={<Button onClick={() => void reload(true)} disabled={loading || busy}><RefreshCw size={15} />Проверить заново</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {actionError && <div className="callout red" role="alert">{actionError}</div>}
    {catalog && <EnvironmentBanner env={catalog.environment} blocked={blocked} />}
    {catalog?.warnings?.map((warning, index) => <div key={index} className="callout">{warning === 'target OS or architecture is unknown; compatibility cannot be confirmed' ? 'Не удалось определить версию Windows и архитектуру: совместимость программ не подтверждена.' : warning}</div>)}
    {loading ? <Spinner label="Проверка установленных программ" /> : <div className={`packages-layout ${install ? 'with-install' : ''}`}>
      <Panel className="packages-panel" title={<><PackagePlus size={16} /> Каталог программ</>} action={!busy && <div className="package-toolbar">
        <label className="select-all"><input type="checkbox" checked={allSelected} onChange={event => setSelected(event.target.checked ? selectable.map(pkg => pkg.id) : [])} />Выбрать все</label>
        <span className="selected-count">Выбрано {selected.length} из {packages.length}</span>
        <Button variant="primary" disabled={!selected.length || blocked} onClick={() => void startInstall()}><DownloadCloud size={15} />Установить {selected.length ? `(${selected.length})` : ''}</Button>
      </div>}>
        {packages.length === 0 ? <EmptyState title="Каталог пуст" detail="Встроенный каталог не содержит программ." /> : <div className="package-table-wrap"><table className="package-table">
          <thead><tr><th scope="col" className="check-col"><span className="visually-hidden">Выбор</span></th><th scope="col">Программа</th><th scope="col">Состояние</th><th scope="col">Источник</th><th scope="col" className="size-col">Размер</th></tr></thead>
          <tbody>
            {groups.map(group => <Fragment key={`group-${group.title}`}>
              <tr className="group-row"><th colSpan={5} scope="colgroup">{group.title}</th></tr>
              {group.items.map(pkg => {
                const selectableRow = pkg.action === 'install' || pkg.action === 'update';
                return <tr key={pkg.id} className={pkg.installed ? 'installed' : ''}>
                  <td className="check-col"><input type="checkbox" aria-label={`Выбрать ${pkg.name}`} disabled={!selectableRow || busy} checked={selectedSet.has(pkg.id)} onChange={() => toggle(pkg.id)} /></td>
                  <td><div className="package-name"><strong>{pkg.name}</strong>{pkg.requires_admin && <ShieldAlert size={13} aria-label="Требуются права администратора" className="admin-icon" />}<small>{pkg.publisher}</small>{pkg.description && <p>{pkg.description}</p>}</div></td>
                  <td><div className="package-state"><Badge tone={actionTone(pkg.action)}>{pkg.action_label}</Badge><small>{pkg.installed ? `Версия ${pkg.installed_version || 'не определена'}` : pkg.reason}</small></div></td>
                  <td><div className="package-source"><span>{sourceLabel(pkg)}</span><small>{pkg.source_version ? `Версия ${pkg.source_version} · ` : ''}{integrityLabel(pkg)}</small></div></td>
                  <td className="size-col">{formatSize(pkg.size_bytes)}</td>
                </tr>;
              })}
            </Fragment>)}
            {visible.length === 0 && <tr><td colSpan={5}><EmptyState title="Ничего не найдено" detail="Измените поисковый запрос." /></td></tr>}
          </tbody>
        </table></div>}
      </Panel>
      {install && <Panel title={<><DownloadCloud size={16} /> Ход установки</>} action={install.running ? <Button variant="danger" className="small" onClick={() => void cancelInstall()} disabled={install.cancel_requested}><X size={14} />{install.cancel_requested ? 'Отмена…' : 'Отменить'}</Button> : <Badge tone={install.cancelled ? 'muted' : install.reboot_required ? 'amber' : 'green'}>{install.cancelled ? 'Отменено' : install.reboot_required ? 'Нужна перезагрузка' : 'Завершено'}</Badge>}>
        <div className="install-progress">
          <div className="install-summary"><div><strong>{install.running ? (install.current_name || install.current || 'Подготовка…') : install.message || 'Установка не запускалась'}</strong><small>{install.running ? `Обработано ${install.completed} из ${install.total}` : install.message}</small></div><span className="install-percent">{progress}%</span></div>
          <Progress value={install.running ? progress : install.cancelled ? progress : 100} label="Прогресс установки" />
          {install.reboot_required && <div className="callout">Установка выполнена, но Windows требует перезагрузку. Продолжите после перезагрузки.</div>}
        </div>
        {(install.events ?? []).length > 0 && <div className="install-events" aria-live="polite">{(install.events ?? []).slice(-60).map((event, index) => <div key={`${event.time}-${index}`} className={`install-event ${event.level}`}><time>{new Date(event.time).toLocaleTimeString('ru-RU')}</time><span>{event.package_name && <b>{event.package_name}: </b>}{event.message}</span></div>)}</div>}
        {(install.results ?? []).length > 0 && <div className="install-results"><table className="package-table compact"><thead><tr><th scope="col">Программа</th><th scope="col">Результат</th><th scope="col">Было</th><th scope="col">Стало</th></tr></thead><tbody>
          {(install.results ?? []).map(result => <tr key={result.package_id}><td>{result.name || result.package_id}</td><td><div className="result-cell"><Badge tone={statusTone(result.status)}>{result.status_label}</Badge>{result.error && <small>{result.error}</small>}{result.exit_code ? <small>Код выхода: {result.exit_code}</small> : null}</div></td><td>{result.installed_before || '—'}</td><td>{result.installed_after || (result.reboot_required ? 'после перезагрузки' : '—')}</td></tr>)}
        </tbody></table></div>}
      </Panel>}
    </div>}
  </>;
}
