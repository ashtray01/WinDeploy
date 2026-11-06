import { useState } from 'react';
import { Activity, RefreshCw } from 'lucide-react';
import { api, type NetworkReportView } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Spinner } from './ui';
import './network.css';

function statusTone(status: string): 'green' | 'red' | 'amber' | 'muted' {
  if (status === 'ok') return 'green';
  if (status === 'failed') return 'red';
  if (status === 'timeout') return 'amber';
  return 'muted';
}

const statusLabel: Record<string, string> = { ok: 'Успешно', failed: 'Ошибка', timeout: 'Таймаут', skipped: 'Пропущено', unsupported: 'Недоступно' };

export default function NetworkPage() {
  const [dns, setDns] = useState('localhost');
  const [tcp, setTcp] = useState('');
  const [http, setHttp] = useState('');
  const [smb, setSmb] = useState('');
  const [icmp, setIcmp] = useState('');
  const [report, setReport] = useState<NetworkReportView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function run() {
    const targets: string[] = [];
    if (dns.trim()) targets.push(`dns:${dns.trim()}`);
    if (tcp.trim()) targets.push(`tcp:${tcp.trim()}`);
    if (http.trim()) targets.push(`http:${http.trim()}`);
    if (smb.trim()) targets.push(`smb:${smb.trim()}`);
    if (icmp.trim()) targets.push(`icmp:${icmp.trim()}`);
    if (targets.length === 0) {
      setError('Укажите хотя бы одну проверку.');
      return;
    }
    setBusy(true);
    setError('');
    try {
      setReport(await api.runNetworkDiagnostics(targets));
    } catch {
      setError('Не удалось выполнить диагностику сети. Проверьте введённые адреса.');
    } finally {
      setBusy(false);
    }
  }

  return <>
    <PageHeading eyebrow="Сеть" title="Диагностика сети" detail="DNS, DHCP, шлюз, TCP, ICMP, HTTP и SMB. Проверки только читают состояние: параметры адаптеров не изменяются." action={<Button onClick={() => void run()} disabled={busy}><RefreshCw size={15} />{busy ? 'Проверка…' : 'Проверить'}</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    <div className="network-layout">
      <Panel title={<><Activity size={16} /> Параметры проверки</>}>
        <div className="network-form">
          <label><span>DNS-имя</span><input value={dns} onChange={event => setDns(event.target.value)} placeholder="example.com" aria-label="DNS-имя" /></label>
          <label><span>TCP-порт</span><input value={tcp} onChange={event => setTcp(event.target.value)} placeholder="fileserver:445" aria-label="TCP-порт" /></label>
          <label><span>SMB-сервер</span><input value={smb} onChange={event => setSmb(event.target.value)} placeholder="fileserver" aria-label="SMB-сервер" /></label>
          <label><span>HTTP-адрес</span><input value={http} onChange={event => setHttp(event.target.value)} placeholder="https://example.com/" aria-label="HTTP-адрес" /></label>
          <label><span>ICMP-узел</span><input value={icmp} onChange={event => setIcmp(event.target.value)} placeholder="8.8.8.8" aria-label="ICMP-узел" /></label>
        </div>
        <p className="network-note">Проверки DHCP и ICMP шлюза выполняются автоматически для подключённых адаптеров. Пароли и учётные данные не используются.</p>
      </Panel>
      {busy && <Spinner label="Выполнение сетевых проверок" />}
      {!busy && report && <Panel title="Результаты" action={<Badge tone={report.failed === 0 ? 'green' : 'amber'}>{report.failed === 0 ? 'Ошибок нет' : `Ошибок: ${report.failed}`}</Badge>}>
        <p className="network-summary">{report.message}</p>
        {(report.checks ?? []).length === 0 ? <EmptyState title="Проверки не выполнены" detail="Измените параметры и повторите запуск." /> : <div className="network-table-wrap"><table className="network-table">
          <thead><tr><th scope="col">Проверка</th><th scope="col">Статус</th><th scope="col">Результат</th><th scope="col">Время</th></tr></thead>
          <tbody>{(report.checks ?? []).map(check => <tr key={check.id}>
            <td>{check.name}</td>
            <td><Badge tone={statusTone(check.status)}>{statusLabel[check.status] ?? check.status}</Badge></td>
            <td>{check.detail || check.error_code || '—'}</td>
            <td className="network-latency">{check.latency_ms} мс</td>
          </tr>)}</tbody>
        </table></div>}
      </Panel>}
    </div>
  </>;
}
