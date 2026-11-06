import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { Database, LoaderCircle, type LucideIcon } from 'lucide-react';

export type Tone = 'blue' | 'green' | 'amber' | 'red' | 'muted';

export function Badge({ children, tone = 'muted', dot = true }: { children: ReactNode; tone?: Tone; dot?: boolean }) {
  return <span className={`badge badge-${tone}`}>{dot && <i aria-hidden="true" />}{children}</span>;
}

export function Button({ children, variant = 'secondary', className = '', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost' | 'danger' }) {
  return <button className={`button button-${variant} ${className}`} {...props}>{children}</button>;
}

export function Panel({ children, title, action, className = '' }: { children: ReactNode; title?: ReactNode; action?: ReactNode; className?: string }) {
  return <section className={`panel ${className}`}>{(title || action) && <div className="panel-heading"><h2>{title}</h2>{action}</div>}{children}</section>;
}

export function EmptyState({ title, detail, icon: Icon = Database }: { title: string; detail: string; icon?: LucideIcon }) {
  return <div className="empty-state"><div className="empty-icon"><Icon size={24} /></div><h3>{title}</h3><p>{detail}</p></div>;
}

export function Progress({ value, label }: { value: number | null; label: string }) {
  const known = value !== null && Number.isFinite(value);
  const normalized = known ? Math.min(100, Math.max(0, value)) : undefined;
  return <div className={`progress ${known ? '' : 'indeterminate'}`} role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={normalized}><span style={known ? { width: `${normalized}%` } : undefined} /></div>;
}

export function Spinner({ label = 'Загрузка' }: { label?: string }) {
  return <span className="spinner-label" role="status"><LoaderCircle className="spin" size={16} />{label}</span>;
}

export function PageHeading({ eyebrow, title, detail, action }: { eyebrow: string; title: string; detail: string; action?: ReactNode }) {
  return <div className="page-heading"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p className="page-description">{detail}</p></div>{action && <div className="heading-actions">{action}</div>}</div>;
}

export function Metric({ icon: Icon, label, value, detail, children }: { icon: LucideIcon; label: string; value: ReactNode; detail: ReactNode; children?: ReactNode }) {
  return <div className="metric panel"><div className="metric-top"><span>{label}</span><Icon size={18} /></div><div className="metric-value">{value}</div><div className="metric-detail">{detail}</div>{children}</div>;
}
