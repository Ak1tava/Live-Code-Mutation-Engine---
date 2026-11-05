import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export function Header({ role, children }: { role?: string; children?: ReactNode }) {
  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link className="brand" to="/" aria-label="Live Code, главная">
          <span className="brand-icon" aria-hidden="true">⌘</span>
          <span>Live Code<span className="brand-period">.</span></span>
        </Link>
        {role && <span className="header-role">{role}</span>}
        <div className="header-content">{children}</div>
      </div>
    </header>
  )
}

export function Loading({ children = 'Загрузка…' }: { children?: ReactNode }) {
  return <div className="loading" role="status"><span className="spinner" aria-hidden="true" />{children}</div>
}

export function Alert({ children, tone = 'error' }: { children?: ReactNode; tone?: 'error' | 'warning' }) {
  return children ? <div role={tone === 'error' ? 'alert' : 'status'} className={`alert alert-${tone}`}>{children}</div> : null
}

export function Connection({ connected }: { connected: boolean }) {
  return <span className={`connection ${connected ? 'online' : 'offline'}`}><span />{connected ? 'На связи' : 'Переподключение'}</span>
}

export function SessionProblem({ message }: { message: string }) {
  return <main className="narrow page">
    <div className="panel stage-card">
      <span className="eyebrow">Сессия</span>
      <h1>Не удалось открыть аудит</h1>
      <p>{message}</p>
      <Link className="button primary" to="/">На главную</Link>
    </div>
  </main>
}
