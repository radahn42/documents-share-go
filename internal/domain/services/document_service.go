package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/pkg/errors"
	"encoding/json"
	"slices"
)

type DocumentService struct {
	docRepo  repositories.DocumentRepository
	userRepo repositories.UserRepository
	cache    CacheService
}

func NewDocumentService(
	docRepo repositories.DocumentRepository,
	userRepo repositories.UserRepository,
	cache CacheService,
) *DocumentService {
	return &DocumentService{
		docRepo:  docRepo,
		userRepo: userRepo,
		cache:    cache,
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
		Name:     name,
		OwnerID:  userID,
		MIME:     mime,
		IsFile:   isFile,
		IsPublic: isPublic,
		FilePath: filePath,
		JSONData: jsonData,
		Grant:    &grant,
	}

	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, errors.NewInternalError("failed to create document")
	}

	s.cache.SetDocument(ctx, doc)

	s.cache.InvalidateUserLists(ctx, userID)
	if doc.Grant != nil {
		for _, grantUserID := range *doc.Grant {
			s.cache.InvalidateUserLists(ctx, grantUserID)
		}
	}

	return doc, nil
}

func (s *DocumentService) GetByID(ctx context.Context, docID, userLogin string) (*entities.Document, error) {
	if doc, err := s.cache.GetDocument(ctx, docID); err == nil {
		hasAccess, err := s.checkAccess(ctx, doc, userLogin)
		if err != nil {
			return nil, errors.NewInternalError("failed to check access")
		}
		if hasAccess {
			return doc, nil
		}
		return nil, errors.NewForbiddenError("access denied")
	}

	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return nil, errors.NewNotFoundError("document not found")
	}

	hasAccess, err := s.checkAccess(ctx, doc, userLogin)
	if err != nil {
		return nil, errors.NewInternalError("failed to check access")
	}
	if !hasAccess {
		return nil, errors.NewForbiddenError("access denied")
	}

	s.cache.SetDocument(ctx, doc)
	return doc, nil
}

func (s *DocumentService) GetList(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error) {
	if filter.RequestingUserLogin == "" {
		return nil, errors.NewForbiddenError("requesting user login is required")
	}

	cacheKey := s.cache.GetListCacheKey(filter)
	if docs, err := s.cache.GetDocumentList(ctx, cacheKey); err == nil {
		return docs, nil
	}

	docs, err := s.docRepo.GetByOwner(ctx, filter)
	if err != nil {
		return nil, errors.NewInternalError("failed to get documents")
	}

	var filteredDocs []*entities.Document
	for _, doc := range docs {
		hasAccess, err := s.checkAccess(ctx, doc, filter.RequestingUserLogin)
		if err != nil {
			continue
		}
		if hasAccess {
			filteredDocs = append(filteredDocs, doc)
		}
	}

	s.cache.SetDocumentList(ctx, cacheKey, filteredDocs)

	return filteredDocs, nil
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
	s.cache.InvalidateUserLists(ctx, userID)

	if doc.Grant != nil {
		for _, grantUserID := range *doc.Grant {
			s.cache.InvalidateUserLists(ctx, grantUserID)
		}
	}

	return nil
}

func (s *DocumentService) checkAccess(ctx context.Context, doc *entities.Document, userLogin string) (bool, error) {
	if doc.IsPublic {
		return true, nil
	}

	user, err := s.userRepo.GetByLogin(ctx, userLogin)
	if err != nil {
		return false, errors.NewInternalError("failed to get user")
	}

	if doc.OwnerID == user.ID {
		return true, nil
	}
	if doc.Grant == nil {
		return false, nil
	}

	return slices.Contains(*doc.Grant, userLogin), nil
}
