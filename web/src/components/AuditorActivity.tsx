import { useState } from 'react'
import type { AuditorFlag, Mutation, RunView, SessionEvent } from '../types'
import { testCount } from './TestResults'

const timeFormat = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
function time(at: string) {
  const date = new Date(at)
  return Number.isNaN(date.valueOf()) ? '—' : timeFormat.format(date)
}

function FlagReview({ flag, mutations, disabled, onReview }: {
  flag: AuditorFlag; mutations: Mutation[]; disabled: boolean; onReview: (mutationId: string, accepted: boolean) => void
}) {
  const [choice, setChoice] = useState<string>()
  const mutationId = choice ?? flag.mutationId
  return <li className="flag-review">
    <code>{flag.file}:{flag.line}</code>
    <p>{flag.note || 'Без комментария'}</p>
    <div className="inline-actions" aria-label="Связать с мутацией">
      {mutations.map((mutation, index) => <button type="button" key={mutation.id}
        className={`button small ${mutationId === mutation.id ? 'is-selected' : 'secondary'}`}
        aria-pressed={mutationId === mutation.id}
        disabled={disabled} onClick={() => setChoice(mutation.id)}>#{index + 1}</button>)}
    </div>
    <div className="inline-actions review-actions">
      <button type="button" className="button secondary small"
        disabled={disabled || !mutationId} onClick={() => onReview(mutationId, true)}>Объяснение принято</button>
      <button type="button" className="button secondary small"
        disabled={disabled || !mutationId} onClick={() => onReview(mutationId, false)}>Не принято</button>
    </div>
    <div className="fine-print">{flag.accepted === null || mutationId !== flag.mutationId
      ? 'Ожидает решения' : flag.accepted ? 'Объяснение принято' : 'Объяснение не принято'}</div>
  </li>
}

type Section = 'flags' | 'runs' | 'events'

export function AuditorActivity({ flags, runs, events, mutations, disabled, onReview }: {
  flags: AuditorFlag[]; runs: RunView[]; events: SessionEvent[]; mutations: Mutation[]; disabled: boolean;
  onReview: (id: string, mutationId: string, accepted: boolean) => void
}) {
  const [section, setSection] = useState<Section>('flags')
  const sections: { id: Section; title: string; count: number }[] = [
    { id: 'flags', title: 'Отметки', count: flags.length },
    { id: 'runs', title: 'Прогоны', count: runs.length },
    { id: 'events', title: 'Сигналы', count: events.length },
  ]
  return <section className="panel activity" aria-label="Ход сессии">
    <div className="activity-tabs" aria-label="Разделы хода сессии">
      {sections.map(tab => <button key={tab.id} type="button" aria-pressed={section === tab.id}
        className={section === tab.id ? 'active' : ''} onClick={() => setSection(tab.id)}>
        {tab.title} <span>{tab.count}</span>
      </button>)}
    </div>
    <div className="activity-body">
      {section === 'flags' && <>
        {flags.length === 0 && <p className="empty">Отметки студента появятся здесь во время аудита.</p>}
        <ul className="plain-list">{flags.map(flag => <FlagReview key={flag.id} flag={flag} mutations={mutations}
          disabled={disabled} onReview={(mutationId, accepted) => onReview(flag.id, mutationId, accepted)} />)}</ul>
      </>}
      {section === 'runs' && <>
        {runs.length === 0 && <p className="empty">Студент ещё не запускал тесты.</p>}
        <ul className="plain-list">{runs.slice().reverse().map((run, index) => {
          const { passed, total } = testCount(run)
          return <li key={`${run.at}-${index}`} className="activity-row">
            <div><strong>{run.final ? 'Финальный прогон' : `Прогон ${runs.length - index}`}</strong><time dateTime={run.at}>{time(run.at)}</time></div>
            <span className={`badge ${total > 0 && passed === total ? 'test-pass' : 'badge-neutral'}`}>
              {run.result.buildError ? 'Ошибка сборки или запуска' : `${passed} из ${total}`}
            </span>
          </li>
        })}</ul>
      </>}
      {section === 'events' && <>
        <p className="activity-caption">Сигналы, не доказательства</p>
        {events.length === 0 && <p className="empty">Событий пока нет.</p>}
        <ul className="plain-list">{events.slice().reverse().map((event, index) => <li key={`${event.at}-${index}`} className="activity-row">
          <div><strong>{event.kind === 'paste' ? 'Вставка' : event.kind === 'hidden' ? 'Уход со вкладки' : 'Возврат на вкладку'}</strong>
            <time dateTime={event.at}>{time(event.at)}</time></div>
          {event.kind === 'paste' && <span className="badge badge-neutral">{event.size} симв.</span>}
        </li>)}</ul>
      </>}
    </div>
  </section>
}
