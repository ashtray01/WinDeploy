import { Activity, AppWindow, Boxes, Cable, CircleHelp, FolderOpen, Globe, HardDrive, LayoutDashboard, Layers3, Monitor, PackagePlus, Printer, Search, Settings2, ShieldCheck, ScrollText, Wrench, type LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { Badge, Progress } from './ui';

export type Page = 'overview' | 'profiles' | 'packages' | 'apps' | 'network' | 'networkdiag' | 'shares' | 'drivers' | 'printers' | 'browsers' | 'winsettings' | 'volumes' | 'system' | 'journal';
export type DataStatus = 'idle' | 'scanning' | 'complete' | 'partial' | 'cancelled' | 'error';
const statusText: Record<DataStatus, string> = { idle: 'Нет данных', scanning: 'Сканирование', complete: 'Данные получены', partial: 'Частичные данные', cancelled: 'Отменено', error: 'Ошибка чтения' };
const items: { id: Page; label: string; icon: LucideIcon; group: string }[] = [
  { id: 'overview', label: 'Обзор', icon: LayoutDashboard, group: 'РАБОЧЕЕ ПРОСТРАНСТВО' },
  { id: 'profiles', label: 'Профили', icon: Layers3, group: '' },
  { id: 'packages', label: 'Установка ПО', icon: PackagePlus, group: '' },
  { id: 'apps', label: 'Программы', icon: AppWindow, group: 'ИНВЕНТАРИЗАЦИЯ' },
  { id: 'network', label: 'Сеть', icon: Cable, group: '' },
  { id: 'volumes', label: 'Накопители', icon: HardDrive, group: '' },
  { id: 'system', label: 'Система', icon: Monitor, group: '' },
  { id: 'shares', label: 'Сетевые папки', icon: FolderOpen, group: 'СЕТЬ И ДОСТУП' },
  { id: 'networkdiag', label: 'Диагностика', icon: Activity, group: '' },
  { id: 'drivers', label: 'Драйверы', icon: Boxes, group: 'ОБОРУДОВАНИЕ' },
  { id: 'printers', label: 'Принтеры', icon: Printer, group: '' },
  { id: 'browsers', label: 'Браузеры', icon: Globe, group: 'НАСТРОЙКА' },
  { id: 'winsettings', label: 'Windows', icon: Wrench, group: '' },
  { id: 'journal', label: 'Журнал', icon: ScrollText, group: 'ИНСТРУМЕНТЫ' },
];

export default function Shell({ page, navigate, children, status = 'idle', hostname, query = '', onQuery, updated }: { page: Page; navigate: (page: Page) => void; children: ReactNode; status?: DataStatus; hostname?: string; query?: string; onQuery?: (query: string) => void; updated?: string }) {
  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark" aria-hidden="true"><i /><i /><i /><i /></div><div><strong>WinDeploy<span>®</span></strong><small>WORKSTATION MANAGER</small></div></div>
      <nav aria-label="Основная навигация">{items.map(({ id, label, icon: Icon, group }) => <div key={id}>{group && <p className="nav-group">{group}</p>}<button className={`nav-item ${page === id ? 'active' : ''}`} aria-current={page === id ? 'page' : undefined} onClick={() => navigate(id)}><Icon size={17} /><span>{label}</span>{page === id && <i className="nav-dot" />}</button></div>)}

      </nav>
      <div className="sidebar-bottom"><div className="local-device"><span className="device-icon"><Monitor size={17} /></span><div><small>ЛОКАЛЬНЫЙ КОМПЬЮТЕР</small><strong title={hostname}>{hostname || 'Ожидание данных'}</strong></div></div><div className="sidebar-help"><span><ShieldCheck size={13} /> Установка ПО</span><CircleHelp size={15} /></div></div>
    </aside>
    <header className="topbar"><div className="breadcrumb">Рабочая станция<span>/</span><strong>{items.find(item => item.id === page)?.label}</strong></div><div className="topbar-right"><label className="global-search"><Search size={15} /><input aria-label="Поиск в разделе" placeholder="Поиск в разделе…" value={query} onChange={event => onQuery?.(event.target.value)} disabled={!onQuery} /><kbd>⌕</kbd></label><Badge tone={status === 'complete' ? 'green' : status === 'scanning' ? 'blue' : status === 'idle' ? 'muted' : 'amber'}>{statusText[status]}</Badge><button className="topbar-settings" disabled aria-label="Настройки пока недоступны" title="Настройки будут доступны позже"><Settings2 size={18} /></button></div></header>
    <main className="app-main">{children}</main>
    <footer className="statusbar"><div><span className={`connection-dot ${status === 'scanning' ? 'working' : ''}`} /><span>Локальное приложение</span><i /><span>Установка программ · проверка SHA-256 и подписи</span></div><div>{status === 'scanning' ? <><span>Чтение Inventory</span><div className="footer-progress"><Progress value={null} label="Сканирование компьютера" /></div></> : <span>{updated ? `Обновлено ${new Date(updated).toLocaleTimeString('ru-RU')}` : 'Сканирование не выполнено'}</span>}<i /><span>MODERN 0.2.0</span></div></footer>
  </div>;
}
