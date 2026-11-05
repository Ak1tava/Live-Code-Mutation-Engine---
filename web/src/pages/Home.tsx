import { useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { createAudit, errorText } from '../api'
import { Alert, Header } from '../components/Shell'

export default function Home() {
  const [url, setUrl] = useState('')
  const [busy, setBusy] = useState<'git' | 'demo' | null>(null)
  const pending = useRef(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  async function start(source: 'git' | 'demo') {
    if (pending.current) return
    pending.current = true
    setBusy(source)
    setError('')
    try {
      const result = await createAudit(source === 'demo' ? { example: 'textkit' } : { gitUrl: url.trim() })
      navigate(result.auditorUrl)
    } catch (cause) {
      setError(errorText(cause))
    } finally {
      pending.current = false
      setBusy(null)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    void start('git')
  }

  return <>
    <Header><span className="header-meta">Tomorrow School <span>×</span> BRIDGERS</span></Header>
    <main className="page home">
      <section className="home-hero">
        <div>
          <div className="eyebrow"><span className="signal-dot" /> P2P-аудит · Go</div>
          <h1>Разберитесь,<br /> как работает <em>код.</em></h1>
          <p className="lead">Три логические ошибки в знакомом проекте. Десять минут, чтобы найти, объяснить и исправить их.</p>
          <div className="hero-stats" aria-label="Формат аудита">
            <div><strong>03</strong><span>мутации</span></div>
            <div><strong>10<span className="stat-unit"> мин</span></strong><span>на решение</span></div>
            <div><strong>Go</strong><span>язык проекта</span></div>
          </div>
        </div>
        <section className="panel home-form" aria-labelledby="new-audit">
          <span className="eyebrow">Рабочее место аудитора</span>
          <h2 id="new-audit">Новый аудит</h2>
          <p>Выберите Go-проект с тестами. Система проверит исходный код и подберёт три независимые ошибки.</p>
          <form onSubmit={submit}>
            <label htmlFor="git-url">Ссылка на git-репозиторий (https)</label>
            <input id="git-url" type="url" pattern="https://.+" placeholder="https://github.com/…" required
              value={url} onChange={e => setUrl(e.target.value)} disabled={!!busy} autoComplete="url" />
            <button type="submit" className="button primary" disabled={!!busy}>
              {busy === 'git' ? 'Создаём аудит…' : 'Подготовить аудит'} <span aria-hidden="true">→</span>
            </button>
          </form>
          <div className="divider">или</div>
          <button className="button secondary" type="button" onClick={() => void start('demo')} disabled={!!busy}>
            {busy === 'demo' ? 'Открываем демо…' : 'Попробовать на демо-проекте'}
          </button>
          <Alert>{error}</Alert>
        </section>
      </section>
      <section className="steps" aria-label="Как проходит аудит">
        <div><span>01</span><div><h2>Подготовка</h2><p>Проверяем проект и выбираем ошибки, которые обнаруживают тесты.</p></div></div>
        <div><span>02</span><div><h2>Работа с кодом</h2><p>Студент исследует проект, делает отметки и запускает тесты.</p></div></div>
        <div><span>03</span><div><h2>Разбор</h2><p>Аудитор наблюдает за изменениями и фиксирует результаты.</p></div></div>
      </section>
    </main>
  </>
}
