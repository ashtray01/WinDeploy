import { useEffect, useState } from 'react';
import { Printer, RefreshCw, ShieldAlert } from 'lucide-react';
import { api, type PrinterInfo, type PrinterAddRequest, type PrinterResult } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Spinner } from './ui';
import './devices.css';

export default function PrintersPage() {
  const [items, setItems] = useState<PrinterInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [kyocera, setKyocera] = useState(false);
  const [models, setModels] = useState<string[]>([]);
  const [form, setForm] = useState<PrinterAddRequest>({ name: '', inf_path: '', model: '', ip_address: '', share: false, share_name: '', set_default: true, dry_run: false });
  const [testFor, setTestFor] = useState('');
  const [confirmAdd, setConfirmAdd] = useState(false);

  async function load() {
    setLoading(true);
    setError('');
    try {
      setItems((await api.getPrinters()) ?? []);
    } catch {
      setError('Не удалось перечислить принтеры Windows.');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); void api.getKyocera2460Models().then(models => setModels(models ?? [])).catch(() => setModels([])); }, []);

  function update<K extends keyof PrinterAddRequest>(key: K, value: PrinterAddRequest[K]) {
    setForm(previous => ({ ...previous, [key]: value }));
    setConfirmAdd(false);
  }

  async function add(dryRun: boolean) {
    setBusy(true);
    setError('');
    try {
      const result: PrinterResult = await api.addPrinter({ ...form, model: kyocera ? form.model : form.model.trim(), dry_run: dryRun });
      setMessage(result.message + (result.default ? ' Назначен принтером по умолчанию.' : ''));
      if (!dryRun) { void load(); setConfirmAdd(false); }
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось добавить принтер.');
    } finally {
      setBusy(false);
    }
  }

  async function setDefault(name: string) {
    setBusy(true);
    setError('');
    try {
      await api.setDefaultPrinter(name);
      setMessage(`Принтер «${name}» назначен по умолчанию.`);
      void load();
    } catch {
      setError('Не удалось изменить принтер по умолчанию.');
    } finally {
      setBusy(false);
    }
  }

  async function printTest(name: string) {
    setBusy(true);
    setError('');
    try {
      await api.printTestPage(name);
      setMessage(`Тестовая страница отправлена на «${name}».`);
      setTestFor('');
    } catch {
      setError('Не удалось отправить тестовую страницу.');
    } finally {
      setBusy(false);
    }
  }

  return <>
    <PageHeading eyebrow="Печать" title="Принтеры" detail="Сетевые принтеры, общие Windows-принтеры, выбор по умолчанию и тестовая страница. Экспериментально в public beta: TCP/IP-порт может не создаваться, VM-приёмка не выполнена." action={<Button onClick={() => void load()} disabled={busy}><RefreshCw size={15} />Обновить</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {message && <div className="callout blue" role="status">{message}</div>}
    <div className="devices-layout">
      <Panel title={<><Printer size={16} /> Установленные принтеры</>} action={<Badge tone="muted">{items.length}</Badge>}>
        {loading ? <Spinner label="Перечисление принтеров" /> : items.length === 0 ? <EmptyState title="Принтеры не найдены" detail="Windows не сообщает об установленных очередях печати." /> : <div className="device-table-wrap"><table className="device-table printer-table">
          <thead><tr><th scope="col">Принтер</th><th scope="col">Драйвер</th><th scope="col">Порт</th><th scope="col">Статус</th><th scope="col">Действия</th></tr></thead>
          <tbody>{items.map(printer => <tr key={printer.name}>
            <td><div className="device-name"><strong>{printer.name}</strong><small>{printer.share_name ? `Общий: ${printer.share_name}` : printer.server || 'Локальный'}</small></div></td>
            <td>{printer.driver || '—'}</td>
            <td className="mono">{printer.port || '—'}</td>
            <td><div className="printer-status"><Badge tone={printer.default ? 'blue' : printer.status === 'ok' ? 'green' : 'amber'}>{printer.default ? 'По умолчанию' : printer.status === 'ok' ? 'Готов' : printer.status === 'offline' ? 'Не в сети' : 'Внимание'}</Badge></div></td>
            <td><div className="row-actions"><Button className="small" disabled={busy || printer.default} onClick={() => void setDefault(printer.name)}>По умолчанию</Button>{testFor === printer.name ? <><Button variant="danger" className="small" disabled={busy} onClick={() => void printTest(printer.name)}>Подтвердить печать</Button><Button variant="ghost" className="small" onClick={() => setTestFor('')}>Отмена</Button></> : <Button className="small" disabled={busy} onClick={() => setTestFor(printer.name)}>Тестовая страница</Button>}</div></td>
          </tr>)}</tbody>
        </table></div>}
      </Panel>
      <div className="devices-side">
        <Panel title="Добавить принтер">
          <div className="driver-form">
            <label><span>Имя</span><input value={form.name} onChange={event => update('name', event.target.value)} placeholder="Принтер отдела" aria-label="Имя принтера" /></label>
            <label><span>IP-адрес</span><input value={form.ip_address} onChange={event => update('ip_address', event.target.value)} placeholder="192.0.2.50" aria-label="IP-адрес принтера" /></label>
            <label><span>INF-драйвер</span><input value={form.inf_path} onChange={event => update('inf_path', event.target.value)} placeholder="C:\Drivers\printer\driver.inf" aria-label="INF-драйвер принтера" /></label>
            <label className="share-check"><input type="checkbox" checked={kyocera} onChange={event => { setKyocera(event.target.checked); update('model', event.target.checked ? (models[0] ?? '') : ''); }} />Kyocera 2460</label>
            {kyocera ? <label><span>Точная модель</span><select value={form.model} onChange={event => update('model', event.target.value)} aria-label="Модель Kyocera">{models.map(model => <option key={model} value={model}>{model}</option>)}</select></label>
              : <label><span>Модель драйвера</span><input value={form.model} onChange={event => update('model', event.target.value)} placeholder="Например, Generic / Text Only" aria-label="Модель драйвера" /></label>}
            <label className="share-check"><input type="checkbox" checked={form.share} onChange={event => update('share', event.target.checked)} />Общий доступ</label>
            {form.share && <label><span>Имя общего ресурса</span><input value={form.share_name} onChange={event => update('share_name', event.target.value)} aria-label="Имя общего ресурса" /></label>}
            <label className="share-check"><input type="checkbox" checked={form.set_default} onChange={event => update('set_default', event.target.checked)} />Сделать по умолчанию</label>
          </div>
          <p className="device-hint"><ShieldAlert size={12} /> Добавление принтера требует прав администратора. Для Kyocera 2460 модель выбирается только явно.</p>
          <div className="driver-actions"><Button disabled={busy || !form.name || !form.inf_path || !form.ip_address || !form.model} onClick={() => void add(true)}>Проверить</Button>{!confirmAdd ? <Button variant="primary" disabled={busy || !form.name || !form.inf_path || !form.ip_address || !form.model} onClick={() => setConfirmAdd(true)}>Добавить</Button> : <Button variant="danger" disabled={busy} onClick={() => void add(false)}>Подтвердить добавление</Button>}</div>
          {confirmAdd && <p className="driver-confirm">Будет создан TCP/IP-порт, установлен драйвер и добавлена очередь «{form.name}».</p>}
        </Panel>
      </div>
    </div>
  </>;
}
