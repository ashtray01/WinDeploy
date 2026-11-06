import { useEffect, useState } from 'react';
import { Boxes, FolderSearch, RefreshCw, ShieldAlert } from 'lucide-react';
import { api, type DriverDevice, type DriverInfo, type DriverInstallRequest, type DriverResult } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Spinner } from './ui';
import './devices.css';

function statusTone(status: string): 'green' | 'red' | 'muted' {
  if (status === 'ok') return 'green';
  if (status === 'problem') return 'red';
  return 'muted';
}

export default function DriversPage({ query }: { query: string }) {
  const [devices, setDevices] = useState<DriverDevice[]>([]);
  const [onlyProblems, setOnlyProblems] = useState(true);
  const [selected, setSelected] = useState<DriverDevice | null>(null);
  const [infPath, setInfPath] = useState('');
  const [hardwareID, setHardwareID] = useState('');
  const [catalogPath, setCatalogPath] = useState('%ProgramData%\\WinDeploy\\Drivers');
  const [catalog, setCatalog] = useState<DriverInfo[]>([]);
  const [result, setResult] = useState<DriverResult | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [scanConfirm, setScanConfirm] = useState(false);
  const [notice, setNotice] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      setDevices((await api.getDevices()) ?? []);
    } catch {
      setError('Не удалось перечислить устройства Windows.');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  function choose(device: DriverDevice) {
    setSelected(device);
    setHardwareID(device.hardware_ids?.[0] ?? '');
    setResult(null);
    setConfirming(false);
  }

  const normalized = query.trim().toLocaleLowerCase();
  const visible = devices.filter(device => {
    if (onlyProblems && device.status !== 'problem') return false;
    if (!normalized) return true;
    return `${device.name} ${device.class} ${device.manufacturer} ${(device.hardware_ids ?? []).join(' ')}`.toLocaleLowerCase().includes(normalized);
  });
  const problemCount = devices.filter(device => device.status === 'problem').length;

  async function scanCatalog() {
    setBusy(true);
    setError('');
    try {
      const packages = (await api.scanDriverCatalog(catalogPath)) ?? [];
      setCatalog(packages);
      if (packages.length === 0) setError('В каталоге не найдено пригодных INF-файлов.');
    } catch {
      setError('Каталог драйверов недоступен.');
    } finally {
      setBusy(false);
    }
  }

  async function install(dryRun: boolean) {
    setBusy(true);
    setError('');
    try {
      const request: DriverInstallRequest = { inf_path: infPath, hardware_id: hardwareID, dry_run: dryRun };
      setResult(await api.installDriver(request));
      if (!dryRun) void load();
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось установить драйвер.');
    } finally {
      setBusy(false);
      setConfirming(false);
    }
  }

  const matching = catalog.filter(pkg => (pkg.hardware_ids ?? []).some(id => id.toLowerCase() === hardwareID.trim().toLowerCase()));

  async function scanUpdates() {
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await api.scanDevicesForUpdates();
      setNotice('Поиск изменений оборудования запущен. Windows может предложить совместимые драйверы согласно политике Windows Update.');
      void load();
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось запустить поиск драйверов.');
    } finally {
      setBusy(false);
      setScanConfirm(false);
    }
  }

  return <>
    <PageHeading eyebrow="Устройства" title="Драйверы" detail="Неизвестные устройства, Hardware ID и установка проверенных INF с обязательной подписью и подтверждением." action={<div className="heading-actions"><Button onClick={() => void load()} disabled={loading || busy}><RefreshCw size={15} />Обновить</Button>{!scanConfirm ? <Button variant="primary" disabled={busy} onClick={() => setScanConfirm(true)}>Найти драйверы</Button> : <Button variant="danger" disabled={busy} onClick={() => void scanUpdates()}>Подтвердить поиск</Button>}</div>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {notice && <div className="callout blue" role="status">{notice}</div>}
    {result && <div className={`callout ${result.installed ? 'blue' : ''}`} role="status"><strong>{result.message}</strong>{result.signature?.signer && <p>Издатель: {result.signature.signer}</p>}{result.reboot_required && <p>Требуется перезагрузка Windows.</p>}</div>}
    <div className="devices-layout">
      <Panel title={<><Boxes size={16} /> Устройства</>} action={<div className="device-filters"><button className={onlyProblems ? 'active' : ''} onClick={() => setOnlyProblems(true)}>Проблемные ({problemCount})</button><button className={!onlyProblems ? 'active' : ''} onClick={() => setOnlyProblems(false)}>Все ({devices.length})</button></div>}>
        {loading ? <Spinner label="Перечисление устройств" /> : visible.length === 0 ? <EmptyState title="Устройства не найдены" detail={onlyProblems ? 'Проблемных устройств нет.' : 'Измените поисковый запрос.'} /> : <div className="device-table-wrap"><table className="device-table">
          <thead><tr><th scope="col">Устройство</th><th scope="col">Класс</th><th scope="col">Hardware ID</th><th scope="col">Драйвер</th><th scope="col">Статус</th></tr></thead>
          <tbody>{visible.map(device => <tr key={device.instance_id} className={selected?.instance_id === device.instance_id ? 'selected' : ''} onClick={() => choose(device)}>
            <td><div className="device-name"><strong>{device.name || 'Неизвестное устройство'}</strong><small>{device.instance_id}</small></div></td>
            <td>{device.class || '—'}</td>
            <td className="mono">{device.hardware_ids?.[0] || '—'}</td>
            <td>{device.driver_version ? <div className="device-name"><strong>{device.driver_version}</strong><small>{device.driver_provider || '—'}</small></div> : '—'}</td>
            <td><Badge tone={statusTone(device.status)}>{device.status === 'ok' ? 'Работает' : device.status === 'problem' ? `Проблема ${device.problem_code || ''}` : 'Неизвестно'}</Badge></td>
          </tr>)}</tbody>
        </table></div>}
      </Panel>
      <div className="devices-side">
        <Panel title="Установка драйвера">
          <div className="driver-form">
            <label><span>Hardware ID</span><input value={hardwareID} onChange={event => setHardwareID(event.target.value)} placeholder="PCI\VEN_…" aria-label="Hardware ID" /></label>
            <label><span>INF-файл</span><input value={infPath} onChange={event => setInfPath(event.target.value)} placeholder="C:\Drivers\device\driver.inf" aria-label="INF-файл" /></label>
            {selected && <p className="device-hint">Выбрано: {selected.name || selected.instance_id}</p>}
          </div>
          <div className="driver-actions"><Button disabled={busy || !infPath || !hardwareID} onClick={() => void install(true)}><ShieldAlert size={14} />Проверить</Button>{!confirming ? <Button variant="primary" disabled={busy || !infPath || !hardwareID} onClick={() => setConfirming(true)}>Установить</Button> : <Button variant="danger" disabled={busy} onClick={() => void install(false)}>Подтвердить установку</Button>}</div>
          {confirming && <p className="driver-confirm">Установка изменит драйвер устройства. Windows проверит подпись; при отказе установка будет остановлена.</p>}
        </Panel>
        <Panel title={<><FolderSearch size={16} /> Каталог драйверов</>}>
          <div className="driver-form"><label><span>Папка</span><input value={catalogPath} onChange={event => setCatalogPath(event.target.value)} aria-label="Папка драйверов" /></label></div>
          <div className="driver-actions"><Button onClick={() => void scanCatalog()} disabled={busy}>Сканировать</Button></div>
          {matching.length > 0 && <div className="driver-matches">{matching.map(pkg => <button key={pkg.path} onClick={() => setInfPath(pkg.path)}><strong>{pkg.provider || 'INF'}</strong><small>{pkg.version || 'версия не указана'} · {pkg.path}</small></button>)}</div>}
          {catalog.length > 0 && matching.length === 0 && <p className="device-hint">Найдено INF: {catalog.length}; совпадений с выбранным Hardware ID нет.</p>}
        </Panel>
      </div>
    </div>
  </>;
}
