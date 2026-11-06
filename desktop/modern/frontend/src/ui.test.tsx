import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Badge, Button, EmptyState, Progress } from './ui';

describe('design system accessibility', () => {
  it('keeps unavailable actions disabled', () => {
    const clicked = vi.fn();
    render(<Button disabled onClick={clicked}>Установить</Button>);
    fireEvent.click(screen.getByRole('button', { name: 'Установить' }));
    expect(clicked).not.toHaveBeenCalled();
  });
  it('does not invent a progress percentage when unknown', () => {
    const { rerender } = render(<Progress value={null} label="Инвентаризация" />);
    expect(screen.getByRole('progressbar')).not.toHaveAttribute('aria-valuenow');
    rerender(<Progress value={140} label="Инвентаризация" />);
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100');
  });
  it('renders explicit empty/error meaning', () => {
    render(<><Badge tone="amber">Частичные данные</Badge><EmptyState title="Нет данных" detail="Выполните инвентаризацию" /></>);
    expect(screen.getByText('Частичные данные')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Нет данных' })).toBeInTheDocument();
  });
});
