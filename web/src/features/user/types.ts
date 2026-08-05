export interface UpdateProfileRequest {
  nickname?: string
  avatar_url?: string
  bio?: string
  email?: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}
