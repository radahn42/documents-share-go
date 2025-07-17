package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/internal/utils"
	"document-server/pkg/errors"
	"document-server/pkg/logger"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo      repositories.UserRepository
	sessionRepo   repositories.SessionRepository
	adminToken    string
	tokenDuration time.Duration
	logger        *zap.Logger
}

func NewAuthService(
	userRepo repositories.UserRepository,
	sessionRepo repositories.SessionRepository,
	adminToken string,
	tokenDuration time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		adminToken:    adminToken,
		tokenDuration: tokenDuration,
		logger:        logger.Logger,
	}
}

func (s *AuthService) Register(ctx context.Context, adminToken, login, password string) (*entities.User, error) {
	s.logger.Debug("User registration attempt",
		zap.String("login", login),
	)

	if adminToken != s.adminToken {
		s.logger.Warn("Invalid admin token provided during registration",
			zap.String("login", login),
		)
		return nil, errors.NewUnauthorizedError("invalid admin token")
	}

	if err := utils.ValidateLogin(login); err != nil {
		s.logger.Warn("Invalid login format during registration",
			zap.String("login", login),
			zap.Error(err),
		)
		return nil, errors.NewBadRequestError(err.Error())
	}

	if err := utils.ValidatePassword(password); err != nil {
		s.logger.Warn("Invalid password format during registration",
			zap.String("login", login),
			zap.Error(err),
		)
		return nil, errors.NewBadRequestError(err.Error())
	}

	if _, err := s.userRepo.GetByLogin(ctx, login); err == nil {
		s.logger.Warn("Attempt to register existing user",
			zap.String("login", login),
		)
		return nil, errors.NewBadRequestError("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password during registration",
			zap.String("login", login),
			zap.Error(err),
		)
		return nil, errors.NewInternalError("failed to hash password")
	}

	user := &entities.User{
		Login:    login,
		Password: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user in repository",
			zap.String("login", login),
			zap.Error(err),
		)
		return nil, errors.NewInternalError("failed to create user")
	}

	s.logger.Info("User registered successfully",
		zap.String("user_id", user.ID),
		zap.String("login", login),
	)

	return user, nil
}

func (s *AuthService) Authenticate(ctx context.Context, login, password string) (string, error) {
	s.logger.Debug("Authentication attempt",
		zap.String("login", login),
	)

	user, err := s.userRepo.GetByLogin(ctx, login)
	if err != nil {
		s.logger.Warn("Authentication failed - user not found",
			zap.String("login", login),
			zap.Error(err),
		)
		return "", errors.NewUnauthorizedError("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		s.logger.Warn("Authentication failed - invalid password",
			zap.String("login", login),
			zap.String("user_id", user.ID),
		)
		return "", errors.NewUnauthorizedError("invalid credentials")
	}

	token := utils.GenerateToken()
	session := &entities.Session{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(s.tokenDuration),
	}

	s.logger.Debug("Creating session for authenticated user",
		zap.String("user_id", user.ID),
		zap.String("login", login),
		zap.Time("expires_at", session.ExpiresAt),
	)

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		s.logger.Error("Failed to create session",
			zap.String("user_id", user.ID),
			zap.String("login", login),
			zap.Error(err),
		)
		return "", errors.NewInternalError("failed to create session")
	}

	s.logger.Info("User authenticated successfully",
		zap.String("user_id", user.ID),
		zap.String("login", login),
		zap.Duration("token_duration", s.tokenDuration),
	)

	return token, nil
}

func (s *AuthService) GetUserByLogin(ctx context.Context, login string) (*entities.User, error) {
	s.logger.Debug("Getting user by login",
		zap.String("login", login),
	)

	user, err := s.userRepo.GetByLogin(ctx, login)
	if err != nil {
		s.logger.Error("Failed to get user by login",
			zap.String("login", login),
			zap.Error(err),
		)
		return nil, err
	}

	s.logger.Debug("User retrieved successfully by login",
		zap.String("user_id", user.ID),
		zap.String("login", login),
	)

	return user, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (*entities.User, error) {
	s.logger.Debug("Validating token")

	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		s.logger.Warn("Token validation failed - session not found",
			zap.Error(err),
		)
		return nil, errors.NewUnauthorizedError("invalid token")
	}

	s.logger.Debug("Session found for token",
		zap.String("user_id", session.UserID),
		zap.Time("expires_at", session.ExpiresAt),
	)

	if session.ExpiresAt.Before(time.Now()) {
		s.logger.Warn("Token validation failed - token expired",
			zap.String("user_id", session.UserID),
			zap.Time("expires_at", session.ExpiresAt),
		)

		go func() {
			if err := s.sessionRepo.Delete(context.Background(), token); err != nil {
				s.logger.Error("Failed to delete expired session",
					zap.String("user_id", session.UserID),
					zap.Error(err),
				)
			} else {
				s.logger.Debug("Expired session deleted",
					zap.String("user_id", session.UserID),
				)
			}
		}()

		return nil, errors.NewUnauthorizedError("token expired")
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		s.logger.Error("Token validation failed - user not found",
			zap.String("user_id", session.UserID),
			zap.Error(err),
		)
		return nil, errors.NewUnauthorizedError("user not found")
	}

	s.logger.Debug("Token validated successfully",
		zap.String("user_id", user.ID),
		zap.String("login", user.Login),
		zap.Time("expires_at", session.ExpiresAt),
	)

	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	s.logger.Debug("Logout attempt")

	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		s.logger.Warn("Logout attempt with invalid token",
			zap.Error(err),
		)
		return s.sessionRepo.Delete(ctx, token)
	}

	s.logger.Debug("Session found for logout",
		zap.String("user_id", session.UserID),
	)

	if err := s.sessionRepo.Delete(ctx, token); err != nil {
		s.logger.Error("Failed to delete session during logout",
			zap.String("user_id", session.UserID),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("User logged out successfully",
		zap.String("user_id", session.UserID),
	)

	return nil
}
