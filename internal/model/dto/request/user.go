package request

type UpdateUserRequest struct {
	Nickname  *string `json:"nickname" binding:"omitempty,max=64"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url"`
	Bio       *string `json:"bio" binding:"omitempty,max=500"`
	Email     *string `json:"email" binding:"omitempty,email,max=255"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,max=128"`
	NewPassword string `json:"new_password" binding:"required,min=12,max=128"`
}
