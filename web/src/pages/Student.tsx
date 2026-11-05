import { lazy, Suspense, useEffect, useMemo, useReducer, useRef, useState, type FormEvent } from 'react'
import { useBlocker, useParams } from 'react-router-dom'
import type { OnMount } from '@monaco-editor/react'
import { errorText, post } from '../api'
import { FileTree } from '../components/FileTree'
import { ReportView } from '../components/ReportView'
import { Alert, Connection, Header, Loading, SessionProblem } from '../components/Shell'
import { TestResults } from '../components/TestResults'
import { Timer } from '../components/Timer'
import { SaveQueue } from '../saveQueue'
import { useCountdown } from '../timer'
import type { EventKind, StudentView } from '../types'
import { useSession } from '../ws'

const CodeEditor = lazy(() => import('../components/Monaco').then(module => ({ default: module.CodeEditor })))
type Location = { path: string; line?: number }
type Command = 'start' | 'run' | 'finish' | 'flag' | 'file' | 'save'

export default function StudentRoute() {
  const { tok = '' } = useParams()
  return <Student key={tok} tok={tok} />
}

function Student({ tok }: { tok: string }) {
  const [, render] = useReducer(count => count + 1, 0)
  const queue = useMemo(() => new SaveQueue(render), [])
  const { view, connected, send, error: wireError, clearError, fatal } = useSession<StudentView>(tok, 's', {
    snapshot: (snapshot, write) => queue.snapshot(snapshot.files ?? {}, snapshot.state === 'running' && !snapshot.finishing ? write : undefined),
    ack: ack => queue.ack(ack),
    reject: (path, message, revision) => queue.reject(path, message, revision),
    disconnected: () => queue.disconnect(),
  })
  const seconds = useCountdown(view?.deadline, view?.serverNow)
  const active = view?.state === 'running' && !view.finishing && seconds > 0
  const [error, setError] = useState('')
  const [command, setCommand] = useState<Command | null>(null)
  const executing = useRef(false)
  const [requestedRun, setRequestedRun] = useState<number | null>(null)
  const [requestedFinish, setRequestedFinish] = useState(false)
  const [requestedStart, setRequestedStart] = useState(false)
  const [location, setLocation] = useState<Location>()
  const [draftFlag, setDraftFlag] = useState<{ path: string; line: number }>()
  const [note, setNote] = useState('')
  const editor = useRef<Parameters<OnMount>[0]>()
  const [editorReady, setEditorReady] = useState(false)
  const eventsEnabled = useRef(active && !requestedFinish)
  eventsEnabled.current = active && !requestedFinish
  const latestRunCount = view?.runs?.length ?? 0
  const running = !!view?.running || (requestedRun !== null && latestRunCount === requestedRun)
  const ending = !!view?.finishing || requestedFinish || (view?.state === 'running' && seconds === 0)

  useEffect(() => {
    if (requestedRun !== null && latestRunCount !== requestedRun) setRequestedRun(null)
  }, [latestRunCount, requestedRun])

  const blocker = useBlocker(({ currentLocation, nextLocation }) =>
    queue.dirty && view?.state === 'running' && currentLocation.pathname !== nextLocation.pathname,
  )

  useEffect(() => {
    if (blocker.state !== 'blocked') return
    let current = true
    void queue.flush().then(() => {
      if (current) blocker.proceed()
    }, cause => {
      if (!current) return
      setError(errorText(cause))
      blocker.reset()
    })
    return () => { current = false }
  }, [blocker, queue])

  useEffect(() => {
    if (!queue.dirty) return
    function warn(event: BeforeUnloadEvent) {
      if (!queue.dirty) return
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [queue, queue.dirty])

  useEffect(() => () => queue.disconnect(), [queue])
  useEffect(() => {
    function visibility() {
      if (eventsEnabled.current) send({ type: 'event', kind: document.hidden ? 'hidden' : 'visible', size: 0 })
    }
    document.addEventListener('visibilitychange', visibility)
    return () => document.removeEventListener('visibilitychange', visibility)
  }, [send])

  const files = Object.keys(view?.files ?? {}).sort((left, right) => left.localeCompare(right))
  const readOnly = view?.readOnly ?? []
  const file = (location && files.includes(location.path) ? location.path : undefined)
    ?? files.find(path => !readOnly.includes(path)) ?? files[0] ?? ''
  const locked = readOnly.includes(file)
  const endpoint = `/api/s/${encodeURIComponent(tok)}`

  async function perform(kind: Command, action: () => Promise<void>) {
    if (executing.current) return
    executing.current = true
    setCommand(kind)
    setError('')
    clearError()
    try {
      await action()
    } catch (cause) {
      setError(errorText(cause))
    } finally {
      setCommand(null)
      executing.current = false
    }
  }

  function start() {
    if (!connected || requestedStart) return
    void perform('start', async () => {
      void document.documentElement.requestFullscreen?.().catch(() => {})
      await post(`${endpoint}/start`)
      setRequestedStart(true)
    })
  }

  function run() {
    if (!connected || !active || ending || running) return
    void perform('run', async () => {
      await queue.flush()
      await post(`${endpoint}/run`)
      setRequestedRun(latestRunCount)
    })
  }

  function finish() {
    if (!connected || !active || ending) return
    void perform('finish', async () => {
      await queue.flush()
      await post(`${endpoint}/finish`)
      setRequestedFinish(true)
    })
  }

  function select(path: string, line?: number) {
    if (path === file) {
      if (line && editor.current) goTo(editor.current, line)
      return
    }
    void perform('file', async () => {
      if (queue.dirty) await queue.flush()
      setEditorReady(false)
      setLocation({ path, line })
    })
  }

  function goTo(instance: Parameters<OnMount>[0], line: number) {
    instance.setPosition({ lineNumber: line, column: 1 })
    instance.revealLineInCenter(line)
    instance.focus()
  }

  const mountEditor: OnMount = instance => {
    editor.current = instance
    setEditorReady(true)
    if (location?.path === file && location.line) goTo(instance, location.line)
    const paste = instance.onDidPaste(event => {
      const text = instance.getModel()?.getValueInRange(event.range) ?? ''
      if (eventsEnabled.current && text) send({ type: 'event', kind: 'paste' satisfies EventKind, size: [...text].length })
    })
    instance.onDidDispose(() => paste.dispose())
  }

  function submitFlag(event: FormEvent) {
    event.preventDefault()
    if (!draftFlag || !connected || !active || ending || !note.trim()) return
    void perform('flag', async () => {
      await queue.flush()
      await post(`${endpoint}/flags`, { file: draftFlag.path, line: draftFlag.line, note: note.trim() })
      setDraftFlag(undefined)
      setNote('')
    })
  }

  const header = <Header role="Студент"><Connection connected={connected} /></Header>
  if (fatal) return <>{header}<SessionProblem message={fatal} /></>
  if (!view) return <>{header}<main className="narrow page"><Loading>Подключаемся к сессии…</Loading></main></>
  if (view.state === 'failed') return <>{header}<SessionProblem message="Подготовка прервалась. Попросите аудитора создать новую сессию." /></>
  if (view.state === 'finished') return <>{header}<main className="narrow page">
    <ReportView audience="student" report={view.report} />
    {queue.dirty && <Alert tone="warning">В этой вкладке остались несохранённые правки. Финальный прогон проверил версии с сервера.</Alert>}
  </main></>

  if (view.state !== 'running') return <>{header}<main className="narrow page">
    <section className="panel stage-card">
      <div className="stage-label"><span className="eyebrow">Рабочее место студента</span><Timer seconds={600} active={false} /></div>
      <h1>{view.state === 'ready' ? 'Готовы к аудиту?' : 'Готовим ваш проект'}</h1>
      {view.state === 'preparing' ? <Loading>Проверяем задание…</Loading> : <>
        <p className="lead">Найдите три логические ошибки в Go-проекте, объясните их и проверьте исправления тестами.</p>
        <ul className="rules">
          <li>У вас 10 минут. Таймер запускается после нажатия «Начать».</li>
          <li>Работайте самостоятельно, без ИИ-помощников.</li>
          <li>Тестовые файлы доступны только для чтения.</li>
          <li>Отмечайте подозрительные строки и коротко объясняйте, в чём ошибка.</li>
        </ul>
        <button className="button primary" disabled={!connected || !!command || requestedStart} onClick={start}>
          {command === 'start' || requestedStart ? 'Запускаем сессию…' : 'Начать'}
        </button>
      </>}
      <Alert>{error || wireError}</Alert>
    </section>
  </main></>

  return <>{header}<main className="page workspace">
    <div className="page-title">
      <div><span className="eyebrow">Рабочее место студента</span><h1>Аудит кода</h1></div>
      <div className="title-actions">
        <Timer seconds={seconds} />
        <button className="button primary" onClick={run} disabled={!connected || !active || ending || running || !!command}>
          {running && !ending ? 'Тесты выполняются…' : 'Запустить тесты'}
        </button>
        <button className="button secondary" onClick={finish} disabled={!connected || !active || ending || !!command}>
          {ending ? 'Завершение…' : 'Завершить'}
        </button>
      </div>
    </div>
    {!connected && <Alert tone="warning">Связь прервалась. Правки останутся в этой вкладке до переподключения.</Alert>}
    {ending && <Alert tone="warning">Сессия завершается. Готовим отчёт по финальному прогону.</Alert>}
    <Alert>{error || queue.error || wireError}</Alert>
    {queue.error && connected && active && !ending && <button className="button secondary retry-button" disabled={!!command}
      onClick={() => void perform('save', () => queue.retry())}>Повторить сохранение</button>}
    <div className="student-layout">
      <FileTree files={files} readOnly={readOnly} selected={file} disabled={!!command || (queue.dirty && !connected)}
        onSelect={select} />
      <section className="panel editor-panel" aria-label="Редактор проекта">
        <div className="panel-head editor-head">
          <h2><code>{file || 'Нет открытого файла'}</code></h2>
          <div className="editor-tools">
            <span role="status" className="fine-print">
              {locked ? 'Только чтение' : queue.error ? 'Правки не сохранены' : queue.dirty ? connected ? 'Сохраняем…' : 'Черновик' : connected ? 'Сохранено' : 'Офлайн'}
            </span>
            <button className="button secondary small" disabled={!file || !editorReady || !connected || !active || ending || !!command}
              onClick={() => {
                setDraftFlag({ path: file, line: editor.current?.getPosition()?.lineNumber ?? 1 })
                setNote('')
              }}>Здесь ошибка</button>
          </div>
        </div>
        {file ? <div className="editor-surface">
          <Suspense fallback={<Loading>Загружаем редактор…</Loading>}>
            <CodeEditor key={file} file={file} value={queue.content(file)}
              readOnly={locked || !active || ending || !!command || blocker.state === 'blocked'}
              onChange={value => { if (!locked && value !== undefined) queue.edit(file, value) }}
              onMount={mountEditor} />
          </Suspense>
        </div> : <p className="empty">Файлы проекта недоступны.</p>}
        {draftFlag && <form className="flag-form" onSubmit={submitFlag}>
          <label htmlFor="flag-note">Что не так в <code>{draftFlag.path}:{draftFlag.line}</code>?</label>
          <input id="flag-note" placeholder="Короткое объяснение" value={note} onChange={event => setNote(event.target.value)}
            maxLength={500} autoFocus required />
          <div className="inline-actions">
            <button className="button primary small" disabled={!note.trim() || !connected || !active || ending || !!command}>Отметить</button>
            <button className="button ghost small" type="button" onClick={() => setDraftFlag(undefined)}>Отмена</button>
          </div>
        </form>}
      </section>
      <aside className="panel student-flags">
        <div className="panel-head"><h2>Мои отметки</h2><span className="counter">{view.flags?.length ?? 0}</span></div>
        {!view.flags?.length && <p className="empty">Выделите строку и нажмите «Здесь ошибка».</p>}
        <ul className="plain-list">{view.flags?.map(flag => <li key={flag.id}>
          <button className="text-link" type="button" disabled={!!command || (queue.dirty && !connected)}
            onClick={() => select(flag.file, flag.line)}><code>{flag.file}:{flag.line}</code></button>
          <p>{flag.note}</p>
        </li>)}</ul>
      </aside>
    </div>
    <TestResults run={view.runs?.at(-1)} running={running} />
  </main></>
}
