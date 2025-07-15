package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
)

type UserService struct {
	userRepo repositories.UserRepository
}

func NewUserService(userRepo repositories.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetByLogin(ctx context.Context, login string) (*entities.User, error) {
	return s.userRepo.GetByLogin(ctx, login)
}
