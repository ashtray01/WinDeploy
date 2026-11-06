import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import DriversPage from './DriversPage';
import type { DriverDevice, DriverInfo, DriverResult } from './bridge';

const mocks = vi.hoisted(() => ({ getDevices: vi.fn(), scanDriverCatalog: vi.fn(), installDriver: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocks }));

const problemHWID = 'PCI\\VEN_1234&DEV_5678';
const problem: DriverDevice = { instance_id: problemHWID + '\\1', name: 'Unknown device', class: '', manufacturer: '', hardware_ids: [problemHWID], status: 'problem', problem_code: 28, unknown: true };
const healthy: DriverDevice = { instance_id: 'PCI\\VEN_AAAA&DEV_BBBB\\2', name: 'Video controller', class: 'Display', manufacturer: 'Example', hardware_ids: ['PCI\\VEN_AAAA&DEV_BBBB'], driver_version: '1.0.0.0', driver_provider: 'Example', status: 'ok', unknown: false };
const inf: DriverInfo = { path: 'C:\\drivers\\device.inf', provider: 'Example Corp', version: '2.0.0.0', class: 'Display', hardware_ids: ['PCI\\VEN_1234&DEV_5678'] };
const signed: DriverResult = { inf_path: inf.path, hardware_id: problemHWID, signature: { verified: true, signer: 'Example Corp', catalog: 'device.cat' }, installed: false, reboot_required: false, exit_code: 0, message: 'Проверка выполнена: подпись принята, установка не запускалась.' };

describe('drivers page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getDevices.mockResolvedValue([problem, healthy]);
    mocks.scanDriverCatalog.mockResolvedValue([inf]);
    mocks.installDriver.mockResolvedValue(signed);
  });

  it('shows problem devices by default and selects hardware ID', async () => {
    render(<DriversPage query="" />);
    expect(await screen.findByText('Unknown device')).toBeInTheDocument();
    expect(screen.queryByText('Video controller')).not.toBeInTheDocument();
    fireEvent.click(screen.getByText('Unknown device'));
    expect(screen.getByLabelText('Hardware ID')).toHaveValue('PCI\\VEN_1234&DEV_5678');
  });

  it('scans a driver folder and verifies the INF without installing', async () => {
    render(<DriversPage query="" />);
    await screen.findByText('Unknown device');
    fireEvent.click(screen.getByText('Unknown device'));
    fireEvent.click(screen.getByRole('button', { name: 'Сканировать' }));
    await waitFor(() => expect(mocks.scanDriverCatalog).toHaveBeenCalled());
    fireEvent.click(await screen.findByText(/Example Corp/));
    expect(screen.getByLabelText('INF-файл')).toHaveValue(inf.path);
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.installDriver).toHaveBeenCalledWith({ inf_path: inf.path, hardware_id: problemHWID, dry_run: true }));
    expect(await screen.findByText(/подпись принята/)).toBeInTheDocument();
  });

  it('requires a confirmation before a real driver install', async () => {
    render(<DriversPage query="" />);
    await screen.findByText('Unknown device');
    fireEvent.click(screen.getByText('Unknown device'));
    fireEvent.change(screen.getByLabelText('INF-файл'), { target: { value: inf.path } });
    fireEvent.click(screen.getByRole('button', { name: 'Установить' }));
    expect(await screen.findByText(/Установка изменит драйвер/)).toBeInTheDocument();
    expect(mocks.installDriver).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить установку' }));
    await waitFor(() => expect(mocks.installDriver).toHaveBeenCalledWith({ inf_path: inf.path, hardware_id: problemHWID, dry_run: false }));
  });
});
