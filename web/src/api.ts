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

export function post(path: string, body?: unknown): Promise<Response> {
  return request(path, {
    method: 'POST',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
}

export async function createAudit(body: { gitUrl?: string; example?: string }): Promise<{ auditorUrl: string }> {
  const response = await post('/api/audits', body)
  return response.json()
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : 'Возникла ошибка. Повторите попытку.'
}
