import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import BrowsersPage from './BrowsersPage';
import type { BrowserResult } from './bridge';

const mocks = vi.hoisted(() => ({ getBrowserEnvironment: vi.fn(), applyBrowserPolicy: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocks, BrowserKind: { Chrome: 'chrome', Edge: 'edge', Firefox: 'firefox' }, BrowserScope: { ScopeUser: 'user', ScopeMachine: 'machine' } }));

const result = { applied: [{ browser: 'chrome', scope: 'user', name: 'HomepageLocation', action: 'planned' }], shortcuts: [], warnings: [], dry_run: true } as unknown as BrowserResult;

describe('browser policies page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getBrowserEnvironment.mockResolvedValue({ chrome: 'C:\\Chrome\\chrome.exe', edge: 'C:\\Edge\\msedge.exe', desktop: 'C:\\Desktop', elevated: false });
    mocks.applyBrowserPolicy.mockResolvedValue(result);
  });

  it('previews supported policies with the real bridge', async () => {
    render(<BrowsersPage />);
    expect(await screen.findByText('Google Chrome')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Домашняя страница'), { target: { value: 'https://intranet.example/' } });
    fireEvent.change(screen.getByLabelText('Стартовые вкладки'), { target: { value: 'https://intranet.example/\nhttps://mail.example/' } });
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.applyBrowserPolicy).toHaveBeenCalledTimes(1));
    const policy = mocks.applyBrowserPolicy.mock.calls[0][0];
    expect(policy.browser).toBe('chrome');
    expect(policy.scope).toBe('user');
    expect(policy.homepage).toBe('https://intranet.example/');
    expect(policy.startup_urls).toEqual(['https://intranet.example/', 'https://mail.example/']);
    expect(mocks.applyBrowserPolicy.mock.calls[0][1]).toBe(true);
  });

  it('requires confirmation before applying a machine policy', async () => {
    render(<BrowsersPage />);
    fireEvent.change(screen.getByLabelText('Область политики'), { target: { value: 'machine' } });
    fireEvent.click(screen.getByRole('button', { name: 'Применить политику' }));
    expect(await screen.findByText(/Изменить политики Google Chrome/)).toBeInTheDocument();
    expect(mocks.applyBrowserPolicy).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));
    await waitFor(() => expect(mocks.applyBrowserPolicy).toHaveBeenCalled());
    expect(mocks.applyBrowserPolicy.mock.calls[0][0].scope).toBe('machine');
    expect(mocks.applyBrowserPolicy.mock.calls[0][1]).toBe(false);
  });

  it('removes the previous success when a required effect fails on the next request', async () => {
    mocks.applyBrowserPolicy.mockResolvedValueOnce({ ...result, dry_run: false }).mockRejectedValueOnce('Обязательные ярлыки не созданы.');
    render(<BrowsersPage />);
    fireEvent.click(screen.getByRole('button', { name: 'Применить политику' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByText('Политика применена.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Применить политику' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Обязательные ярлыки не созданы.');
    expect(screen.queryByText('Политика применена.')).not.toBeInTheDocument();
  });
});
