import { useMemo, useState, type ReactNode } from 'react';
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight, Search } from 'lucide-react';
import { Button, EmptyState } from './ui';
import './tables.css';

export interface Column<T> { key: string; label: string; value: (row: T) => string | number; render?: (row: T) => ReactNode }
export default function DataTable<T>({ rows, columns, rowKey, query, onQuery, emptyTitle = 'Нет записей' }: { rows: T[]; columns: Column<T>[]; rowKey: (row: T) => string; query: string; onQuery: (query: string) => void; emptyTitle?: string }) {
  const [sort, setSort] = useState({ key: columns[0]?.key ?? '', descending: false });
  const [page, setPage] = useState(0);
  const filtered = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase('ru-RU');
    const items = rows.filter(row => !needle || columns.some(column => String(column.value(row)).toLocaleLowerCase('ru-RU').includes(needle)));
    const column = columns.find(column => column.key === sort.key);
    return column ? items.sort((a, b) => {
      const x = column.value(a), y = column.value(b);
      const result = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), 'ru', { numeric: true, sensitivity: 'base' });
      return sort.descending ? -result : result;
    }) : items;
  }, [rows, columns, query, sort]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / 25));
  const currentPage = Math.min(page, totalPages - 1);
  return <>
    <div className="table-toolbar"><label className="table-search"><Search size={15} /><input aria-label="Поиск в таблице" placeholder="Поиск по всем столбцам…" value={query} onChange={event => { setPage(0); onQuery(event.target.value); }} /></label><span>Найдено <strong>{filtered.length}</strong> из {rows.length}</span></div>
    {filtered.length ? <div className="table-scroll"><table className="data-table"><thead><tr>{columns.map(column => <th key={column.key} scope="col" aria-sort={sort.key === column.key ? sort.descending ? 'descending' : 'ascending' : 'none'}><button aria-label={`Сортировать: ${column.label}`} onClick={() => { setPage(0); setSort({ key: column.key, descending: sort.key === column.key && !sort.descending }); }}>{column.label}{sort.key === column.key ? sort.descending ? <ArrowDown size={11} /> : <ArrowUp size={11} /> : <ArrowUpDown size={11} />}</button></th>)}</tr></thead><tbody>{filtered.slice(currentPage * 25, (currentPage + 1) * 25).map(row => <tr key={rowKey(row)}>{columns.map(column => <td key={column.key}>{column.render?.(row) ?? String(column.value(row))}</td>)}</tr>)}</tbody></table></div> : <EmptyState title={query ? 'Ничего не найдено' : emptyTitle} detail={query ? 'Измените запрос или очистите поиск.' : 'Источник не вернул записей для этого раздела.'} icon={Search} />}
    <div className="table-footer"><span>Данные только для просмотра</span><div><span>Страница {currentPage + 1} из {totalPages}</span><Button className="small" aria-label="Предыдущая страница" disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}><ChevronLeft size={13} /></Button><Button className="small" aria-label="Следующая страница" disabled={currentPage >= totalPages - 1} onClick={() => setPage(currentPage + 1)}><ChevronRight size={13} /></Button></div></div>
  </>;
}
