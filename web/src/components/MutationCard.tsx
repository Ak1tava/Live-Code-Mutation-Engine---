import type { Hint, Mutation } from '../types'

const kinds: Record<string, string> = {
  boundary: 'Граница условия',
  negate: 'Инверсия сравнения',
  logical: 'Логический оператор',
  arith: 'Арифметическая операция',
  incdec: 'Шаг счётчика',
  constant: 'Числовая константа',
}

export function MutationCard({ mutation, index, hint, fixed, onSelect }: {
  mutation: Mutation; index: number; hint?: Hint; fixed?: boolean; onSelect: () => void
}) {
  return <article className="panel mutation-card">
    <div className="mutation-top">
      <span className="mutation-index">{String(index + 1).padStart(2, '0')}</span>
      <span className={`badge ${fixed ? 'test-pass' : 'badge-neutral'}`}>{fixed === undefined ? 'Не проверена' : fixed ? 'Исправлена' : 'Не исправлена'}</span>
    </div>
    <h2>{kinds[mutation.kind] ?? mutation.kind}</h2>
    <button type="button" className="text-link mutation-path" onClick={onSelect}>
      <code>{mutation.file}:{mutation.line}</code><span aria-hidden="true">↗</span>
    </button>
    <div className="operator-swap" aria-label="Исходный и изменённый оператор">
      <code>{mutation.orig}</code><span aria-hidden="true">→</span><code>{mutation.repl}</code>
    </div>
    {hint && <details className="hint"><summary>Подсказка аудитору · {hint.source === 'llm' ? 'ИИ' : 'Шаблон'}</summary>
      <p>{hint.explanation}</p><p className="hint-question">{hint.question}</p>
    </details>}
  </article>
}
