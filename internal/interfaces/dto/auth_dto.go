package dto

type RegisterRequest struct {
	Token    string `json:"token" binding:"required"`
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterResponse struct {
	Login string `json:"login"`
}

type AuthRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type LogoutResponse struct {
	Success bool `json:"success"`
}
