import { Info, TriangleAlert } from 'lucide-react';
import type { AdapterConfig, InstalledApp, InterfaceInfo, Report, State, Volume } from './bridge';
import DataTable, { type Column } from './DataTable';
import { Badge, EmptyState, PageHeading, Panel, Progress } from './ui';
import ScanActions from './ScanActions';
import { bytes, sectionStatuses, sourceKnown, sourceNames } from './format';
import type { Page } from './Shell';

const appsColumns: Column<InstalledApp>[] = [
  { key: 'name', label: 'Программа', value: row => row.name, render: row => <strong className="cell-title">{row.name}</strong> },
  { key: 'version', label: 'Версия', value: row => row.version || '—', render: row => <span className="mono">{row.version || '—'}</span> },
  { key: 'publisher', label: 'Издатель', value: row => row.publisher || '—' },
  { key: 'scope', label: 'Область', value: row => row.scope === 'machine' ? 'Компьютер' : 'Текущий пользователь' },
  { key: 'views', label: 'Представление реестра', value: row => row.views?.join(' / ') || '—' },
];
type NetworkRow = InterfaceInfo & { config?: AdapterConfig };
function speed(value?: number) { return value ? `${(value / 1e6).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} Мбит/с` : 'Неизвестно'; }
const stack = (values?: string[] | null) => values?.length ? <div className="cell-stack mono">{values.map(value => <span key={value}>{value}</span>)}</div> : <span className="muted">—</span>;
const networkColumns: Column<NetworkRow>[] = [
  { key: 'name', label: 'Интерфейс', value: row => row.name, render: row => <div><strong className="cell-title">{row.name}</strong><small className="cell-sub">{row.hardware_address || 'MAC недоступен'} · MTU {row.mtu}</small>{row.issue && <Badge tone="amber">Чтение частично</Badge>}</div> },
  { key: 'state', label: 'Состояние', value: row => row.up ? 'Up' : 'Down', render: row => <Badge tone={row.up ? 'green' : 'muted'}>{row.up ? 'Up' : 'Down'}</Badge> },
  { key: 'addresses', label: 'IP / префикс', value: row => row.addresses?.join(' ') || '', render: row => stack(row.addresses) },
  { key: 'dns', label: 'DNS', value: row => row.config?.dns_servers?.join(' ') || '', render: row => stack(row.config?.dns_servers) },
  { key: 'gateway', label: 'Шлюзы', value: row => row.config?.gateways?.join(' ') || '', render: row => stack(row.config?.gateways) },
  { key: 'mode', label: 'IPv4', value: row => !row.config ? 'Неизвестно' : !row.config.ipv4_enabled ? 'Не включён' : row.config.dhcp_v4_enabled ? 'DHCP' : 'Статический' },
  { key: 'speed', label: 'Передача / приём', value: row => row.config?.transmit_bits_per_second ?? -1, render: row => <div className="cell-stack"><span>{speed(row.config?.transmit_bits_per_second)}</span><small className="muted">{speed(row.config?.receive_bits_per_second)}</small></div> },
];
const kind: Record<string, string> = { fixed: 'Локальный', ramdisk: 'RAM-диск', network: 'Сетевой', removable: 'Съёмный', optical: 'Оптический', unknown: 'Неизвестен' };
const volumeColumns: Column<Volume>[] = [
  { key: 'root', label: 'Том', value: row => row.root, render: row => <strong className="mono">{row.root}</strong> },
  { key: 'kind', label: 'Тип', value: row => kind[row.kind] || row.kind },
  { key: 'total', label: 'Всего', value: row => row.capacity_known ? row.total_bytes : -1, render: row => row.capacity_known ? bytes(row.total_bytes) : '—' },
  { key: 'available', label: 'Доступно пользователю', value: row => row.capacity_known ? row.available_bytes : -1, render: row => row.capacity_known ? bytes(row.available_bytes) : '—' },
  { key: 'usage', label: 'Использование', value: row => row.capacity_known && row.total_bytes ? 1 - row.available_bytes / row.total_bytes : -1, render: row => row.capacity_known && row.total_bytes ? <div className="table-usage"><Progress value={(1 - row.available_bytes / row.total_bytes) * 100} label={`Занято на ${row.root}`} /></div> : '—' },
  { key: 'status', label: 'Чтение', value: row => row.capacity_known ? 'Прочитано' : row.issue?.includes('failed') ? 'Ошибка чтения' : 'Не запрашивалось', render: row => <Badge tone={row.capacity_known ? 'green' : 'amber'}>{row.capacity_known ? 'Прочитано' : row.issue?.includes('failed') ? 'Ошибка чтения' : 'Не запрашивалось'}</Badge> },
];
const titles = { apps: ['Программы', 'Видимые записи установленного ПО из реестра Windows.'], network: ['Сетевые интерфейсы', 'Адреса, DNS и шлюзы. Только чтение конфигурации.'], volumes: ['Накопители', 'Логические тома и доступная пользователю ёмкость.'], system: ['Системные характеристики', 'Реальные сведения об операционной системе и оборудовании.'] };

function SourceNotice({ report, names }: { report: Report; names: string[] }) {
  const issues = report.sections?.filter(section => names.includes(section.name) && section.status !== 'ok') ?? [];
  return issues.length ? <div className="callout" role="alert"><TriangleAlert size={15} /><span>Сведения доступны не полностью: {issues.map(section => `${sourceNames[section.name]} — ${sectionStatuses[section.status] || section.status}`).join('; ')}. Повторите сканирование при необходимости.</span></div> : null;
}

export default function InventoryPages({ page, state, query, onQuery, scan, cancel }: { page: Extract<Page, 'apps' | 'network' | 'volumes' | 'system'>; state: State | null; query: string; onQuery: (query: string) => void; scan: () => unknown; cancel: () => unknown }) {
  const report = state?.report;
  const [title, detail] = titles[page];
  const names = page === 'system' ? ['system', 'cpu', 'memory'] : page === 'network' ? ['interfaces', 'network'] : [page];
  const network: NetworkRow[] = (report?.interfaces ?? []).map(iface => ({ ...iface, config: report?.network?.find(adapter => adapter.index === iface.index || adapter.ipv6_index === iface.index) }));
  const system = report?.system;
  const properties = [
    ['Имя компьютера', system?.hostname], ['Windows', state?.system_name], ['Сборка Windows', system?.windows.build], ['Версия NT', system ? `${system.windows.major}.${system.windows.minor}` : undefined],
    ['Архитектура ОС', system?.architecture], ['Архитектура процесса', system?.process_architecture], ['Процессор', report?.cpu?.model], ['Производитель CPU', report?.cpu?.vendor], ['Логических процессоров', report?.cpu?.logical_processors],
    ['Оперативная память', bytes(report?.memory?.total_physical_bytes)], ['Доступная память', bytes(report?.memory?.available_physical_bytes)], ['Последнее чтение', report ? new Date(report.finished_at).toLocaleString('ru-RU') : undefined],
  ].map(([name, value]) => ({ name: String(name), value: value == null ? '—' : String(value) }));
  const columns: Column<{ name: string; value: string }>[] = [{ key: 'name', label: 'Характеристика', value: row => row.name }, { key: 'value', label: 'Значение', value: row => row.value }];
  const unavailable = report && page !== 'system' && !sourceKnown(report, page === 'network' ? 'interfaces' : page);
  return <>
    <PageHeading eyebrow="Инвентаризация" title={title} detail={detail} action={<ScanActions state={state} scan={scan} cancel={cancel} />} />
    {report && <SourceNotice report={report} names={names} />}
    <div className="callout blue"><Info size={14} /><span>{page === 'apps' ? 'Это записи реестра, а не проверка файлов. Portable и AppX/MSIX пока не учитываются. HKCU относится к пользователю процесса.' : page === 'network' ? 'Up не подтверждает доступность интернета. Скорость — параметр линка, сетевые настройки не изменяются.' : page === 'volumes' ? 'Ёмкость сетевых, съёмных и оптических дисков не запрашивается. Значения могут учитывать квоты пользователя.' : 'Неизвестные значения остаются неизвестными. Совместимость Windows 7 требует отдельной VM-проверки.'}</span></div>
    <Panel>{!report ? <EmptyState title="Сведения ещё не получены" detail="Запустите сканирование компьютера." /> : unavailable ? <EmptyState title="Источник недоступен" detail="Нулевые счётчики не подставляются вместо ошибки чтения. Повторите сканирование." /> : page === 'apps' ? <DataTable rows={report.apps ?? []} columns={appsColumns} rowKey={row => `${row.scope}:${row.key}:${row.version}:${row.views}`} query={query} onQuery={onQuery} /> : page === 'network' ? <DataTable rows={network} columns={networkColumns} rowKey={row => `${row.index}:${row.name}`} query={query} onQuery={onQuery} /> : page === 'volumes' ? <DataTable rows={report.volumes ?? []} columns={volumeColumns} rowKey={row => row.root} query={query} onQuery={onQuery} /> : <DataTable rows={properties} columns={columns} rowKey={row => row.name} query={query} onQuery={onQuery} />}</Panel>
  </>;
}
