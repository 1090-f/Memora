import { request } from '@/api/client'
import type { CurrentUser } from '@/stores/auth'
import type { UpdateProfileRequest, ChangePasswordRequest } from './types'

export function getCurrentUser(): Promise<CurrentUser> {
  return request<CurrentUser>('/users/me')
}

export function updateProfile(data: UpdateProfileRequest): Promise<CurrentUser> {
  return request<CurrentUser>('/users/me', {
    method: 'PATCH',
    body: data,
  })
}

export function changePassword(data: ChangePasswordRequest): Promise<void> {
  return request<void>('/users/me/password', {
    method: 'PATCH',
    body: data,
  })
}
