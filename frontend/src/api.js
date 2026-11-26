const API_URL = (import.meta.env.VITE_API_URL || 'http://localhost:8081').replace(/\/+$/, '')
export const WS_URL = import.meta.env.VITE_WS_URL || API_URL.replace(/^http/, 'ws') + '/ws'

export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.status = status
  }
}

export async function request(path, { method = 'GET', body, token, signal } = {}) {
  let response
  try {
    response = await fetch(API_URL + path, {
      method,
      headers: {
        Accept: 'application/json',
        ...(body !== undefined && { 'Content-Type': 'application/json' }),
        ...(token && { Authorization: `Bearer ${token}` }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new ApiError('Não foi possível acessar a API. Tente novamente.')
  }

  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(data?.erro || 'Não foi possível concluir a solicitação.', response.status)
  }

  return data
}
