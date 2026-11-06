import { fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import DataTable from './DataTable';

function Fixture() {
  const [query, setQuery] = useState('');
  return <DataTable rows={[{ name: 'Beta', publisher: 'Vendor Two' }, { name: 'Alpha', publisher: 'Vendor One' }]} columns={[{ key: 'name', label: 'Имя', value: row => row.name }, { key: 'publisher', label: 'Издатель', value: row => row.publisher }]} rowKey={row => row.name} query={query} onQuery={setQuery} />;
}
describe('inventory table interactions', () => {
  it('sorts, searches every column and reports empty results honestly', () => {
    render(<Fixture />);
    expect(screen.getAllByRole('row')[1]).toHaveTextContent('Alpha');
    fireEvent.click(screen.getByRole('button', { name: 'Сортировать: Имя' }));
    expect(screen.getAllByRole('row')[1]).toHaveTextContent('Beta');
    fireEvent.change(screen.getByRole('textbox', { name: 'Поиск в таблице' }), { target: { value: 'Vendor One' } });
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', { name: 'Поиск в таблице' }), { target: { value: 'not-present' } });
    expect(screen.getByRole('heading', { name: 'Ничего не найдено' })).toBeInTheDocument();
  });
});
