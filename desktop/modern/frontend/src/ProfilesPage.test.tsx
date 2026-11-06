import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, test, vi } from 'vitest';
import ProfilesPage, { DAG } from './ProfilesPage';
import { api } from './bridge';
vi.mock('./bridge', () => ({ api: { listProfiles: vi.fn(), getProfile: vi.fn(), previewProfile: vi.fn() } }));
const task = { id: 'first', name: 'Первый', category: 'diagnostics', requirements: {}, timeout: 0 };
test('DAG draws only real dependency edges', () => {
  render(<DAG tasks={[{ task, compatibility: 'compatible', reason: '' }, { task: { ...task, id: 'second', dependencies: ['first'] }, compatibility: 'blocked', reason: '' }]} />);
  expect(screen.getAllByTestId('dag-edge')).toHaveLength(1);
});
test('profile selections call the real bridge contract and render incompatible preview', async () => {
  vi.mocked(api.listProfiles).mockResolvedValue({ source: 'Тестовый источник', profiles: [{ id: 'test', name: 'Тест', parent: '' }], errors: [] });
  vi.mocked(api.getProfile).mockResolvedValue({ id: 'test', name: 'Тест', components: [task] });
  vi.mocked(api.previewProfile).mockResolvedValue({ scope: 'structure-and-dag-only', tasks: [{ task, compatibility: 'incompatible', reason: 'Не совпадает ОС' }] });
  render(<ProfilesPage query="" onActivity={() => {}} />);
  await screen.findByRole('checkbox');
  fireEvent.click(screen.getByRole('button', { name: 'Построить план' }));
  await screen.findByText('Несовместимо');
  expect(api.previewProfile).toHaveBeenCalledWith('test', ['first']);
  expect(screen.queryByText('Success')).not.toBeInTheDocument();
});
test('failed catalog has an error, no invented profiles', async () => {
  vi.mocked(api.listProfiles).mockRejectedValue(new Error('unavailable'));
  render(<ProfilesPage query="" onActivity={() => {}} />);
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Не удалось прочитать'));
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
});
