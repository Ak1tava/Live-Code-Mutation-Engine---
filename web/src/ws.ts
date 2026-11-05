import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, request } from './api'
import type { FileAck } from './saveQueue'

type FileEvent = { path: string; content: string }
type Message<V> =
  | { type: 'snapshot'; view: V }
  | ({ type: 'file' } & FileEvent)
  | ({ type: 'fileAck' } & FileAck)
  | { type: 'error'; message: string; path?: string; revision?: number }

interface Observer<V> {
  snapshot?: (view: V, send: (msg: object) => boolean) => void
  ack?: (ack: FileAck) => void
  reject?: (path: string, message: string, revision?: number) => void
  disconnected?: () => void
}

const delays = [500, 1000, 2000, 5000]

export function useSession<V extends { files: Record<string, string> }>(
  tok: string,
  role: 'a' | 's',
  observer?: Observer<V>,
) {
  const [view, setView] = useState<V | null>(null)
  const [connected, setConnected] = useState(false)
  const [fatal, setFatal] = useState('')
  const [error, setError] = useState('')
  const [lastFile, setLastFile] = useState<FileEvent>()
  const wire = useRef<WebSocket>()
  const ready = useRef(false)
  const callbacks = useRef(observer)
  callbacks.current = observer

  const send = useCallback((message: object) => {
    const socket = wire.current
    if (!ready.current || socket?.readyState !== WebSocket.OPEN) return false
    try {
      socket.send(JSON.stringify(message))
      return true
    } catch {
      socket.close()
      return false
    }
  }, [])

  useEffect(() => {
    const token = encodeURIComponent(tok)
    const url = new URL(`/ws/${token}`, location.origin)
    url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const lifetime = new AbortController()
    let retry: number | undefined
    let attempt = 0
    let receivedSnapshot = false
    let diagnosing = false
    let terminal = false
    let handshake: number | undefined

    function connect() {
      if (terminal || lifetime.signal.aborted) return
      const socket = new WebSocket(url)
      wire.current = socket
      ready.current = false
      handshake = window.setTimeout(() => socket.close(), 15_000)
      socket.onmessage = event => {
        if (wire.current !== socket || lifetime.signal.aborted) return
        let message: Message<V>
        try {
          message = JSON.parse(event.data)
        } catch {
          setError('Сервер отправил непонятный ответ.')
          return
        }
        if (message.type === 'snapshot') {
          window.clearTimeout(handshake)
          receivedSnapshot = true
          attempt = 0
          ready.current = true
          setView(message.view)
          setConnected(true)
          callbacks.current?.snapshot?.(message.view, send)
        } else if (message.type === 'file') {
          const { path, content } = message
          setLastFile({ path, content })
          setView(current => current && { ...current, files: { ...current.files, [path]: content } })
        } else if (message.type === 'fileAck') {
          callbacks.current?.ack?.(message)
        } else if (message.type === 'error') {
          if (message.path && callbacks.current?.reject) {
            callbacks.current.reject(message.path, message.message, message.revision)
          } else {
            setError(message.message)
          }
        }
      }
      socket.onclose = () => {
        if (wire.current !== socket || lifetime.signal.aborted) return
        window.clearTimeout(handshake)
        ready.current = false
        setConnected(false)
        callbacks.current?.disconnected?.()
        if (!receivedSnapshot && !diagnosing) {
          diagnosing = true
          void request(`/api/${role}/${token}`, { signal: lifetime.signal }).catch(cause => {
            if (lifetime.signal.aborted) return
            if (cause instanceof ApiError && [403, 404].includes(cause.status)) {
              terminal = true
              window.clearTimeout(retry)
              wire.current?.close()
              setFatal(cause.message)
            }
          }).finally(() => { diagnosing = false })
        }
        retry = window.setTimeout(connect, delays[Math.min(attempt++, delays.length - 1)])
      }
    }

    function onOnline() {
      if (!terminal && wire.current?.readyState === WebSocket.CLOSED) {
        window.clearTimeout(retry)
        connect()
      }
    }

    function onOffline() {
      wire.current?.close()
    }

    setView(null)
    setFatal('')
    setError('')
    setLastFile(undefined)
    connect()
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => {
      lifetime.abort()
      ready.current = false
      window.clearTimeout(retry)
      window.clearTimeout(handshake)
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
      const socket = wire.current
      wire.current = undefined
      if (socket) {
        socket.onclose = null
        // Let an in-progress handshake end quietly before closing.
        if (socket.readyState === WebSocket.CONNECTING) socket.onopen = () => socket.close()
        else socket.close()
      }
    }
  }, [role, send, tok])

  return { view, connected, send, lastFile, fatal, error, clearError: () => setError('') }
}
