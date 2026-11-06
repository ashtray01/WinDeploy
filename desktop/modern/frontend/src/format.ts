import type { Report } from './bridge';
export function bytes(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value < 0) return '—';
  if (value === 0) return '0 Б';
  const index = Math.min(4, Math.floor(Math.log(value) / Math.log(1024)));
  return `${(value / 1024 ** index).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} ${['Б', 'КБ', 'МБ', 'ГБ', 'ТБ'][index]}`;
}
export function time(value?: string): string { return value ? new Date(value).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '—'; }
export function sourceKnown(report: Report | null | undefined, name: string): boolean {
  return report?.sections?.some(section => section.name === name && (section.status === 'ok' || section.status === 'partial')) ?? false;
}
export const sourceNames: Record<string, string> = { system: 'Система', cpu: 'Процессор', memory: 'Оперативная память', volumes: 'Накопители', interfaces: 'Интерфейсы', network: 'Конфигурация сети', apps: 'Программы' };
export const sectionStatuses: Record<string, string> = { ok: 'Прочитано', partial: 'Частично', failed: 'Ошибка', cancelled: 'Отменено', 'not-collected': 'Не прочитано' };
