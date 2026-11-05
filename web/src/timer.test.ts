import { describe, expect, it } from 'vitest'
import { remainingSeconds } from './timer'

describe('время сервера', () => {
  const start = '2026-11-05T09:00:00Z'
  const deadline = '2026-11-05T09:10:00Z'

  it('считает от серверного времени и прошедшего локального интервала', () => {
    expect(remainingSeconds(deadline, start, 1499)).toBe(599)
    expect(remainingSeconds(deadline, '2026-11-05T09:08:00Z', 0)).toBe(120)
  })

  it('держит ноль после дедлайна и при отсутствии времени', () => {
    expect(remainingSeconds(deadline, start, 610_000)).toBe(0)
    expect(remainingSeconds(undefined, start, 100)).toBe(0)
    expect(remainingSeconds(deadline, 'не дата', 100)).toBe(0)
  })
})
