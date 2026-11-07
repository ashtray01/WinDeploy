import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ExecutionPage from './ExecutionPage';
import type { DeploymentView, DeploySessionView } from './bridge';

const mocks = vi.hoisted(() => ({
  listProfiles: vi.fn(), getProfile: vi.fn(), getDeployment: vi.fn(), listDeploymentSessions: vi.fn(),
  startDeployment: vi.fn(), cancelDeployment: vi.fn(), resumeDeployment: vi.fn(), getDeploymentSession: vi.fn(),
}));
vi.mock('./bridge', () => ({ api: mocks }));

const idle: DeploymentView = { running: false, cancel_requested: false, generation: 0, session_id: '', profile_id: '', profile_name: '', started_at: '', finished_at: '', tasks: [], events: [], message: '', reboot_required: false, cancelled: false, completed: false };
const runningTask = { id: 'dns-check', name: 'Проверить DNS', category: 'diagnostics', status: 'Running', status_label: 'Выполняется', attempts: 0, reboot_required: false, warnings: [] };
const running: DeploymentView = { ...idle, running: true, generation: 1, session_id: 'd1', profile_id: 'office', profile_name: 'Офис', message: 'Выполнение запущено.', tasks: [runningTask] };
const sessions: DeploySessionView[] = [{ id: 'd1', profile_id: 'office', profile_name: 'Офис', updated_at: new Date().toISOString(), total: 1, succeeded: 1, failed: 0, remaining: 0, completed: true, cancelled: false, reboot_required: false }];

describe('deployment execution page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listProfiles.mockResolvedValue({ source: 'test', profiles: [{ id: 'office', name: 'Офис', parent: '' }], errors: [] });
    mocks.getProfile.mockResolvedValue({ id: 'office', name: 'Офис', components: [{ id: 'dns-check', name: 'Проверить DNS', category: 'diagnostics', dependencies: [], requirements: {}, timeout: 30000000000 }] });
    mocks.getDeployment.mockResolvedValue(idle);
    mocks.listDeploymentSessions.mockResolvedValue(sessions);
    mocks.startDeployment.mockResolvedValue(running);
    mocks.cancelDeployment.mockResolvedValue({ ...running, cancel_requested: true });
    mocks.getDeploymentSession.mockResolvedValue({ ...running, running: false, completed: true, tasks: [{ ...runningTask, status: 'Success', status_label: 'Успешно' }] });
  });

  it('requires confirmation and starts the selected profile', async () => {
    render(<ExecutionPage />);
    await waitFor(() => expect(mocks.getProfile).toHaveBeenCalledWith('office'));
    expect(await screen.findByText('Проверить DNS')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Запустить' }));
    expect(await screen.findByText(/Выполнить выбранные компоненты/)).toBeInTheDocument();
    expect(mocks.startDeployment).not.toHaveBeenCalled();
    mocks.getDeployment.mockResolvedValue(running);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить запуск' }));
    await waitFor(() => expect(mocks.startDeployment).toHaveBeenCalledWith('office', ['dns-check']));
    expect((await screen.findAllByText('Выполняется')).length).toBeGreaterThan(0);
  });

  it('shows saved sessions and allows viewing them', async () => {
    render(<ExecutionPage />);
    expect(await screen.findByText(/1 ок \/ 0 ошибок/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Просмотр' }));
    await waitFor(() => expect(mocks.getDeploymentSession).toHaveBeenCalledWith('d1'));
    expect(await screen.findByText('Успешно')).toBeInTheDocument();
  });

  it('cancels a running deployment', async () => {
    mocks.getDeployment.mockResolvedValue(running);
    render(<ExecutionPage />);
    expect(await screen.findByRole('button', { name: 'Отменить' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Отменить' }));
    await waitFor(() => expect(mocks.cancelDeployment).toHaveBeenCalled());
  });

  it('keeps reboot verification pending and offers resume without showing success', async () => {
    mocks.getDeployment.mockResolvedValue({ ...running, running: false, completed: false, reboot_required: true, tasks: [{ ...runningTask, status: 'RebootRequired', status_label: 'Ожидает проверки после перезагрузки', reboot_required: true }] });
    mocks.listDeploymentSessions.mockResolvedValue([{ ...sessions[0], completed: false, succeeded: 0, remaining: 1, reboot_required: true }]);
    render(<ExecutionPage />);
    expect(await screen.findByText('Ожидает проверки')).toBeInTheDocument();
    expect(await screen.findByText(/Зависимые задачи ожидают подтверждения/)).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: 'Прогресс развёртывания' })).toHaveAttribute('aria-valuenow', '0');
    expect(screen.queryByText('Завершено')).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Продолжить' })).toBeInTheDocument();
  });

  it('shows unconfirmed effect warnings for a task', async () => {
    mocks.getDeployment.mockResolvedValue({ ...running, running: false, completed: true, tasks: [{ ...runningTask, status: 'Success', status_label: 'Успешно', warnings: ['Принтер по умолчанию не изменён'] }] });
    render(<ExecutionPage />);
    expect(await screen.findByText('Принтер по умолчанию не изменён')).toBeInTheDocument();
  });
});
