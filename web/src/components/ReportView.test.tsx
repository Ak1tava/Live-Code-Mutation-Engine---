import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ReportView } from './ReportView'
import { TestResults } from './TestResults'
import type { Report } from '../types'

describe('ошибки инфраструктуры', () => {
  it('не выдаёт недоступную проверку за провал студента', () => {
    const report: Report = { restored: false, verificationError: 'Песочница недоступна', durationSec: 60,
      hiddenCount: 0, pasteCount: 0, pasteChars: 0, mutations: [{ id: 'm1', file: 'main.go', line: 1,
        kind: 'operator', orig: '+', repl: '-', detected: true, explained: true, fixed: false, fixedAfterSec: -1 }] }
    const html = renderToStaticMarkup(<ReportView audience="auditor" report={report} />)
    expect(html).toContain('Оценка недоступна')
    expect(html).toContain('Песочница недоступна')
    expect(html).not.toContain('Проект не восстановлен')
    expect(html).not.toContain('>Нет<')
  })

  it('показывает причину инфраструктурной ошибки прогона', () => {
    const html = renderToStaticMarkup(<TestResults running={false} run={{ at: '', final: false,
      result: { id: 'run', tests: null, durationMs: 0, buildError: '', infrastructureError: 'Docker недоступен' } }} />)
    expect(html).toContain('Docker недоступен')
    expect(html).not.toContain('Прогон завершился без результатов')
  })
})
