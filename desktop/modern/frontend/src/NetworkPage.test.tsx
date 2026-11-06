import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import NetworkPage from './NetworkPage';
import type { NetworkReportView } from './bridge';

const mocks = vi.hoisted(() => ({ runNetworkDiagnostics: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocks }));

const report: NetworkReportView = {
  checks: [
    { id: 'dns-localhost', name: 'DNS · localhost', kind: 'dns', status: 'ok', detail: 'Адресов: 2', latency_ms: 4, required: true },
    { id: 'icmp-1', name: 'ICMP · 8.8.8.8', kind: 'icmp', status: 'failed', detail: 'Узел не ответил на ICMP', error_code: 'icmp-failed', latency_ms: 12, required: false },
  ],
  failed: 1,
  complete: true,
  message: 'Проверки завершены. Ошибок: 1 из 2.',
};

describe('network diagnostics page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.runNetworkDiagnostics.mockResolvedValue(report);
  });

  it('runs bounded checks from the form and renders real statuses', async () => {
    render(<NetworkPage />);
    expect(screen.getByLabelText('DNS-имя')).toHaveValue('localhost');
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.runNetworkDiagnostics).toHaveBeenCalledWith(['dns:localhost']));
    expect(await screen.findByText('DNS · localhost')).toBeInTheDocument();
    expect(screen.getByText('Узел не ответил на ICMP')).toBeInTheDocument();
    expect(screen.getByText('Ошибок: 1')).toBeInTheDocument();
  });

  it('adds explicit TCP, HTTP and SMB targets', async () => {
    render(<NetworkPage />);
    fireEvent.change(screen.getByLabelText('TCP-порт'), { target: { value: 'fileserver:445' } });
    fireEvent.change(screen.getByLabelText('HTTP-адрес'), { target: { value: 'https://example.test/' } });
    fireEvent.change(screen.getByLabelText('SMB-сервер'), { target: { value: 'fileserver' } });
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.runNetworkDiagnostics).toHaveBeenCalledWith(['dns:localhost', 'tcp:fileserver:445', 'http:https://example.test/', 'smb:fileserver']));
  });
});
