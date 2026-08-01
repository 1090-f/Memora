import { request } from '@/api/client'
import type { LoginRequest, LoginData, LogoutResponse } from './types'

export function login(credentials: LoginRequest): Promise<LoginData> {
  return request<LoginData>('/auth/login', {
    method: 'POST',
    body: credentials,
    auth: false,
  })
}

export function logout(): Promise<LogoutResponse> {
  return request<LogoutResponse>('/auth/logout', {
    method: 'POST',
  })
}
