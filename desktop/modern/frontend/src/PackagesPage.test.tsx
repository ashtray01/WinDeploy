import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PackagesPage from './PackagesPage';
import type { PackageCatalogView, PackageInstallView } from './bridge';

const mocks = vi.hoisted(() => ({
  getPackages: vi.fn(),
  getPackageInstall: vi.fn(),
  startPackageInstall: vi.fn(),
  cancelPackageInstall: vi.fn(),
}));
vi.mock('./bridge', () => ({ api: mocks }));

const idleInstall: PackageInstallView = {
  running: false, cancel_requested: false, generation: 0, started_at: '', finished_at: '',
  total: 0, completed: 0, results: [], events: [], message: '', reboot_required: false, cancelled: false,
};

const catalog: PackageCatalogView = {
  source: 'test',
  environment: { elevated: true, winget: true, winget_version: '1.9.0', windows: 'Windows 11', architecture: 'amd64' },
  warnings: [],
  packages: [
    {
      id: 'sevenzip', name: '7-Zip', publisher: 'Igor Pavlov', category: 'utilities', group: 'Архиваторы',
      description: 'Архиватор', requirements: { versions: ['windows-11'], architectures: ['amd64'] },
      requires_admin: true, installed: false, action: 'install', action_label: 'Установить',
      reason: 'Программа не установлена.', source_kind: 'winget', source_version: '26.05', integrity: 'sha256', size_bytes: 1609600,
    },
    {
      id: 'chrome', name: 'Google Chrome', publisher: 'Google LLC', category: 'browsers', group: 'Браузеры',
      description: 'Браузер', requirements: { versions: ['windows-11'], architectures: ['amd64'] },
      requires_admin: true, installed: true, installed_version: '157.0.8103.39', action: 'up-to-date',
      action_label: 'Уже установлено', reason: 'Установленная версия актуальна 157.0.8103.39.',
      source_kind: 'winget', source_version: '157.0.8103.39', integrity: 'sha256',
    },
  ],
};

describe('package manager page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getPackages.mockResolvedValue(catalog);
    mocks.getPackageInstall.mockResolvedValue(idleInstall);
    mocks.startPackageInstall.mockResolvedValue({ ...idleInstall, running: true, total: 1 });
    mocks.cancelPackageInstall.mockResolvedValue({ ...idleInstall, running: false, cancelled: true });
  });

  it('requires an explicit selection and confirmation before starting a real install', async () => {
    render(<PackagesPage query="" />);
    expect(await screen.findByText('7-Zip')).toBeInTheDocument();
    expect(screen.getByText('Google Chrome')).toBeInTheDocument();
    expect(screen.getByText('Уже установлено')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Установить/ })).toBeDisabled();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать 7-Zip' }));
    const installButton = screen.getByRole('button', { name: /Установить \(1\)/ });
    expect(installButton).toBeEnabled();
    fireEvent.click(installButton);
    expect(mocks.startPackageInstall).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить установку' }));
    expect(mocks.startPackageInstall).toHaveBeenCalledWith(['sevenzip']);
  });

  it('blocks system installs without administrator rights', async () => {
    mocks.getPackages.mockResolvedValue({ ...catalog, environment: { ...catalog.environment, elevated: false } });
    render(<PackagesPage query="" />);
    expect(await screen.findByText(/доступна установка только пользовательских программ/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать 7-Zip' }));
    expect(await screen.findByText(/нужны права администратора/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Установить \(1\)/ })).toBeDisabled();
  });

  it('shows live progress and allows cooperative cancellation', async () => {
    mocks.getPackageInstall.mockResolvedValue({
      ...idleInstall, running: true, generation: 1, total: 1, completed: 0, current: 'sevenzip', current_name: '7-Zip',
      message: 'Запуск установки.',
      events: [{ time: new Date().toISOString(), level: 'info', package_id: 'sevenzip', package_name: '7-Zip', message: 'Загрузка установщика…' }],
    });
    render(<PackagesPage query="" />);
    expect(await screen.findByText('Ход установки')).toBeInTheDocument();
    expect(await screen.findByText('Загрузка установщика…')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Отменить' }));
    expect(mocks.cancelPackageInstall).toHaveBeenCalled();
  });

  it('does not show a completion panel before any install request', async () => {
    render(<PackagesPage query="" />);
    await screen.findByText('7-Zip');
    expect(screen.queryByRole('heading', { name: 'Ход установки' })).not.toBeInTheDocument();
    expect(screen.queryByText('Завершено')).not.toBeInTheDocument();
  });

  it('does not show a green completion when a package failed', async () => {
    mocks.getPackageInstall.mockResolvedValue({
      ...idleInstall, total: 1, completed: 1, message: 'Установка завершена с ошибками. Успешно: 0, ошибок: 1.',
      results: [{ package_id: 'sevenzip', name: '7-Zip', status: 'failed', status_label: 'Ошибка', action: 'install', attempts: 1, started_at: '', finished_at: '', error: 'Установщик завершился с ошибкой.' }],
    });
    render(<PackagesPage query="" />);
    expect(await screen.findByText('Завершено с ошибками')).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: 'Прогресс установки' })).toHaveAttribute('aria-valuenow', '100');
  });

  it('filters the catalog by the shared search query', async () => {
    render(<PackagesPage query="chrome" />);
    expect(await screen.findByText('Google Chrome')).toBeInTheDocument();
    expect(screen.queryByText('7-Zip')).not.toBeInTheDocument();
  });
});
