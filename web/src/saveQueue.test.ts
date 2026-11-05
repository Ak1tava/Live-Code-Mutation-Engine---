import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SaveQueue, type FileWrite } from './saveQueue'

const path = 'main.go'
const snapshot = { [path]: 'исходное' }

describe('подтверждение автосохранения', () => {
  const send = vi.fn<(write: FileWrite) => boolean>()
  let queue: SaveQueue

  beforeEach(() => {
    vi.useFakeTimers()
    send.mockReset().mockReturnValue(true)
    queue = new SaveQueue(() => {})
    queue.snapshot(snapshot, send)
  })

  afterEach(() => {
    queue.disconnect()
    vi.useRealTimers()
  })

  it('сохраняет через секунду после последней правки и защищает черновик от снимка', () => {
    queue.edit(path, 'первая')
    vi.advanceTimersByTime(800)
    queue.edit(path, 'вторая')
    queue.snapshot(snapshot, send)
    expect(queue.content(path)).toBe('вторая')
    vi.advanceTimersByTime(999)
    expect(send).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(send).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ path, content: 'вторая' }))
  })

  it('ждёт подтверждения каждой последней версии перед действием', async () => {
    queue.edit(path, 'первая')
    vi.advanceTimersByTime(1000)
    const first = send.mock.calls[0][0]
    queue.edit(path, 'вторая')
    let resolved = false
    const flushed = queue.flush().then(() => { resolved = true })
    queue.ack(first)
    const second = send.mock.calls[1][0]
    expect(second.content).toBe('вторая')
    queue.ack(first)
    await Promise.resolve()
    expect(resolved).toBe(false)
    queue.ack(second)
    await flushed
    expect(queue.dirty).toBe(false)
    queue.snapshot(snapshot, send)
    expect(queue.content(path)).toBe('вторая')
    queue.snapshot({ [path]: 'вторая' }, send)
    queue.snapshot({ [path]: 'третья' }, send)
    expect(queue.content(path)).toBe('третья')
  })

  it('возвращает ошибку при обрыве и повторяет отправку после снимка', async () => {
    queue.edit(path, 'локальная правка')
    const flushed = queue.flush()
    const rejected = expect(flushed).rejects.toThrow('соединение')
    queue.disconnect()
    await rejected
    expect(queue.content(path)).toBe('локальная правка')
    expect(queue.dirty).toBe(true)
    await expect(queue.flush()).rejects.toThrow('соединение')
    queue.snapshot(snapshot, send)
    expect(send).toHaveBeenCalledTimes(2)
    const afterReconnect = queue.flush()
    queue.ack(send.mock.calls[1][0])
    await afterReconnect
    expect(queue.content(path)).toBe('локальная правка')
  })

  it('сохраняет отклонённую правку и позволяет исправить её', async () => {
    queue.edit(path, 'первая')
    const flushed = queue.flush()
    const rejected = expect(flushed).rejects.toThrow('Сохранение отклонено')
    const first = send.mock.calls[0][0]
    queue.reject(path, 'Сохранение отклонено', first.revision)
    await rejected
    expect(queue.content(path)).toBe('первая')
    queue.edit(path, 'вторая')
    const retried = queue.flush()
    queue.ack(send.mock.calls[1][0])
    await retried
  })

  it('считает размер файла в байтах UTF-8', async () => {
    queue.edit(path, 'я'.repeat(131073))
    await expect(queue.flush()).rejects.toThrow('256 КиБ')
    expect(send).not.toHaveBeenCalled()
  })

  it('не ждёт подтверждения бесконечно и оставляет черновик', async () => {
    queue.edit(path, 'первая')
    const flushed = queue.flush()
    const rejected = expect(flushed).rejects.toThrow('подтвердил')
    vi.advanceTimersByTime(10_000)
    await rejected
    expect(queue.dirty).toBe(true)
  })
  it('после таймаута ACK разрывает зависшее соединение и повторяет черновик по новому снимку', async () => {
    let reconnected = false
    queue = new SaveQueue(() => {}, () => { reconnected = true })
    queue.snapshot(snapshot, send)
    queue.edit(path, 'черновик')
    const flushed = queue.flush()
    const rejected = expect(flushed).rejects.toThrow('подтвердил')
    vi.advanceTimersByTime(10_000)
    await rejected
    expect(reconnected).toBe(true)
    expect(queue.content(path)).toBe('черновик')
    queue.snapshot(snapshot, send)
    queue.ack(send.mock.calls.at(-1)![0])
    expect(queue.dirty).toBe(false)
  })

  it('отправляет каждую новую правку немедленно у дедлайна, включая правку после ACK', () => {
    queue.edit(path, 'последняя секунда', true)
    expect(send.mock.calls.at(-1)?.[0].content).toBe('последняя секунда')
    queue.ack(send.mock.calls.at(-1)![0])
    queue.edit(path, 'последние полсекунды', true)
    expect(send.mock.calls.at(-1)?.[0].content).toBe('последние полсекунды')
    queue.ack(send.mock.calls.at(-1)![0])
    expect(queue.dirty).toBe(false)
  })

  it('отправляет срочную правку после ACK предыдущей версии без нового debounce', () => {
    queue.edit(path, 'первая')
    vi.advanceTimersByTime(1000)
    const first = send.mock.calls[0][0]
    queue.edit(path, 'срочная', true)
    expect(send.mock.calls).toHaveLength(1)
    queue.ack(first)
    expect(send.mock.calls.at(-1)?.[0].content).toBe('срочная')
  })
  it('сохраняет подтверждённую основу для офлайн правки после обрыва до снимка', async () => {
    queue.edit(path, 'исправление')
    const flushed = queue.flush()
    queue.ack(send.mock.calls[0][0])
    await flushed
    queue.disconnect()
    expect(queue.content(path)).toBe('исправление')
    queue.edit(path, queue.content(path) + ' + офлайн')
    queue.snapshot({ [path]: 'исправление' }, send)
    expect(send.mock.calls.at(-1)?.[0].content).toBe('исправление + офлайн')
  })
  it('принимает авторитетный снимок после повторного подключения без черновика', async () => {
    queue.edit(path, 'исправление')
    const flushed = queue.flush()
    queue.ack(send.mock.calls[0][0])
    await flushed
    queue.disconnect()
    queue.snapshot({ [path]: 'правка из второй вкладки' }, send)
    expect(queue.content(path)).toBe('правка из второй вкладки')
  })

})
