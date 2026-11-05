import { afterEach, describe, expect, it, vi } from 'vitest'
import { post } from './api'

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

describe('таймаут команд', () => {
  it('прерывает POST через 15 секунд и возвращает понятную ошибку', async () => {
    vi.useFakeTimers()
    let aborted = false
    vi.stubGlobal('fetch', (_path: string, init: RequestInit) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener('abort', () => {
        aborted = true
        reject(new DOMException('Aborted', 'AbortError'))
      })
    }))
    let failure = ''
    void post('/api/s/token/run').catch(error => { failure = error.message })
    await vi.advanceTimersByTimeAsync(15_000)
    expect(aborted).toBe(true)
    expect(failure).toContain('время ожидания')
  })
})
