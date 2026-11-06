import { useCallback, useEffect, useRef, useState } from 'react';
import { History, Play, RefreshCw, Square } from 'lucide-react';
import { api, type Catalog, type DeploymentView, type DeploySessionView, type ProfileDetails } from './bridge';
import { Badge, Button, EmptyState, PageHeading, Panel, Progress, Spinner } from './ui';
import './deploy.css';

function taskTone(status: string): 'green' | 'red' | 'amber' | 'blue' | 'muted' {
  switch (status) {
    case 'Success': case 'AlreadyInstalled': return 'green';
    case 'Failed': return 'red';
    case 'Skipped': case 'NotApplicable': case 'Cancelled': return 'amber';
    case 'Running': return 'blue';
    default: return 'muted';
  }
}

export default function ExecutionPage() {
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [profileID, setProfileID] = useState('');
  const [details, setDetails] = useState<ProfileDetails | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [deployment, setDeployment] = useState<DeploymentView | null>(null);
  const [sessions, setSessions] = useState<DeploySessionView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [resumeID, setResumeID] = useState('');
  const [retryFailed, setRetryFailed] = useState(false);
  const [viewedSession, setViewedSession] = useState<DeploymentView | null>(null);
  const runningRef = useRef(false);
  const busy = deployment?.running ?? false;

  const loadSessions = useCallback(async () => {
    try { setSessions((await api.listDeploymentSessions()) ?? []); } catch { setSessions([]); }
  }, []);

  useEffect(() => {
    let active = true;
    setLoading(true);
    api.listProfiles().then(data => {
      if (!active) return;
      setCatalog(data);
      const first = data.profiles?.[0]?.id ?? '';
      setProfileID(first);
    }).catch(() => { if (active) setError('Не удалось прочитать профили.'); })
      .finally(() => { if (active) setLoading(false); });
    void api.getDeployment().then(view => { if (active) { setDeployment(view); runningRef.current = view?.running ?? false; } }).catch(() => undefined);
    void loadSessions();
    return () => { active = false; };
  }, [loadSessions]);

  useEffect(() => {
    if (!profileID) { setDetails(null); return; }
    let active = true;
    api.getProfile(profileID).then(data => {
      if (!active) return;
      setDetails(data);
      setSelected((data.components ?? []).map(component => component.id));
    }).catch(() => { if (active) setDetails(null); });
    return () => { active = false; };
  }, [profileID]);

  useEffect(() => {
    let active = true;
    const timer = window.setInterval(async () => {
      try {
        const next = await api.getDeployment();
        if (!active) return;
        setDeployment(next);
        if (runningRef.current && !next.running) { runningRef.current = false; void loadSessions(); }
        runningRef.current = next.running;
      } catch { /* transient bridge errors must not erase progress */ }
    }, 800);
    return () => { active = false; window.clearInterval(timer); };
  }, [loadSessions]);

  async function start() {
    setError('');
    try {
      setViewedSession(null);
      setDeployment(await api.startDeployment(profileID, selected));
      setConfirming(false);
      runningRef.current = true;
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось запустить развёртывание.');
    }
  }

  async function cancel() {
    try { setDeployment(await api.cancelDeployment()); } catch { setError('Не удалось запросить отмену.'); }
  }

  async function resume() {
    if (!resumeID) return;
    setError('');
    try {
      setViewedSession(null);
      setDeployment(await api.resumeDeployment(resumeID, retryFailed));
      setResumeID(''); setConfirming(false); runningRef.current = true;
    } catch (caught) {
      setError(typeof caught === 'string' ? caught : 'Не удалось продолжить сессию.');
    }
  }

  async function viewSession(id: string) {
    try { setViewedSession(await api.getDeploymentSession(id)); } catch { setError('Сессия недоступна.'); }
  }

  const active = viewedSession ?? deployment;
  const completed = active ? active.tasks?.filter(task => ['Success', 'AlreadyInstalled', 'NotApplicable'].includes(task.status)).length ?? 0 : 0;
  const total = active?.tasks?.length ?? 0;

  return <>
    <PageHeading eyebrow="Развёртывание" title="Выполнение профиля" detail="Плановое выполнение компонентов с зависимостями, отменой, сохранением состояния, продолжением после перезагрузки и отчётом HTML/JSON." action={busy ? <Button variant="danger" onClick={() => void cancel()} disabled={deployment?.cancel_requested}><Square size={14} />{deployment?.cancel_requested ? 'Отмена…' : 'Отменить'}</Button> : <Button onClick={() => { void loadSessions(); }}><RefreshCw size={15} />Сессии</Button>} />
    {error && <div className="callout red" role="alert">{error}</div>}
    {loading ? <Spinner label="Чтение профилей" /> : <div className="deploy-layout">
      <Panel title={<><Play size={16} /> Профиль и компоненты</>}>
        <div className="deploy-form">
          <label><span>Профиль</span><select value={profileID} onChange={event => setProfileID(event.target.value)} aria-label="Профиль для выполнения">{(catalog?.profiles ?? []).map(profile => <option key={profile.id} value={profile.id}>{profile.name}</option>)}</select></label>
        </div>
        {details ? <div className="deploy-components">{(details.components ?? []).map(component => <label key={component.id}><input type="checkbox" checked={selected.includes(component.id)} disabled={busy} onChange={event => setSelected(previous => event.target.checked ? [...previous, component.id] : previous.filter(id => id !== component.id))} /><span><strong>{component.name || component.id}</strong><small>{component.id} · {component.category}{component.dependencies?.length ? ` · зависит от ${component.dependencies.join(', ')}` : ''}</small></span></label>)}</div>
          : <EmptyState title="Профиль не выбран" detail="Выберите профиль для построения плана." />}
        <div className="deploy-actions">
          <span>Выбрано {selected.length} из {details?.components?.length ?? 0}</span>
          {!confirming ? <Button variant="primary" disabled={busy || !profileID || selected.length === 0} onClick={() => setConfirming(true)}><Play size={14} />Запустить</Button>
            : <><span className="deploy-confirm">Выполнить выбранные компоненты? Изменяющие операции требуют прав администратора.</span><Button variant="danger" disabled={busy} onClick={() => void start()}>Подтвердить запуск</Button><Button variant="ghost" onClick={() => setConfirming(false)}>Отмена</Button></>}
        </div>
      </Panel>
      <Panel title="Ход выполнения" action={<Badge tone={active?.running ? 'blue' : active?.cancelled ? 'amber' : active?.completed ? 'green' : 'muted'}>{active?.running ? 'Выполняется' : active?.cancelled ? 'Отменено' : active?.completed ? 'Завершено' : 'Нет запуска'}</Badge>}>
        {active && total > 0 ? <>
          <div className="deploy-progress"><div><strong>{active.profile_name}</strong><small>{active.message}</small></div><span>{completed}/{total}</span></div>
          <Progress value={total > 0 ? Math.round((completed / total) * 100) : 0} label="Прогресс развёртывания" />
          {active.reboot_required && <div className="callout">Требуется перезагрузка Windows. После перезагрузки продолжите сессию.</div>}
          <div className="deploy-table-wrap"><table className="deploy-table"><thead><tr><th scope="col">Задача</th><th scope="col">Статус</th><th scope="col">Попытки</th><th scope="col">Диагностика</th></tr></thead><tbody>{active.tasks?.map(task => <tr key={task.id}><td><div className="deploy-task"><strong>{task.name}</strong><small>{task.category}</small></div></td><td><Badge tone={taskTone(task.status)}>{task.status_label}</Badge>{task.reboot_required && <small className="deploy-reboot">+ перезагрузка</small>}</td><td>{task.attempts}</td><td>{task.error || '—'}</td></tr>)}</tbody></table></div>
          {(active.report_html || active.report_json) && <div className="deploy-report"><small>Отчёт</small><p className="mono">{active.report_html}</p><p className="mono">{active.report_json}</p></div>}
          {(active.events ?? []).length > 0 && <div className="deploy-events">{(active.events ?? []).slice(-30).map((event, index) => <div key={index} className={`deploy-event ${event.level}`}><time>{new Date(event.time).toLocaleTimeString('ru-RU')}</time><span>{event.message}</span></div>)}</div>}
        </> : <EmptyState title="Запусков пока нет" detail="Выберите профиль и запустите выполнение." />}
      </Panel>
      <Panel title={<><History size={16} /> Сохранённые сессии</>} action={<Button className="small" onClick={() => void loadSessions()}><RefreshCw size={13} />Обновить</Button>}>
        {sessions.length === 0 ? <EmptyState title="Сессий нет" detail="Выполненные и прерванные сессии появятся здесь." /> : <div className="deploy-table-wrap"><table className="deploy-table"><thead><tr><th scope="col">Профиль</th><th scope="col">Итог</th><th scope="col">Обновлено</th><th scope="col">Действия</th></tr></thead><tbody>{sessions.map(session => <tr key={session.id}>
          <td><div className="deploy-task"><strong>{session.profile_name}</strong><small>{session.id}</small></div></td>
          <td><Badge tone={session.completed ? 'green' : session.cancelled ? 'amber' : 'muted'}>{session.completed ? 'Завершена' : session.cancelled ? 'Отменена' : 'Не завершена'}</Badge><small className="deploy-reboot">{session.succeeded} ок / {session.failed} ошибок / {session.remaining} осталось</small></td>
          <td>{new Date(session.updated_at).toLocaleString('ru-RU')}</td>
          <td><div className="row-actions"><Button className="small" onClick={() => void viewSession(session.id)}>Просмотр</Button>{resumeID === session.id ? <><label className="deploy-check"><input type="checkbox" checked={retryFailed} onChange={event => setRetryFailed(event.target.checked)} />повторить ошибки</label><Button variant="danger" className="small" onClick={() => void resume()}>Подтвердить</Button></> : !session.completed || session.failed > 0 ? <Button className="small" onClick={() => setResumeID(session.id)}>Продолжить</Button> : null}</div></td>
        </tr>)}</tbody></table></div>}
      </Panel>
    </div>}
  </>;
}
