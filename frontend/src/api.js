const API_URL = (import.meta.env.VITE_API_URL || 'http://localhost:8081').replace(/\/+$/, '')

// Se VITE_API_URL for um caminho relativo (ex: "/api", quando o front é
// servido pelo mesmo nginx que distribui as APIs), o websocket usa o mesmo
// endereço da página, trocando http por ws.
function defaultWsUrl() {
  if (API_URL.startsWith('/')) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}${API_URL}/ws`
  }
  return API_URL.replace(/^http/, 'ws') + '/ws'
}

export const WS_URL = import.meta.env.VITE_WS_URL || defaultWsUrl()

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
