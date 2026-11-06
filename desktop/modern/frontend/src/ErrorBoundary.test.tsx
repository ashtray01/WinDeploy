import { render, screen } from '@testing-library/react';
import { expect, test, vi } from 'vitest';
import ErrorBoundary from './ErrorBoundary';
test('render failure shows a recovery action instead of an empty window', () => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
  function Broken(): never { throw new Error('test render failure'); }
  render(<ErrorBoundary><Broken /></ErrorBoundary>);
  expect(screen.getByRole('alert')).toHaveTextContent('Не удалось отобразить интерфейс');
  expect(screen.getByRole('button', { name: 'Перезагрузить окно' })).toBeEnabled();
});
