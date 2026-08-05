package request

// UpdateUserRequest 表示更新用户资料的请求，所有字段均为可选。
type UpdateUserRequest struct {
	Nickname  *string `json:"nickname" binding:"omitempty,max=64"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url"`
	Bio       *string `json:"bio" binding:"omitempty,max=500"`
	Email     *string `json:"email" binding:"omitempty,email,max=255"`
}

// ChangePasswordRequest 表示修改密码的请求，包含旧密码和新密码。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,max=128"`
	NewPassword string `json:"new_password" binding:"required,min=12,max=128"`
}
