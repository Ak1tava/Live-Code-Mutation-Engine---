import type { RunView, TestResult } from '../types'
import { Loading } from './Shell'

const statuses: Record<TestResult['status'], string> = {
  pass: 'Пройден',
  fail: 'Ошибка',
  timeout: 'Таймаут',
}

export function testCount(run?: RunView) {
  const tests = run?.result.tests ?? []
  return { passed: tests.filter(test => test.status === 'pass').length, total: tests.length }
}

export function TestResults({ run, running }: { run?: RunView; running: boolean }) {
  const counts = testCount(run)
  return <section className="panel result-panel" aria-label="Результаты тестов">
    <div className="panel-head">
      <h2>Результаты тестов</h2>
      {counts.total > 0 && <span className="badge badge-neutral">{counts.passed} из {counts.total} пройдено</span>}
    </div>
    {running && <Loading>Выполняем тесты…</Loading>}
    {!run && !running && <p className="empty">Запустите тесты, чтобы увидеть, какие сценарии проходят.</p>}
    {run?.result.infrastructureError && <div className="build-error"><strong>Проверка недоступна</strong><pre>{run.result.infrastructureError}</pre></div>}
    {run?.result.buildError && <div className="build-error"><strong>Ошибка сборки или запуска</strong><pre>{run.result.buildError}</pre></div>}
    {run && !run.result.buildError && !run.result.infrastructureError && !counts.total && <p className="empty">Прогон завершился без результатов тестов.</p>}
    {(run?.result.tests ?? []).map(test => <details className="test-line" key={`${test.pkg}:${test.name}`}>
      <summary>
        <code>{test.pkg !== '.' && `${test.pkg}/`}{test.name}</code>
        <span className={`badge test-${test.status}`}>{statuses[test.status] ?? 'Неизвестно'}</span>
      </summary>
      <pre>{test.output || 'Тест ничего не вывел.'}</pre>
    </details>)}
  </section>
}
