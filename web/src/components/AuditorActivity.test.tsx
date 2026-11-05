import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { AuditorActivity } from './AuditorActivity'

it('показывает отсутствие привязки после ответа API с accepted:null', () => {
  const html = renderToStaticMarkup(<AuditorActivity flags={[{ id: 'flag', file: 'main.go', line: 2,
    note: 'заметка', at: '2026-11-05T12:00:00Z', mutationId: '', accepted: null }]}
    mutations={[]} runs={[]} events={[]} disabled={false} onReview={() => {}} />)
  expect(html).toContain('Без привязки к мутации')
  expect(html).not.toContain('Ожидает решения')
})
