package repositories

import (
	"context"
	"document-server/internal/domain/entities"
)

type SessionRepository interface {
	Create(ctx context.Context, session *entities.Session) error
	GetByToken(ctx context.Context, token string) (*entities.Session, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
}
