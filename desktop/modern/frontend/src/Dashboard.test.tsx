import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import Dashboard from './Dashboard';
import type { State } from './bridge';

describe('inventory dashboard', () => {
  it('does not invent counts or system identity before a report', () => {
    render(<Dashboard state={null} navigate={vi.fn()} scan={vi.fn()} cancel={vi.fn()} />);
    expect(screen.getByTestId('machine-name')).toHaveTextContent('Ожидание данных');
    expect(screen.getByTestId('app-count')).toHaveTextContent('—');
  });
  it('renders supplied collector data and offers rescan', () => {
    const state: State = { scanning: false, cancel_requested: false, generation: 4, status: 'complete', system_name: 'Windows 11', error: '', journal: [], report: { version: 1, started_at: '2026-11-06T10:00:00Z', finished_at: '2026-11-06T10:00:01Z', complete: true, sections: [{ name: 'apps', status: 'ok' }], system: { hostname: 'UNIT-WORKSTATION', architecture: 'amd64', process_architecture: 'amd64', windows: { major: 10, minor: 0, build: 26100, service_pack_major: 0, product_type: 1 } }, apps: [], volumes: [], interfaces: [], network: [] } };
    render(<Dashboard state={state} navigate={vi.fn()} scan={vi.fn()} cancel={vi.fn()} />);
    expect(screen.getByTestId('machine-name')).toHaveTextContent('UNIT-WORKSTATION');
    expect(screen.getByTestId('app-count')).toHaveTextContent('0');
    expect(screen.getByRole('button', { name: 'Обновить данные' })).toBeInTheDocument();
    expect(screen.getByTestId('scan-generation')).toHaveTextContent('#4');
  });
});
