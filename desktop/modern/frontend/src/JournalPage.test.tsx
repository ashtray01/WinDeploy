import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import JournalPage from './JournalPage';
import type { State } from './bridge';
test('journal displays supplied events without invented success', () => {
  const state = { journal: [{ id: 7, time: '2026-11-06T12:00:00Z', level: 'warning', source: 'inventory', message: 'Источник недоступен' }] } as State;
  render(<JournalPage state={state} query="" onQuery={() => {}} refresh={async () => {}} />);
  expect(screen.getByText('Источник недоступен')).toBeInTheDocument();
  expect(screen.getByText('Предупреждение')).toBeInTheDocument();
  expect(screen.queryByText('Success')).not.toBeInTheDocument();
});
