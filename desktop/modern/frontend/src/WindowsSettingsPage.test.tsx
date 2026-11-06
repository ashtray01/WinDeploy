import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import WindowsSettingsPage from './WindowsSettingsPage';
import type { SettingsResult, WindowsSettingsView } from './bridge';

const mocks = vi.hoisted(() => ({
  getWindowsSettings: vi.fn(), getPowerPlanOptions: vi.fn(), applyComputerName: vi.fn(), applyTimezone: vi.fn(),
  applyExplorer: vi.fn(), applyPowerPlan: vi.fn(), getWindowsUpdateDiagnostics: vi.fn(),
}));
vi.mock('./bridge', () => ({ api: mocks }));

const view: WindowsSettingsView = {
  hostname: 'PC-01',
  timezone: 'Russian Standard Time',
  power_plan: '381b4222-f694-41f0-9685-ff5bb260df2e',
  explorer: { Hidden: '2', HideFileExt: '1', ShowSuperHidden: '0', LaunchTo: '2' },
  activation: { genuine: true, status: 'genuine', detail: 'Windows подтверждает подлинность.' },
  elevated: true,
  windows: 'Windows 11',
};
const result: SettingsResult = { changes: [{ name: 'computer-name', old_value: 'PC-01', new_value: 'PC-02', action: 'planned' }], applied: false, reboot_required: false, warnings: [], message: 'Проверка выполнена.' };

describe('windows settings page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getWindowsSettings.mockResolvedValue(view);
    mocks.getPowerPlanOptions.mockResolvedValue([{ guid: '381b4222-f694-41f0-9685-ff5bb260df2e', name: 'Сбалансированная' }]);
    mocks.applyComputerName.mockResolvedValue(result);
    mocks.applyTimezone.mockResolvedValue(result);
    mocks.applyExplorer.mockResolvedValue(result);
    mocks.applyPowerPlan.mockResolvedValue(result);
    mocks.getWindowsUpdateDiagnostics.mockResolvedValue({ checks: [{ name: 'Служба Windows Update', status: 'ok', detail: 'running' }], services: [], message: 'ok' });
  });

  it('shows licensing and previews a rename', async () => {
    render(<WindowsSettingsPage />);
    expect(await screen.findByText('Windows 11')).toBeInTheDocument();
    expect(screen.getByText('Лицензия подтверждена')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Имя компьютера'), { target: { value: 'PC-02' } });
    fireEvent.click(screen.getAllByRole('button', { name: 'Проверить' })[0]);
    await waitFor(() => expect(mocks.applyComputerName).toHaveBeenCalledWith('PC-02', true));
  });

  it('requires confirmation before renaming and reloads state', async () => {
    render(<WindowsSettingsPage />);
    await screen.findByText('Windows 11');
    fireEvent.change(screen.getByLabelText('Имя компьютера'), { target: { value: 'PC-02' } });
    fireEvent.click(screen.getByRole('button', { name: 'Переименовать' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить переименование' }));
    await waitFor(() => expect(mocks.applyComputerName).toHaveBeenCalledWith('PC-02', false));
    await waitFor(() => expect(mocks.getWindowsSettings.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('runs read-only Windows Update diagnostics', async () => {
    render(<WindowsSettingsPage />);
    await screen.findByText('Windows 11');
    fireEvent.click(screen.getByRole('button', { name: 'Диагностика' }));
    await waitFor(() => expect(mocks.getWindowsUpdateDiagnostics).toHaveBeenCalled());
    expect(await screen.findByText('Служба Windows Update')).toBeInTheDocument();
  });
});
