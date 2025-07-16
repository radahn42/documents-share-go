package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/pkg/errors"
	"encoding/json"
	"log"
	"slices"
	"sync"
	"time"
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

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get user")
	}

	go s.performCacheOperations(doc, user.Login)

	return doc, nil
}

func (s *DocumentService) GetByID(ctx context.Context, docID, userLogin string) (*entities.Document, error) {
	if doc, err := s.cache.GetDocument(ctx, docID); err == nil {
		if hasAccess, err := s.checkAccess(ctx, doc, userLogin); err != nil {
			return nil, errors.NewInternalError("failed to check access")
		} else if !hasAccess {
			return nil, errors.NewForbiddenError("access denied")
		}
		return doc, nil
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

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.SetDocument(cacheCtx, doc); err != nil {
			log.Printf("Failed to cache document %s: %v", doc.ID, err)
		}
	})

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

	filteredDocs, err := s.filterDocumentsWithAccess(ctx, docs, filter.RequestingUserLogin)
	if err != nil {
		return nil, err
	}

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.SetDocumentList(cacheCtx, cacheKey, filteredDocs); err != nil {
			log.Printf("Failed to cache document list: %v", err)
		}
	})

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

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		log.Printf("Failed to get user for cache invalidation: %v", err)
	}

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.cache.InvalidateDocument(cacheCtx, docID); err != nil {
			log.Printf("Failed to invalidate document cache %s: %v", docID, err)
		}

		if user != nil {
			if err := s.cache.InvalidateUserLists(cacheCtx, user.Login); err != nil {
				log.Printf("Failed to invalidate user lists for %s: %v", user.Login, err)
			}
		}

		if doc.Grant != nil {
			for _, grantUserLogin := range *doc.Grant {
				if err := s.cache.InvalidateUserLists(cacheCtx, grantUserLogin); err != nil {
					log.Printf("Failed to invalidate user lists for granted user %s: %v", grantUserLogin, err)
				}
			}
		}
	})

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

	if doc.Grant != nil && slices.Contains(*doc.Grant, userLogin) {
		return true, nil
	}

	return false, nil
}

func (s *DocumentService) filterDocumentsWithAccess(ctx context.Context, docs []*entities.Document, userLogin string) ([]*entities.Document, error) {
	const numWorkers = 5
	jobs := make(chan *entities.Document, len(docs))
	results := make(chan *entities.Document, len(docs))

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range jobs {
				if hasAccess, err := s.checkAccess(ctx, doc, userLogin); err != nil {
					log.Printf("Error checking access for document %s: %v", doc.ID, err)
				} else if hasAccess {
					select {
					case results <- doc:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, doc := range docs {
			select {
			case jobs <- doc:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var filteredDocs []*entities.Document
	for doc := range results {
		filteredDocs = append(filteredDocs, doc)
	}

	return filteredDocs, nil
}

func (s *DocumentService) performCacheOperations(doc *entities.Document, ownerLogin string) {
	s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.cache.SetDocument(cacheCtx, doc); err != nil {
			log.Printf("Failed to cache document %s: %v", doc.ID, err)
		}

		if err := s.cache.InvalidateUserLists(cacheCtx, ownerLogin); err != nil {
			log.Printf("Failed to invalidate user lists for %s: %v", ownerLogin, err)
		}

		if doc.Grant != nil {
			for _, grantUserLogin := range *doc.Grant {
				if err := s.cache.InvalidateUserLists(cacheCtx, grantUserLogin); err != nil {
					log.Printf("Failed to invalidate user lists for granted user %s: %v", grantUserLogin, err)
				}
			}
		}
	})
}

func (s *DocumentService) safeCacheOperation(operation func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in cache operation: %v", r)
		}
	}()
	operation()
}
