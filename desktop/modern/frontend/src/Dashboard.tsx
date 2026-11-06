import { AppWindow, ArrowRight, Cable, Check, Cpu, HardDrive, Info, Layers3, MemoryStick, Monitor, RefreshCw, ShieldCheck } from 'lucide-react';
import type { State } from './bridge';
import type { Page } from './Shell';
import { Badge, EmptyState, Metric, PageHeading, Panel, Progress } from './ui';
import { bytes, sourceKnown, time } from './format';
import ScanActions from './ScanActions';
import './dashboard.css';

export default function Dashboard({ state, navigate, scan, cancel }: { state: State | null; navigate: (page: Page) => void; scan: () => unknown; cancel: () => unknown }) {
  const report = state?.report, system = report?.system, cpu = report?.cpu, memory = report?.memory;
  const volumes = (report?.volumes ?? []).filter(volume => volume.capacity_known);
  const active = (report?.interfaces ?? []).filter(iface => iface.up && !iface.loopback);
  const ok = report?.sections?.filter(section => section.status === 'ok').length ?? 0;
  const total = report?.sections?.length ?? 0;
  return <>
    <PageHeading eyebrow="Подготовка рабочих станций" title="Обзор компьютера" detail="Сведения о рабочей станции перед настройкой." action={<ScanActions state={state} scan={scan} cancel={cancel} />} />
    <div className="overview-hero">
      <Panel className="machine-panel"><div className="device-visual"><Monitor size={51} strokeWidth={1.1} /><span><ShieldCheck size={15} /></span></div><div className="machine-copy"><div className="machine-eyebrow">ЛОКАЛЬНАЯ РАБОЧАЯ СТАНЦИЯ <Badge tone="blue" dot={false}>{system?.architecture === 'amd64' ? 'x64' : system?.architecture === '386' ? 'x86' : 'Нет сведений'}</Badge></div><h2 data-testid="machine-name">{system?.hostname || 'Ожидание данных'}</h2><p>{state?.system_name || 'Версия Windows ещё не прочитана'}{system && <> <span>·</span> сборка {system.windows.build}</>}</p><div className="machine-meta"><span><Cpu size={12} />{cpu?.vendor || 'CPU: нет сведений'}</span><span><RefreshCw size={11} />{report ? `Проверено в ${time(report.finished_at)}` : 'Сканирование не завершено'}</span></div></div></Panel>
      <Panel className="scan-panel"><div className="scan-panel-title"><span className="scan-symbol">{report?.complete && !state?.scanning ? <Check size={17} /> : <Info size={17} />}</span><div><h2>{state?.scanning ? 'Чтение сведений' : report?.complete ? 'Инвентаризация завершена' : 'Статус инвентаризации'}</h2><p>{state?.scanning ? 'Ожидание ответов источников Windows' : report ? `${ok} из ${total} источников прочитаны полностью` : 'Данные появятся после сканирования'}</p></div></div><Progress value={state?.scanning || !report ? null : total ? ok / total * 100 : null} label="Доступность источников Inventory" /><div className="scan-panel-foot"><span>Системные изменения не выполняются</span><span data-testid="scan-generation">#{state?.generation ?? 0}</span></div></Panel>
    </div>
    <div className="metrics-grid">
      <Metric icon={Cpu} label="ПРОЦЕССОР" value={cpu ? `${cpu.logical_processors} потоков` : '—'} detail={cpu?.model || 'Источник CPU недоступен'} />
      <Metric icon={MemoryStick} label="ОПЕРАТИВНАЯ ПАМЯТЬ" value={bytes(memory?.total_physical_bytes)} detail={memory ? `${bytes(memory.available_physical_bytes)} доступно · ${memory.load_percent}% занято` : 'Источник RAM недоступен'}><Progress value={memory?.load_percent ?? null} label="Использование памяти" /></Metric>
      <Metric icon={HardDrive} label="ЛОКАЛЬНЫЕ ТОМА" value={sourceKnown(report, 'volumes') ? volumes.length : '—'} detail={sourceKnown(report, 'volumes') ? `${bytes(volumes.reduce((sum, volume) => sum + volume.available_bytes, 0))} доступно на прочитанных томах` : 'Сведения о ёмкости недоступны'} />
      <Metric icon={AppWindow} label="ЗАПИСИ ПРОГРАММ" value={<span data-testid="app-count">{sourceKnown(report, 'apps') ? report?.apps?.length ?? 0 : '—'}</span>} detail="Реестр Windows · текущий пользователь и компьютер" />
    </div>
    <div className="overview-bottom">
      <Panel title="Локальные накопители" action={<button className="link-button" onClick={() => navigate('volumes')}>Все тома <ArrowRight size={12} /></button>}>
        {volumes.length ? <div className="volume-overview">{volumes.slice(0, 3).map(volume => <div key={volume.root} className="volume-row"><div className="volume-icon"><HardDrive size={19} /></div><div className="volume-row-content"><div><strong>{volume.root}</strong><span>{bytes(volume.available_bytes)} доступно из {bytes(volume.total_bytes)}</span></div><Progress value={volume.total_bytes ? (1 - volume.available_bytes / volume.total_bytes) * 100 : null} label={`Занято на ${volume.root}`} /></div></div>)}</div> : <EmptyState title="Нет доступной ёмкости" detail="Показаны только действительно прочитанные локальные тома." icon={HardDrive} />}
      </Panel>
      <Panel title="Сетевые интерфейсы" action={<button className="link-button" onClick={() => navigate('network')}>Подробнее <ArrowRight size={12} /></button>}>
        {active.length ? <div className="network-overview">{active.slice(0, 3).map(iface => <div className="network-row" key={iface.index}><span className="network-icon"><Cable size={16} /></span><div><strong>{iface.name}</strong><p className="mono">{iface.addresses?.join(' · ') || 'IP-адреса не назначены'}</p></div><Badge tone="green">Up</Badge></div>)}</div> : <EmptyState title="Нет активных интерфейсов" detail="Состояние интерфейса не является проверкой доступности интернета." icon={Cable} />}
      </Panel>
    </div>
    <div className="profile-prompt"><div className="profile-prompt-icon"><Layers3 size={22} /></div><div><h2>План настройки начинается с профиля</h2><p>Просмотрите компоненты и зависимости рабочего места до любых изменений.</p></div><button className="link-button" onClick={() => navigate('profiles')}>Перейти к профилям <ArrowRight size={14} /></button></div>
  </>;
}
