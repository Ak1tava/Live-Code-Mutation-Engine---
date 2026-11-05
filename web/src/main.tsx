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

const router = createBrowserRouter([
  { path: '/', element: <Home /> },
  { path: '/a/:tok', element: loading(<Auditor />) },
  { path: '/s/:tok', element: loading(<Student />) },
  { path: '*', element: <><Header /><SessionProblem message="Проверьте адрес страницы." /></> },
])

createRoot(document.getElementById('root')!).render(<StrictMode><RouterProvider router={router} /></StrictMode>)
