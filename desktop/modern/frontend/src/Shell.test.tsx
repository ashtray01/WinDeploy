import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import Shell from './Shell';

describe('desktop navigation', () => {
  it('changes available pages and keeps future modules unavailable', () => {
    const navigate = vi.fn();
    render(<Shell page="overview" navigate={navigate}><p>Область содержимого</p></Shell>);
    fireEvent.click(screen.getByRole('button', { name: 'Профили' }));
    expect(navigate).toHaveBeenCalledWith('profiles');
    fireEvent.click(screen.getByRole('button', { name: 'Установка ПО' }));
    expect(navigate).toHaveBeenCalledWith('packages');
    fireEvent.click(screen.getByRole('button', { name: 'Браузеры' }));
    expect(navigate).toHaveBeenCalledWith('browsers');
    fireEvent.click(screen.getByRole('button', { name: 'Windows' }));
    expect(navigate).toHaveBeenCalledWith('winsettings');
    expect(screen.getByRole('button', { name: 'Обзор' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByText(/проверка SHA-256 и подписи/)).toBeInTheDocument();
  });
});
