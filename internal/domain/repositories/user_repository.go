package repositories

import (
	"context"
	"document-server/internal/domain/entities"
)

type UserRepository interface {
	Create(ctx context.Context, user *entities.User) error
	GetByLogin(ctx context.Context, login string) (*entities.User, error)
	GetByID(ctx context.Context, id string) (*entities.User, error)
}
