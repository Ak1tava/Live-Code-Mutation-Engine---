export type PendingRun = { count: number; snapshot: number }

export function isRunPending(request: PendingRun | null, count: number, running: boolean, snapshot: number) {
  return request !== null && count === request.count && (snapshot <= request.snapshot || running)
}

export function watchConnection(expired: () => void) {
  let timeout: ReturnType<typeof setTimeout>
  const received = () => {
    clearTimeout(timeout)
    timeout = setTimeout(expired, 40_000)
  }
  received()
  return { received, dispose: () => clearTimeout(timeout) }
}
