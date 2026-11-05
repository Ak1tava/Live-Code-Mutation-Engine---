import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { QRCodeSVG } from 'qrcode.react'
import { errorText, post } from '../api'
import { AuditorActivity } from '../components/AuditorActivity'
import { MutationCard } from '../components/MutationCard'
import { ReportView } from '../components/ReportView'
import { Alert, Connection, Header, Loading, SessionProblem } from '../components/Shell'
import { Timer } from '../components/Timer'
import { useCountdown } from '../timer'
import { states, type AuditorView } from '../types'
import { useSession } from '../ws'

const CodeDiff = lazy(() => import('../components/Monaco').then(module => ({ default: module.CodeDiff })))

const stages = [
  ['baseline', 'Проверяем оригинал'],
  ['candidates', 'Ищем мутации'],
  ['triples', 'Проверяем независимость'],
  ['done', 'Готовим подсказки'],
] as const

function Preparation({ progress }: { progress: AuditorView['progress'] }) {
  const stage = stages.findIndex(([name]) => name === progress.stage)
  return <main className="narrow page">
    <section className="panel stage-card">
      <span className="eyebrow">Подготовка задания</span>
      <h1>{stages[stage]?.[1] ?? 'Загружаем проект'}</h1>
      <p className="lead">Система проверит тесты и подберёт три ошибки, которые можно исправить независимо друг от друга.</p>
      {progress.total > 0 ? <div className="progress-group">
        <progress max={progress.total} value={progress.done} aria-label="Прогресс текущего этапа" />
        <span>{progress.done} из {progress.total}</span>
      </div> : <Loading>Подготовка в процессе…</Loading>}
      <ol className="prep-steps">{stages.map(([name, label], index) => <li key={name} className={index === stage ? 'current' : ''}>
        <span>{index < stage ? '✓' : `0${index + 1}`}</span>{label}
      </li>)}</ol>
    </section>
  </main>
}

function Invitation({ url }: { url: string }) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)
  const link = useRef<HTMLInputElement>(null)
  const absolute = new URL(url, window.location.origin).href

  useEffect(() => {
    if (!copied) return
    const timeout = window.setTimeout(() => setCopied(false), 2000)
    return () => window.clearTimeout(timeout)
  }, [copied])

  async function copy() {
    try {
      await navigator.clipboard.writeText(absolute)
      setCopied(true)
      setCopyError(false)
    } catch {
      link.current?.select()
      setCopyError(true)
    }
  }

  return <section className="panel invitation" aria-label="Приглашение студента">
    <div className="qr-code"><QRCodeSVG value={absolute} size={76} marginSize={2} title="QR-код ссылки студента" /></div>
    <div>
      <h2>Ссылка для студента</h2>
      <div className="invitation-controls">
        <input ref={link} aria-label="Ссылка для студента" readOnly value={absolute} onFocus={event => event.currentTarget.select()} />
        <button type="button" className="button secondary small" onClick={() => void copy()}>{copied ? 'Скопировано' : 'Скопировать'}</button>
        <a href={absolute} target="_blank" rel="noreferrer" className="button ghost small">Открыть ↗</a>
      </div>
      {copyError && <p className="fine-print" role="status">Выделенную ссылку можно скопировать вручную.</p>}
    </div>
  </section>
}

export default function AuditorRoute() {
  const { tok = '' } = useParams()
  return <Auditor key={tok} tok={tok} />
}

function Auditor({ tok }: { tok: string }) {
  const { view, connected, snapshotVersion, getSnapshotVersion, fatal, error: wireError, clearError } = useSession<AuditorView>(tok, 'a')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)
  const [requestedFinish, setRequestedFinish] = useState<number | null>(null)
  const [confirmFinish, setConfirmFinish] = useState(false)
  const [selected, setSelected] = useState<{ path: string; line?: number }>()
  const seconds = useCountdown(view?.deadline, view?.serverNow)
  const endpoint = `/api/a/${encodeURIComponent(tok)}`

  useEffect(() => {
    if (requestedFinish !== null && snapshotVersion > requestedFinish && !view?.finishing) setRequestedFinish(null)
  }, [requestedFinish, snapshotVersion, view?.finishing])

  async function finish() {
    const snapshot = getSnapshotVersion()
    if (await perform('finish')) {
      setRequestedFinish(snapshot)
      setConfirmFinish(false)
    }
  }

  async function perform(path: string, body?: unknown) {
    if (pending.current || !connected) return false
    pending.current = true
    setBusy(true)
    setError('')
    clearError()
    try {
      await post(`${endpoint}/${path}`, body)
      return true
    } catch (cause) {
      setError(errorText(cause))
      return false
    } finally {
      pending.current = false
      setBusy(false)
    }
  }

  const header = <Header role="Аудитор"><Connection connected={connected} /></Header>
  if (fatal) return <>{header}<SessionProblem message={fatal} /></>
  if (!view) return <>{header}<main className="narrow page"><Loading>Подключаемся к аудиту…</Loading></main></>
  if (view.state === 'preparing') return <>{header}<Preparation progress={view.progress} /></>
  if (view.state === 'failed') return <>{header}<main className="narrow page"><section className="panel stage-card">
    <span className="eyebrow">Подготовка задания</span><h1>Не удалось подготовить аудит</h1>
    <Alert>{view.error || 'Сервер не смог подготовить задание.'}</Alert>
    <Link to="/" className="button primary">Другой репозиторий</Link>
  </section></main></>
  const mutations = view.mutations ?? []
  const paths = [...new Set([...mutations.map(mutation => mutation.file), ...Object.keys(view.mutated ?? {}).sort()])]
  const file = selected && paths.includes(selected.path) ? selected.path : paths[0]
  const current = view.state === 'running'
  const finished = view.state === 'finished'
  const ending = !!view.finishing || (requestedFinish !== null && snapshotVersion <= requestedFinish) || (current && seconds === 0)
  const lastRun = view.runs?.at(-1)
  const passedTests = new Set((lastRun?.result.tests ?? []).filter(test => test.status === 'pass').map(test => `${test.pkg}:${test.name}`))
  const focus = selected?.path === file ? selected?.line : undefined
  const lines = mutations.filter(mutation => mutation.file === file).map(mutation => mutation.line)
  if (focus !== undefined) lines.sort((left, right) => Math.abs(left - focus) - Math.abs(right - focus))

  return <>{header}<main className="page workspace auditor">
    {finished && <ReportView audience="auditor" report={view.report} />}
    <div className="page-title">
      <div><span className="eyebrow">Рабочее место аудитора</span><h1>{finished ? 'Разбор сессии' : 'Наблюдение'}</h1></div>
      <div className="title-actions">
        <span className="badge badge-neutral">{ending ? 'Завершается' : states[view.state]}</span>
        {!finished && <Timer seconds={current ? seconds : 600} active={current} />}
        {!finished && <button className="button secondary" disabled={!connected || !current || ending || busy}
          onClick={() => setConfirmFinish(true)}>Завершить</button>}
        {finished && view.report?.verificationError && <button className="button primary" disabled={!connected || ending || busy}
          onClick={() => void finish()}>{ending || busy ? 'Проверяем…' : 'Повторить проверку'}</button>}
      </div>
    </div>
    {confirmFinish && current && !ending && <section className="panel finish-confirm" role="region" aria-label="Подтверждение завершения">
      <p>Завершить сессию? Студент больше не сможет редактировать код.</p>
      <div className="inline-actions">
        <button className="button primary small" disabled={!connected || busy} onClick={() => void finish()}>Да, завершить</button>
        <button className="button secondary small" disabled={busy} onClick={() => setConfirmFinish(false)}>Продолжить работу</button>
      </div>
    </section>}
    {!connected && <Alert tone="warning">Связь прервалась. Показываем последние полученные данные.</Alert>}
    {ending && <Alert tone="warning">Выполняется финальный прогон. Отчёт появится автоматически.</Alert>}
    <Alert>{error || wireError}</Alert>
    {!finished && view.studentUrl && <Invitation url={view.studentUrl} />}
    {view.stats && <details className="panel selection-stats">
      <summary>Как подготовлено задание</summary>
      <dl className="report-metrics">
        <div><dt>Кандидаты</dt><dd>{view.stats.candidates}</dd></div>
        <div><dt>Обнаружены тестами</dt><dd>{view.stats.killed}</dd></div>
        <div><dt>Не обнаружены тестами</dt><dd>{view.stats.survived}</dd></div>
        <div><dt>Ошибки сборки</dt><dd>{view.stats.buildErrors}</dd></div>
        <div><dt>Проверено троек</dt><dd>{view.stats.triplesChecked}</dd></div>
      </dl>
    </details>}
    <div className="section-heading"><h2>Мутации</h2><span>Доступно аудитору</span></div>
    {mutations.length ? <div className="mutation-grid">{mutations.map((mutation, index) => {
      const tests = view.killingTests?.[index] ?? []
      return <MutationCard key={mutation.id} mutation={mutation} index={index}
        hint={view.hints?.find(hint => hint.mutationId === mutation.id)}
        fixed={view.report?.verificationError || lastRun?.result.infrastructureError ? undefined : tests.length > 0 && tests.every(test => passedTests.has(test))}
        onSelect={() => setSelected({ path: mutation.file, line: mutation.line })} />
    })}</div> : <p className="panel empty">Список мутаций пока недоступен.</p>}
    <div className="auditor-layout">
      <section className="panel editor-panel" aria-label="Сравнение файлов">
        <div className="panel-head"><h2>Правки в реальном времени</h2>
          {view.running && <span className="fine-print">Идёт прогон тестов…</span>}
        </div>
        {file ? <>
          <div className="file-tabs" aria-label="Файлы для сравнения">{paths.map(path => <button key={path}
            type="button" title={path} aria-pressed={path === file} className={path === file ? 'active' : ''}
            onClick={() => setSelected({ path })}><code>{path}</code></button>)}</div>
          <div className="diff-labels"><span>Выданный код</span><span>Код студента</span></div>
          <div className="diff-surface"><Suspense fallback={<Loading>Загружаем сравнение…</Loading>}>
            <CodeDiff key={`${file}:${focus ?? ''}`} file={file} original={view.mutated[file] ?? ''}
              modified={view.files[file] ?? ''} lines={lines} />
          </Suspense></div>
        </> : <p className="empty">Файлы для сравнения ещё не загружены.</p>}
      </section>
      <AuditorActivity flags={view.flags ?? []} runs={view.runs ?? []} events={view.events ?? []}
        mutations={mutations} disabled={!connected || (!current && !finished) || ending || busy}
        onReview={(id, mutationId, accepted) => void perform(`flags/${encodeURIComponent(id)}`, { mutationId, accepted })} />
    </div>
  </main></>
}
