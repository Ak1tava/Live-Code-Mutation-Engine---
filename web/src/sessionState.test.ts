import { afterEach, describe, expect, it, vi } from 'vitest'
import { isRunPending, watchConnection } from './sessionState'

afterEach(() => vi.useRealTimers())

describe('ожидание прогона', () => {
  it('не снимает блокировку по старому снимку сразу после POST', () => {
    expect(isRunPending({ count: 2, snapshot: 8 }, 2, false, 8)).toBe(true)
  })
  it('снимает блокировку после reconnect без нового результата', () => {
    expect(isRunPending({ count: 2, snapshot: 8 }, 2, false, 9)).toBe(false)
  })
  it('сохраняет ожидание живого прогона и снимает его после результата', () => {
    expect(isRunPending({ count: 2, snapshot: 8 }, 2, true, 9)).toBe(true)
    expect(isRunPending({ count: 2, snapshot: 8 }, 3, false, 10)).toBe(false)
  })
})

describe('обнаружение зависшего WebSocket', () => {
  it('перезапускает соединение после 40 секунд тишины, считая от последнего heartbeat', () => {
    vi.useFakeTimers()
    let closed = false
    const connection = watchConnection(() => { closed = true })
    vi.advanceTimersByTime(30_000)
    connection.received()
    vi.advanceTimersByTime(39_999)
    expect(closed).toBe(false)
    vi.advanceTimersByTime(1)
    expect(closed).toBe(true)
    connection.dispose()
  })
  it('не перезапускает уже закрытое соединение', () => {
    vi.useFakeTimers()
    let closed = false
    const connection = watchConnection(() => { closed = true })
    connection.dispose()
    vi.advanceTimersByTime(45_000)
    expect(closed).toBe(false)
  })
})
