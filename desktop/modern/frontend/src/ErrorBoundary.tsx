import { Component, type ReactNode } from 'react';
import { Button, EmptyState, Panel } from './ui';

export default class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    if (this.state.failed) return <main className="app-main" role="alert"><Panel><EmptyState title="Не удалось отобразить интерфейс" detail="Перезапустите окно. Данные не заменены примерами, системные изменения не выполнялись." /><div className="profile-actions"><Button onClick={() => window.location.reload()}>Перезагрузить окно</Button></div></Panel></main>;
    return this.props.children;
  }
}
