import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { State } from './bridge';
import { useInventory } from './useInventory';

const mocked = vi.hoisted(() => ({ state: vi.fn(), startScan: vi.fn(), cancelScan: vi.fn() }));
vi.mock('./bridge', () => ({ api: mocked }));
afterEach(() => vi.resetAllMocks());
const idle: State = { generation: 0, scanning: false, cancel_requested: false, status: 'idle', report: null, error: '', journal: [] };

describe('real bridge state handling', () => {
  it('requests backend scan without inventing a report', async () => {
    mocked.state.mockResolvedValue(idle);
    mocked.startScan.mockResolvedValue({ ...idle, scanning: true, generation: 1, status: 'scanning' });
    const { result, unmount } = renderHook(useInventory);
    await waitFor(() => expect(mocked.startScan).toHaveBeenCalledOnce());
    expect(result.current.state?.report).toBeNull();
    await act(async () => { mocked.cancelScan.mockResolvedValue({ ...idle, generation: 1, status: 'cancelled' }); await result.current.cancel(); });
    expect(result.current.state?.status).toBe('cancelled');
    unmount();
  });
  it('shows connection failure rather than fixture data', async () => {
    mocked.state.mockRejectedValue(new Error('transport unavailable'));
    const { result } = renderHook(useInventory);
    await waitFor(() => expect(result.current.connectionError).toContain('Не удалось связаться'));
    expect(result.current.state).toBeNull();
    expect(mocked.startScan).not.toHaveBeenCalled();
  });
});
