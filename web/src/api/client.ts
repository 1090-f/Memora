import type { ApiEnvelope } from '@/types/common'
import { AppError } from './errors'

const BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

let getAccessToken: () => string | null = () => null
let onUnauthorized: () => void = () => undefined

export function configureAuthTransport(config: {
  getAccessToken: () => string | null
  onUnauthorized: () => void
}): void {
  getAccessToken = config.getAccessToken
  onUnauthorized = config.onUnauthorized
}

export interface ApiRequestInit extends Omit<RequestInit, 'body'> {
  body?: BodyInit | Record<string, unknown> | null
  auth?: boolean
}

export async function appErrorFromResponse(response: Response): Promise<AppError> {
  let code = 'UNKNOWN_ERROR'
  let message = response.statusText
  let details: unknown = null
  let requestId: string | undefined

  try {
    const body = await response.json() as Record<string, unknown>
    if (typeof body.code === 'string') code = body.code
    if (typeof body.message === 'string') message = body.message
    if (body.details !== undefined) details = body.details
    if (typeof body.request_id === 'string') requestId = body.request_id
  } catch {
    // response body is not JSON
  }

  return new AppError(code, message, response.status, requestId, details)
}

export async function request<T>(path: string, init?: ApiRequestInit): Promise<T> {
  const { auth = true, body, headers: customHeaders, ...rest } = init ?? {}

  const headers = new Headers(customHeaders)

  if (auth) {
    const token = getAccessToken()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
  }

  let serializedBody: BodyInit | undefined

  if (body instanceof FormData) {
    serializedBody = body
  } else if (body !== null && body !== undefined && typeof body === 'object') {
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }
    serializedBody = JSON.stringify(body)
  } else if (body !== null && body !== undefined) {
    serializedBody = body as BodyInit
  }

  const url = new URL(path, BASE_URL.startsWith('http') ? BASE_URL : window.location.origin)
  if (BASE_URL.startsWith('/')) {
    url.pathname = BASE_URL + path
  }

  const response = await fetch(url.toString(), {
    ...rest,
    headers,
    body: serializedBody,
  })

  if (response.status === 401) {
    onUnauthorized()
  }

  if (!response.ok) {
    throw await appErrorFromResponse(response)
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T
  }

  const envelope = await response.json() as ApiEnvelope<T>
  return envelope.data
}
