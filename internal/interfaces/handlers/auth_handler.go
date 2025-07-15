package handlers

import (
	"document-server/internal/domain/services"
	"document-server/internal/interfaces/dto"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc *services.AuthService
}

func NewAuthHandler(authSvc *services.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, 400, err.Error())
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), req.Token, req.Login, req.Password)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	respondWithSuccess(c, dto.RegisterResponse{Login: user.Login}, nil)
}

func (h *AuthHandler) Authenticate(c *gin.Context) {
	var req dto.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, http.StatusBadRequest, 400, err.Error())
		return
	}

	token, err := h.authSvc.Authenticate(c.Request.Context(), req.Login, req.Password)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	respondWithSuccess(c, dto.AuthResponse{Token: token}, nil)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		respondWithError(c, http.StatusBadRequest, 400, "token is required")
		return
	}

	err := h.authSvc.Logout(c.Request.Context(), token)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	respondWithSuccess(c, dto.LogoutResponse{Success: true}, nil)
}
