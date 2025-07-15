package repositories

import (
	"context"
	"document-server/internal/domain/entities"
)

type DocumentRepository interface {
	Create(ctx context.Context, doc *entities.Document) error
	GetByID(ctx context.Context, id string) (*entities.Document, error)
	GetByOwner(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error)
	Update(ctx context.Context, doc *entities.Document) error
	Delete(ctx context.Context, id string) error
	CheckAccess(ctx context.Context, docID, userID string) (bool, error)
}
