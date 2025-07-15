package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/internal/utils"
	"document-server/pkg/errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo      repositories.UserRepository
	sessionRepo   repositories.SessionRepository
	adminToken    string
	jwtSecret     string
	tokenDuration time.Duration
}

func NewAuthService(
	userRepo repositories.UserRepository,
	sessionRepo repositories.SessionRepository,
	adminToken, jwtSecret string,
	tokenDuration time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		adminToken:    adminToken,
		jwtSecret:     jwtSecret,
		tokenDuration: tokenDuration,
	}
}

func (s *AuthService) Register(ctx context.Context, adminToken, login, password string) (*entities.User, error) {
	if adminToken != s.adminToken {
		return nil, errors.NewUnauthorizedError("invalid admin token")
	}

	if err := utils.ValidateLogin(login); err != nil {
		return nil, errors.NewBadRequestError(err.Error())
	}
	if err := utils.ValidatePassword(password); err != nil {
		return nil, errors.NewBadRequestError(err.Error())
	}

	if _, err := s.userRepo.GetByLogin(ctx, login); err == nil {
		return nil, errors.NewBadRequestError("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.NewInternalError("failed to hash password")
	}

	user := &entities.User{
		ID:        uuid.NewString(),
		Login:     login,
		Password:  string(hashedPassword),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, errors.NewInternalError("failed to create user")
	}

	return user, nil
}

func (s *AuthService) Authenticate(ctx context.Context, login, password string) (string, error) {
	user, err := s.userRepo.GetByLogin(ctx, login)
	if err != nil {
		return "", errors.NewUnauthorizedError("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.NewUnauthorizedError("invalid credentials")
	}

	token := utils.GenerateToken()
	session := &entities.Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(s.tokenDuration),
		UpdatedAt: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return "", errors.NewInternalError("failed to create session")
	}

	return token, nil
}

func (s *AuthService) GetUserByLogin(ctx context.Context, login string) (*entities.User, error) {
	return s.userRepo.GetByLogin(ctx, login)
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*entities.User, error) {
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, errors.NewUnauthorizedError("invalid token")
	}

	if session.ExpiresAt.Before(time.Now()) {
		s.sessionRepo.Delete(ctx, token)
		return nil, errors.NewUnauthorizedError("token expired")
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, errors.NewUnauthorizedError("user not found")
	}

	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionRepo.Delete(ctx, token)
}
