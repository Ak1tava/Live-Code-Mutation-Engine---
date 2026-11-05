import { duration } from '../timer'

export function Timer({ seconds, active = true }: { seconds: number; active?: boolean }) {
  return <div role="timer" aria-label={`Осталось ${seconds} с`}
    className={`timer ${active && seconds <= 60 ? 'timer-urgent' : ''}`}>
    <span aria-hidden="true">◷</span><strong>{duration(seconds)}</strong>
  </div>
}
