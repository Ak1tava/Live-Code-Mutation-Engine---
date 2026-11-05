import { useEffect, useMemo, useState } from 'react'

export function remainingSeconds(deadline?: string, serverNow?: string, elapsedMs = 0) {
  const difference = Date.parse(deadline ?? '') - Date.parse(serverNow ?? '') - elapsedMs
  return Number.isFinite(difference) ? Math.max(0, Math.ceil(difference / 1000)) : 0
}

export function useCountdown(deadline?: string, serverNow?: string) {
  const anchor = useMemo(() => ({ deadline, serverNow, receivedAt: Date.now() }), [deadline, serverNow])
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!deadline || !serverNow) return
    const timer = window.setInterval(() => setNow(Date.now()), 250)
    return () => window.clearInterval(timer)
  }, [deadline, serverNow])
  return remainingSeconds(anchor.deadline, anchor.serverNow, Math.max(0, now - anchor.receivedAt))
}

export function duration(seconds: number) {
  return `${Math.floor(seconds / 60).toString().padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`
}
