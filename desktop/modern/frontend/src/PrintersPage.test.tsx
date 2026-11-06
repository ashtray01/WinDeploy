import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PrintersPage from './PrintersPage';
import type { PrinterInfo, PrinterResult } from './bridge';

const mocks = vi.hoisted(() => ({ getPrinters: vi.fn(), addPrinter: vi.fn(), setDefaultPrinter: vi.fn(), printTestPage: vi.fn(), getKyocera2460Models: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocks }));

const printers: PrinterInfo[] = [
  { name: 'Office Printer', driver: 'Generic / Text Only', port: 'IP_192.168.1.50', server: '', shared: false, default: true, status: 'ok' },
  { name: 'Warehouse', driver: 'Kyocera', port: 'IP_192.168.1.60', server: '', shared: true, share_name: 'WH', default: false, status: 'offline' },
];
const added: PrinterResult = { name: 'New Printer', installed: true, default: true, test_printed: false, reboot_needed: false, message: 'Принтер добавлен.', warnings: [] };

describe('printers page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getPrinters.mockResolvedValue(printers);
    mocks.getKyocera2460Models.mockResolvedValue(['TASKalfa 2460ci', 'TASKalfa 2460ci+']);
    mocks.addPrinter.mockResolvedValue(added);
    mocks.setDefaultPrinter.mockResolvedValue(undefined);
    mocks.printTestPage.mockResolvedValue(undefined);
  });

  it('lists printers with default state and sets a default printer', async () => {
    render(<PrintersPage />);
    expect(await screen.findByText('Office Printer')).toBeInTheDocument();
    expect((await screen.findAllByText('По умолчанию')).length).toBeGreaterThan(0);
    const buttons = screen.getAllByRole('button', { name: 'По умолчанию' });
    const available = buttons.find(button => !button.hasAttribute('disabled'));
    expect(available).toBeTruthy();
    fireEvent.click(available!);
    await waitFor(() => expect(mocks.setDefaultPrinter).toHaveBeenCalledWith('Warehouse'));
  });

  it('requires confirmation before sending a real test page', async () => {
    render(<PrintersPage />);
    await screen.findByText('Office Printer');
    fireEvent.click(screen.getAllByRole('button', { name: 'Тестовая страница' })[0]);
    expect(mocks.printTestPage).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить печать' }));
    await waitFor(() => expect(mocks.printTestPage).toHaveBeenCalledWith('Office Printer'));
  });

  it('adds a Kyocera 2460 printer only with an explicit model', async () => {
    render(<PrintersPage />);
    await screen.findByText('Office Printer');
    fireEvent.change(screen.getByLabelText('Имя принтера'), { target: { value: 'Kyocera 2460 приемная' } });
    fireEvent.change(screen.getByLabelText('IP-адрес принтера'), { target: { value: '192.168.1.70' } });
    fireEvent.change(screen.getByLabelText('INF-драйвер принтера'), { target: { value: 'C:\\drivers\\kyocera.inf' } });
    fireEvent.click(screen.getByLabelText('Kyocera 2460'));
    await waitFor(() => expect(mocks.getKyocera2460Models).toHaveBeenCalled());
    const modelSelect = await screen.findByLabelText('Модель Kyocera');
    await waitFor(() => expect(modelSelect).toHaveValue('TASKalfa 2460ci'));
    fireEvent.click(screen.getByRole('button', { name: 'Проверить' }));
    await waitFor(() => expect(mocks.addPrinter).toHaveBeenCalled());
    const request = mocks.addPrinter.mock.calls[0][0];
    expect(request.model).toBe('TASKalfa 2460ci');
    expect(request.dry_run).toBe(true);
  });
});
