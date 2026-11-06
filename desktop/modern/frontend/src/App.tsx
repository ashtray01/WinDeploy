import { EmptyState, PageHeading, Panel } from './ui';
export default function App() {
  return <main style={{ padding: 36 }}><PageHeading eyebrow="Подготовка рабочих станций" title="WinDeploy" detail="Modern · режим просмотра" /><Panel title="Локальный компьютер"><EmptyState title="Источник данных ещё не подключён" detail="Интерфейс не показывает результаты до получения настоящего Inventory." /></Panel></main>;
}
