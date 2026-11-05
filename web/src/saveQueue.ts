export interface FileWrite {
  type: 'file'
  path: string
  content: string
  revision: number
}

export type FileAck = Omit<FileWrite, 'type' | 'revision'> & { revision?: number }

type Send = (write: FileWrite) => boolean
type Timer = ReturnType<typeof setTimeout>
type Draft = {
  write: FileWrite
  ready: boolean
  debounce?: Timer
  flight?: { write: FileWrite; timeout: Timer }
  error?: string
}

const offline = 'Потеряно соединение с сервером. Черновик сохранён в этой вкладке.'
const encoder = new TextEncoder()

export class SaveQueue {
  private files: Record<string, string> = {}
  private readonly confirmed = new Map<string, string>()
  private readonly drafts = new Map<string, Draft>()
  private readonly waiters = new Set<{ resolve: () => void; reject: (reason: Error) => void }>()
  private send?: Send
  private revision = 0

  constructor(private readonly changed: () => void, private readonly reconnect?: () => void) {}

  get dirty() {
    return this.drafts.size > 0
  }

  get error() {
    return [...this.drafts.values()].find(draft => draft.error)?.error
  }

  content(path: string) {
    return this.drafts.get(path)?.write.content ?? this.confirmed.get(path) ?? this.files[path] ?? ''
  }

  snapshot(files: Record<string, string>, send?: Send) {
    const reconnecting = !this.send && !!send
    this.files = files
    if (reconnecting || !send) this.confirmed.clear()
    for (const [path, content] of this.confirmed) {
      if (files[path] === content) this.confirmed.delete(path)
    }
    if (!send) {
      this.disconnect('Редактирование недоступно в текущем состоянии сессии.')
      return
    }

    this.send = send
    if (reconnecting) {
      for (const draft of this.drafts.values()) {
        clearTimeout(draft.debounce)
        draft.ready = true
        draft.error = undefined
      }
      this.pump()
    }
  }

  disconnect(reason = offline) {
    this.send = undefined
    this.files = { ...this.files, ...Object.fromEntries(this.confirmed) }
    this.confirmed.clear()
    for (const draft of this.drafts.values()) {
      clearTimeout(draft.flight?.timeout)
      draft.flight = undefined
    }
    this.settle(reason)
  }

  edit(path: string, content: string, immediate = false) {
    if (content === this.content(path)) return
    const draft = this.drafts.get(path)
    clearTimeout(draft?.debounce)
    const next: Draft = {
      write: { type: 'file', path, content, revision: ++this.revision },
      ready: immediate,
      flight: draft?.flight,
    }
    if (!immediate) next.debounce = setTimeout(() => {
      next.ready = true
      this.pump()
    }, 1000)
    this.drafts.set(path, next)
    this.pump()
  }

  flush() {
    if (!this.send) return Promise.reject(new Error(offline))
    const result = new Promise<void>((resolve, reject) => this.waiters.add({ resolve, reject }))
    for (const draft of this.drafts.values()) {
      clearTimeout(draft.debounce)
      draft.ready = true
    }
    this.pump()
    return result
  }

  retry() {
    for (const draft of this.drafts.values()) draft.error = undefined
    return this.flush()
  }

  ack({ path, content, revision }: FileAck) {
    const draft = this.drafts.get(path)
    const sent = draft?.flight?.write
    if (!draft || !sent || sent.content !== content || (revision !== undefined && revision !== sent.revision)) return
    clearTimeout(draft.flight?.timeout)
    draft.flight = undefined
    this.confirmed.set(path, content)
    if (draft.write.revision === sent.revision) this.drafts.delete(path)
    this.pump()
  }

  reject(path: string, message: string, revision?: number) {
    const draft = this.drafts.get(path)
    const sent = draft?.flight?.write
    if (!draft || !sent || (revision !== undefined && revision !== sent.revision)) return
    clearTimeout(draft.flight?.timeout)
    draft.flight = undefined
    if (draft.write.revision === sent.revision) draft.error = message
    this.pump()
  }

  private pump() {
    for (const draft of this.drafts.values()) {
      if (!this.send || !draft.ready || draft.flight || draft.error) continue
      if (encoder.encode(draft.write.content).byteLength > 256 * 1024) {
        draft.error = 'Файл превышает 256 КиБ. Сократите его, чтобы сохранить правку.'
        continue
      }
      const write = draft.write
      if (!this.send(write)) {
        this.disconnect()
        return
      }
      const timeout = setTimeout(() => {
        this.disconnect('Сервер не подтвердил сохранение. Восстанавливаем соединение; черновик сохранён в этой вкладке.')
        this.reconnect?.()
      }, 10_000)
      draft.flight = { write, timeout }
    }
    this.settle(this.error)
  }

  private settle(error?: string) {
    if (error || !this.dirty) {
      for (const waiter of this.waiters) {
        if (error) waiter.reject(new Error(error))
        else waiter.resolve()
      }
      this.waiters.clear()
    }
    this.changed()
  }
}
