import type { CurrentUser } from '@/stores/auth'

export interface LoginRequest {
  account: string
  password: string
}

export interface LoginData {
  access_token: string
  token_type: string
  expires_in: number
  user: CurrentUser
}

export interface LogoutResponse {
  message: string
}
