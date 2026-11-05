export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

export async function request(path: string, init?: RequestInit): Promise<Response> {
  let response: Response
  try {
    response = await fetch(path, init)
  } catch {
    throw new Error('Не удалось связаться с сервером. Проверьте соединение и повторите попытку.')
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new ApiError(body?.error || 'Сервер не смог выполнить запрос. Повторите попытку.', response.status)
  }
  return response
}

export async function post(path: string, body?: unknown): Promise<Response> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 15_000)
  try {
    return await request(path, {
      method: 'POST',
      signal: controller.signal,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (error) {
    if (controller.signal.aborted) throw new Error('Истекло время ожидания ответа. Проверьте состояние сессии и повторите попытку.')
    throw error
  } finally {
    clearTimeout(timeout)
  }
}

export async function createAudit(body: { gitUrl?: string; example?: string }): Promise<{ auditorUrl: string }> {
  const response = await post('/api/audits', body)
  return response.json()
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : 'Возникла ошибка. Повторите попытку.'
}
