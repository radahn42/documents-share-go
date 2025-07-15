package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/pkg/errors"
	"encoding/json"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
)

type DocumentService struct {
	docRepo repositories.DocumentRepository
	cache   CacheService
}

func NewDocumentService(docRepo repositories.DocumentRepository, cache CacheService) *DocumentService {
	return &DocumentService{
		docRepo: docRepo,
		cache:   cache,
	}
}

func (s *DocumentService) Create(
	ctx context.Context,
	userID, name, mime string,
	isFile, isPublic bool,
	filePath *string,
	jsonData *json.RawMessage,
	grant []string,
) (*entities.Document, error) {
	doc := &entities.Document{
		ID:        uuid.NewString(),
		Name:      name,
		OwnerID:   userID,
		MIME:      mime,
		IsFile:    isFile,
		IsPublic:  isPublic,
		FilePath:  filePath,
		JSONData:  jsonData,
		Grant:     &grant,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, errors.NewInternalError("failed to create document")
	}

	s.cache.InvalidatePrefix(ctx, "docs:list:")

	return doc, nil
}

func (s *DocumentService) GetByID(ctx context.Context, docID, userID string) (*entities.Document, error) {
	if doc, err := s.cache.GetDocument(ctx, docID); err == nil {
		if s.checkAccess(doc, userID) {
			return doc, nil
		}
		return nil, errors.NewForbiddenError("access denied")
	}

	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return nil, errors.NewNotFoundError("document not found")
	}

	if !s.checkAccess(doc, userID) {
		return nil, errors.NewForbiddenError("access denied")
	}

	s.cache.SetDocument(ctx, doc)

	return doc, nil
}

func (s *DocumentService) GetList(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error) {
	cacheKey := s.cache.GetListCacheKey(filter)
	if docs, err := s.cache.GetDocumentList(ctx, cacheKey); err == nil {
		return docs, nil
	}

	docs, err := s.docRepo.GetByOwner(ctx, filter)
	if err != nil {
		log.Printf("filter: %+v", filter)
		return nil, errors.NewInternalError("failed to get documents")
	}

	s.cache.SetDocumentList(ctx, cacheKey, docs)

	return docs, nil
}

func (s *DocumentService) Delete(ctx context.Context, docID, userID string) error {
	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return errors.NewNotFoundError("document not found")
	}

	if doc.OwnerID != userID {
		return errors.NewForbiddenError("access denied")
	}

	if err := s.docRepo.Delete(ctx, docID); err != nil {
		return errors.NewInternalError("failed to delete document")
	}

	s.cache.InvalidateDocument(ctx, docID)
	s.cache.InvalidatePrefix(ctx, "docs:list:")

	return nil
}

func (s *DocumentService) checkAccess(doc *entities.Document, userID string) bool {
	if doc.OwnerID == userID || doc.IsPublic {
		return true
	}

	return slices.Contains(*doc.Grant, userID)
}
