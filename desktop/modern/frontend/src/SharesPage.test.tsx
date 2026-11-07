import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SharesPage from './SharesPage';
import type { ShareResultView } from './bridge';

const mocks = vi.hoisted(() => ({ addShare: vi.fn(), listShareConnections: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocks }));

const connected: ShareResultView = { share: 'finance', unc: '\\\\fileserver\\finance', connected: true, credential_saved: true, shortcut_path: 'C:\\Desktop\\Finance.lnk', conflict: false, message: 'Сетевой ресурс подключён, ярлык создан на рабочем столе.', warnings: [] };
const conflict: ShareResultView = { share: 'finance', unc: '\\\\fileserver\\finance', connected: false, credential_saved: false, conflict: true, message: 'Конфликт учётных данных.', warnings: [] };

describe('network shares page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listShareConnections.mockResolvedValue([{ remote_name: '\\\\other\\share', local_name: 'Z:' }]);
    mocks.addShare.mockResolvedValue(connected);
  });

  it('requires explicit confirmation before connecting', async () => {
    render(<SharesPage />);
    expect(await screen.findByText('\\\\other\\share')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('UNC-путь'), { target: { value: '\\\\fileserver\\finance' } });
    fireEvent.change(screen.getByLabelText('Название подключения'), { target: { value: 'Finance' } });
    fireEvent.change(screen.getByLabelText('Пользователь'), { target: { value: 'OFFICE\\user' } });
    fireEvent.change(screen.getByLabelText('Пароль'), { target: { value: 'secret' } });
    expect(mocks.addShare).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Подключить' }));
    expect(await screen.findByText(/Пароль будет сохранён/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    await waitFor(() => expect(mocks.addShare).toHaveBeenCalledTimes(1));
    const request = mocks.addShare.mock.calls[0][0];
    expect(request.unc).toBe('\\\\fileserver\\finance');
    expect(request.password).toBe('secret');
    expect(request.dry_run).toBe(false);
    expect(request.resolve_conflict).toBe(false);
    expect(await screen.findByText(/Сетевой ресурс подключён/)).toBeInTheDocument();
  });

  it('offers explicit conflict resolution after Error 1219', async () => {
    mocks.addShare.mockResolvedValueOnce(conflict).mockResolvedValueOnce(connected);
    render(<SharesPage />);
    fireEvent.change(screen.getByLabelText('UNC-путь'), { target: { value: '\\\\fileserver\\finance' } });
    fireEvent.click(screen.getByRole('button', { name: 'Подключить' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText(/Windows Error 1219/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Закрыть конфликтующие сеансы и подключить' }));
    await waitFor(() => expect(mocks.addShare).toHaveBeenCalledTimes(2));
    expect(mocks.addShare.mock.calls[1][0].resolve_conflict).toBe(true);
  });

  it('supports a dry-run preview without system changes', async () => {
    mocks.addShare.mockResolvedValueOnce({ ...connected, connected: false, shortcut_path: 'C:\\Desktop\\Finance.lnk', message: 'Пробный режим: система не изменялась.' });
    render(<SharesPage />);
    fireEvent.change(screen.getByLabelText('UNC-путь'), { target: { value: '\\\\fileserver\\finance' } });
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.addShare).toHaveBeenCalled());
    expect(mocks.addShare.mock.calls[0][0].dry_run).toBe(true);
    expect(await screen.findByText('Пробный режим: система не изменялась.')).toBeInTheDocument();
  });

  it('shows a partial connection and its missing shortcut warning', async () => {
    mocks.addShare.mockResolvedValueOnce({ ...connected, shortcut_path: '', message: 'Операция завершена частично.', warnings: ['Обязательный ярлык не подтверждён.'] });
    render(<SharesPage />);
    fireEvent.change(screen.getByLabelText('UNC-путь'), { target: { value: '\\\\fileserver\\finance' } });
    fireEvent.click(screen.getByRole('button', { name: 'Подключить' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('Обязательный ярлык не подтверждён.')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Операция завершена частично.');
    expect(screen.queryByText(/ярлык создан на рабочем столе/)).not.toBeInTheDocument();
  });
});
