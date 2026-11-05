import { duration } from '../timer'
import type { Report, ReportSummary } from '../types'

type Props = { audience: 'student'; report?: ReportSummary } | { audience: 'auditor'; report?: Report }

function Result({ value }: { value: boolean }) {
  return <span className={`badge ${value ? 'test-pass' : 'badge-neutral'}`}>{value ? 'Да' : 'Нет'}</span>
}

export function ReportView(props: Props) {
  const report = props.report
  const full = props.audience === 'auditor' ? props.report : undefined
  return <section className="report">
    <div className="page-title">
      <div><span className="eyebrow">Итоги аудита</span><h1>Сессия завершена</h1></div>
      <button className="button secondary no-print" onClick={() => window.print()}>Печать</button>
    </div>
    {report ? <>
      <div className={`panel report-banner ${!report.verificationError && report.restored ? 'restored' : ''}`}>
        <span className="report-icon" aria-hidden="true">{!report.verificationError && report.restored ? '✓' : '!'}</span>
        <div>
          <h2>{report.verificationError ? 'Оценка недоступна' : report.restored ? 'Проект восстановлен' : 'Проект не восстановлен'}</h2>
          <p>{report.verificationError || (report.restored ? 'Все тесты финального прогона прошли.' : 'Финальный прогон не подтвердил исправность проекта.')}</p>
        </div>
        <div className="report-time"><span>Длительность</span><strong>{duration(Math.max(0, report.durationSec))}</strong></div>
      </div>
      {full && <>
        <section className="panel">
          <div className="panel-head"><h2>Разбор мутаций</h2></div>
          <div className="table-scroll"><table>
            <thead><tr><th>Мутация</th><th>Обнаружена</th><th>Объяснена</th><th>Исправлена</th><th>Время</th></tr></thead>
            <tbody>{full.mutations.map(mutation => <tr key={mutation.id}>
              <th scope="row"><code>{mutation.file}:{mutation.line}</code><br /><code>{mutation.orig} → {mutation.repl}</code></th>
              <td><Result value={mutation.detected} /></td>
              <td><Result value={mutation.explained} /></td>
              <td>{report.verificationError ? <span className="badge badge-neutral">Не проверена</span> : <Result value={mutation.fixed} />}</td>
              <td>{mutation.fixedAfterSec < 0 ? '—' : duration(mutation.fixedAfterSec)}</td>
            </tr>)}</tbody>
          </table></div>
        </section>
        <section className="panel">
          <div className="panel-head"><h2>Сигналы</h2><span className="fine-print">События, не доказательства</span></div>
          <dl className="report-metrics">
            <div><dt>Уходы со вкладки</dt><dd>{full.hiddenCount}</dd></div>
            <div><dt>Вставки</dt><dd>{full.pasteCount}</dd></div>
            <div><dt>Вставлено символов</dt><dd>{full.pasteChars}</dd></div>
          </dl>
        </section>
      </>}
    </> : <p className="panel empty">Отчёт по этой сессии недоступен.</p>}
  </section>
}
