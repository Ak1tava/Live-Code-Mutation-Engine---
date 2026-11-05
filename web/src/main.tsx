import { StrictMode, lazy, Suspense, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { Header, Loading, SessionProblem } from './components/Shell'
import Home from './pages/Home'
import './styles.css'

const Auditor = lazy(() => import('./pages/Auditor'))
const Student = lazy(() => import('./pages/Student'))

function loading(children: ReactNode) {
  return <Suspense fallback={<><Header /><main className="narrow page"><Loading>Открываем аудит…</Loading></main></>}>
    {children}
  </Suspense>
}

function RouteError() {
  return <><Header /><main className="narrow page"><section className="panel stage-card">
    <h1>Не удалось загрузить страницу</h1>
    <p>Проверьте соединение и обновите страницу. Состояние сессии хранится на сервере.</p>
    <button className="button primary" onClick={() => window.location.reload()}>Обновить страницу</button>
  </section></main></>
}

const router = createBrowserRouter([
  { path: '/', element: <Home />, errorElement: <RouteError /> },
  { path: '/a/:tok', element: loading(<Auditor />), errorElement: <RouteError /> },
  { path: '/s/:tok', element: loading(<Student />), errorElement: <RouteError /> },
  { path: '*', element: <><Header /><SessionProblem message="Проверьте адрес страницы." /></> },
])

createRoot(document.getElementById('root')!).render(<StrictMode><RouterProvider router={router} /></StrictMode>)
