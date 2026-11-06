import { useEffect, useState } from 'react';
import { MonitorCog, PlugZap, ShieldCheck, SlidersHorizontal } from 'lucide-react';
import { api, type PowerPlanOption, type SettingsResult, type UpdateReport, type WindowsSettingsView } from './bridge';
import { Badge, Button, PageHeading, Panel, Spinner } from './ui';
import './settings.css';

type Pending = { label: string; run: () => PromiseLike<SettingsResult> } | null;

export default function WindowsSettingsPage() {
  const [view, setView] = useState<WindowsSettingsView | null>(null);
  const [plans, setPlans] = useState<PowerPlanOption[]>([]);
  const [name, setName] = useState('');
  const [timezone, setTimezone] = useState('');
  const [showHidden, setShowHidden] = useState(false);
  const [showExtensions, setShowExtensions] = useState(true);
  const [showSuperHidden, setShowSuperHidden] = useState(false);
  const [openThisPC, setOpenThisPC] = useState(true);
  const [power, setPower] = useState('');
  const [diagnostics, setDiagnostics] = useState<UpdateReport | null>(null);
  const [result, setResult] = useState<SettingsResult | null>(null);
  const [pending, setPending] = useState<Pending>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load() {
    setLoading(true);
    setError('');
    try {
      const data = await api.getWindowsSettings();
      setView(data);
      setName(data.hostname ?? '');
      setTimezone(data.timezone ?? '');
      setShowHidden(data.explorer?.['Hidden'] === '1');
      setShowExtensions(data.explorer?.['HideFileExt'] === '0');
      setShowSuperHidden(data.explorer?.['ShowSuperHidden'] === '1');
      setOpenThisPC(data.explorer?.['LaunchTo'] === '1');
      setPower(data.power_plan ?? '');
      setPlans((await api.getPowerPlanOptions()) ?? []);
    } catch {
      setError('Не удалось прочитать параметры Windows.');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { void load(); }, []);

  async function preview(run: () => PromiseLike<SettingsResult>) {
    setBusy(true); setError('');
    try { setResult(await run()); } catch (caught) { setError(typeof caught === 'string' ? caught : 'Не удалось выполнить проверку.'); }
    finally { setBusy(false); }
  }

  async function apply(run: () => PromiseLike<SettingsResult>) {
    setBusy(true); setError('');
    try {
      setResult(await run());
      setPending(null);
      await load();
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось применить настройку.');
    } finally { setBusy(false); }
  }

  async function runDiagnostics() {
    setBusy(true); setError('');
    try { setDiagnostics(await api.getWindowsUpdateDiagnostics()); } catch { setError('Не удалось выполнить диагностику Windows Update.'); }
    finally { setBusy(false); }
  }

  const namePreview = () => preview(() => api.applyComputerName(name, true));
  const timezonePreview = () => preview(() => api.applyTimezone(timezone, true));
  const explorerPreview = () => preview(() => api.applyExplorer({ show_hidden: showHidden, show_extensions: showExtensions, show_super_hidden: showSuperHidden, open_this_pc: openThisPC }, true));
  const powerPreview = () => preview(() => api.applyPowerPlan(power, true));

  return <>
    <PageHeading eyebrow="Система" title="Настройка Windows" detail="Имя компьютера, часовой пояс, Проводник, электропитание и диагностика Windows Update. Каждое изменение сначала проверяется, затем подтверждается." action={<Button onClick={() => void load()} disabled={loading || busy}>Обновить</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {result && <div className={`callout ${result.applied ? 'blue' : ''}`} role="status">{result.message}{result.warnings?.length ? <ul>{result.warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul> : null}{result.changes?.length ? <ul>{result.changes.map((change, index) => <li key={index}>{change.name}: {change.old_value || '—'} → {change.new_value} ({change.action})</li>)}</ul> : null}</div>}
    {loading ? <Spinner label="Чтение параметров Windows" /> : view && <div className="settings-layout">
      <Panel title={<><ShieldCheck size={16} /> Система и лицензирование</>}>
        <div className="settings-summary">
          <div><small>Windows</small><strong>{view.windows}</strong></div>
          <div><small>Компьютер</small><strong>{view.hostname || '—'}</strong></div>
          <div><small>Права</small><strong>{view.elevated ? 'администратор' : 'обычный пользователь'}</strong></div>
        </div>
        <div className="settings-activation"><Badge tone={view.activation?.genuine ? 'green' : 'amber'}>{view.activation?.genuine ? 'Лицензия подтверждена' : view.activation?.status === 'unknown' ? 'Статус неизвестен' : 'Требует внимания'}</Badge><p>{view.activation?.detail || 'Лицензирование не изменяется WinDeploy. Используйте штатные средства Windows: slmgr.vbs /dlv.'}</p></div>
      </Panel>
      <Panel title={<><MonitorCog size={16} /> Имя компьютера</>}>
        <div className="settings-form"><label><span>Новое имя</span><input value={name} onChange={event => setName(event.target.value)} aria-label="Имя компьютера" /></label></div>
        <div className="settings-actions"><Button disabled={busy || !name.trim()} onClick={() => void namePreview()}>Проверить</Button>{pending?.label === 'name' ? <><Button variant="danger" disabled={busy} onClick={() => void apply(pending.run)}>Подтвердить переименование</Button><Button variant="ghost" onClick={() => setPending(null)}>Отмена</Button></> : <Button variant="primary" disabled={busy || !name.trim()} onClick={() => setPending({ label: 'name', run: () => api.applyComputerName(name, false) })}>Переименовать</Button>}</div>
        <p className="settings-note">Переименование требует прав администратора и перезагрузки.</p>
      </Panel>
      <Panel title={<><SlidersHorizontal size={16} /> Часовой пояс</>}>
        <div className="settings-form"><label><span>Идентификатор зоны Windows</span><input value={timezone} onChange={event => setTimezone(event.target.value)} placeholder="Russian Standard Time" aria-label="Часовой пояс" list="timezone-ids" /><datalist id="timezone-ids"><option value="UTC" /><option value="Russian Standard Time" /><option value="Central Standard Time" /><option value="W. Europe Standard Time" /></datalist></label></div>
        <div className="settings-actions"><Button disabled={busy || !timezone.trim()} onClick={() => void timezonePreview()}>Проверить</Button>{pending?.label === 'timezone' ? <><Button variant="danger" disabled={busy} onClick={() => void apply(pending.run)}>Подтвердить смену зоны</Button><Button variant="ghost" onClick={() => setPending(null)}>Отмена</Button></> : <Button variant="primary" disabled={busy || !timezone.trim()} onClick={() => setPending({ label: 'timezone', run: () => api.applyTimezone(timezone, false) })}>Применить</Button>}</div>
      </Panel>
      <Panel title={<><SlidersHorizontal size={16} /> Проводник</>}>
        <div className="settings-checks">
          <label><input type="checkbox" checked={showHidden} onChange={event => setShowHidden(event.target.checked)} />Показывать скрытые файлы</label>
          <label><input type="checkbox" checked={showExtensions} onChange={event => setShowExtensions(event.target.checked)} />Показывать расширения файлов</label>
          <label><input type="checkbox" checked={showSuperHidden} onChange={event => setShowSuperHidden(event.target.checked)} />Показывать системные файлы</label>
          <label><input type="checkbox" checked={openThisPC} onChange={event => setOpenThisPC(event.target.checked)} />Открывать «Этот компьютер»</label>
        </div>
        <div className="settings-actions"><Button disabled={busy} onClick={() => void explorerPreview()}>Проверить</Button>{pending?.label === 'explorer' ? <><Button variant="danger" disabled={busy} onClick={() => void apply(pending.run)}>Подтвердить</Button><Button variant="ghost" onClick={() => setPending(null)}>Отмена</Button></> : <Button variant="primary" disabled={busy} onClick={() => setPending({ label: 'explorer', run: () => api.applyExplorer({ show_hidden: showHidden, show_extensions: showExtensions, show_super_hidden: showSuperHidden, open_this_pc: openThisPC }, false) })}>Применить</Button>}</div>
      </Panel>
      <Panel title={<><PlugZap size={16} /> Электропитание</>}>
        <div className="settings-form"><label><span>Схема</span><select value={power} onChange={event => setPower(event.target.value)} aria-label="Схема электропитания"><option value="">Выберите схему</option>{plans.map(plan => <option key={plan.guid} value={plan.guid}>{plan.name}</option>)}</select></label></div>
        <div className="settings-actions"><Button disabled={busy || !power} onClick={() => void powerPreview()}>Проверить</Button>{pending?.label === 'power' ? <><Button variant="danger" disabled={busy} onClick={() => void apply(pending.run)}>Подтвердить</Button><Button variant="ghost" onClick={() => setPending(null)}>Отмена</Button></> : <Button variant="primary" disabled={busy || !power} onClick={() => setPending({ label: 'power', run: () => api.applyPowerPlan(power, false) })}>Применить</Button>}</div>
      </Panel>
      <Panel title="Windows Update" action={<Button onClick={() => void runDiagnostics()} disabled={busy}>Диагностика</Button>}>
        {diagnostics ? <div className="settings-checks">{diagnostics.checks?.map((check, index) => <div key={index} className="settings-check"><Badge tone={check.status === 'ok' ? 'green' : 'amber'}>{check.status === 'ok' ? 'ОК' : 'Внимание'}</Badge><span>{check.name}</span><small>{check.detail}</small></div>)}</div> : <p className="settings-note">Диагностика только читает состояние службы, маркеры перезагрузки и политики обновлений.</p>}
      </Panel>
    </div>}

  </>;
}
