package services

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"document-server/pkg/errors"
	"document-server/pkg/logger"
	"encoding/json"
	"log"
	"runtime"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"
)

type DocumentService struct {
	docRepo  repositories.DocumentRepository
	userRepo repositories.UserRepository
	cache    CacheService
	logger   *zap.Logger
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
		logger:   logger.Logger,
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
	s.logger.Debug("Creating document",
		zap.String("user_id", userID),
		zap.String("name", name),
		zap.String("mime", mime),
		zap.Bool("is_file", isFile),
		zap.Bool("is_public", isPublic),
		zap.Strings("grant", grant),
	)

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
		s.logger.Error("Failed to create document in repository",
			zap.String("user_id", userID),
			zap.String("name", name),
			zap.Error(err),
		)
		return nil, errors.NewInternalError("failed to create document")
	}

	s.logger.Info("Document created successfully",
		zap.String("doc_id", doc.ID),
		zap.String("user_id", userID),
	)

	return doc, nil
}

func (s *DocumentService) GetByID(ctx context.Context, docID, userLogin string) (*entities.Document, error) {
	s.logger.Debug("Getting document by ID",
		zap.String("doc_id", docID),
		zap.String("user_login", userLogin),
	)

	if doc, err := s.cache.GetDocument(ctx, docID); err == nil {
		s.logger.Debug("Document found in cache",
			zap.String("doc_id", docID),
		)

		if hasAccess, err := s.checkAccess(ctx, doc, userLogin); err != nil {
			s.logger.Error("Failed to check access for cached document",
				zap.String("doc_id", docID),
				zap.String("user_login", userLogin),
				zap.Error(err),
			)
			return nil, errors.NewInternalError("failed to check access")
		} else if !hasAccess {
			s.logger.Warn("Access denied for cached document",
				zap.String("doc_id", docID),
				zap.String("user_login", userLogin),
			)
			return nil, errors.NewForbiddenError("access denied")
		}

		s.logger.Debug("Document access granted from cache",
			zap.String("doc_id", docID),
			zap.String("user_login", userLogin),
		)
		return doc, nil
	}

	s.logger.Debug("Document not found in cache, querying database",
		zap.String("doc_id", docID),
	)

	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		s.logger.Error("Document not found in database",
			zap.String("doc_id", docID),
			zap.Error(err),
		)
		return nil, errors.NewNotFoundError("document not found")
	}

	hasAccess, err := s.checkAccess(ctx, doc, userLogin)
	if err != nil {
		s.logger.Error("Failed to check access for document from database",
			zap.String("doc_id", docID),
			zap.String("user_login", userLogin),
			zap.Error(err),
		)
		return nil, errors.NewInternalError("failed to check access")
	}
	if !hasAccess {
		s.logger.Warn("Access denied for document from database",
			zap.String("doc_id", docID),
			zap.String("user_login", userLogin),
		)
		return nil, errors.NewForbiddenError("access denied")
	}

	s.logger.Info("Document retrieved successfully",
		zap.String("doc_id", docID),
		zap.String("user_login", userLogin),
	)

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.SetDocument(cacheCtx, doc); err != nil {
			s.logger.Error("Failed to cache document",
				zap.String("doc_id", docID),
				zap.Error(err),
			)
		} else {
			s.logger.Debug("Document cached successfully",
				zap.String("doc_id", docID),
			)
		}
	})

	return doc, nil
}

func (s *DocumentService) GetList(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error) {
	s.logger.Debug("Getting document list",
		zap.String("requesting_user", filter.RequestingUserLogin),
		zap.Any("filter", filter),
	)

	if filter.RequestingUserLogin == "" {
		s.logger.Warn("Document list requested without user login")
		return nil, errors.NewForbiddenError("requesting user login is required")
	}

	cacheKey := s.cache.GetListCacheKey(filter)

	if docs, err := s.cache.GetDocumentList(ctx, cacheKey); err == nil {
		s.logger.Debug("Document list found in cache",
			zap.String("cache_key", cacheKey),
			zap.Int("count", len(docs)),
		)
		return docs, nil
	}

	s.logger.Debug("Document list not found in cache, querying database",
		zap.String("cache_key", cacheKey),
	)

	docs, err := s.docRepo.GetByOwner(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get documents from database",
			zap.String("requesting_user", filter.RequestingUserLogin),
			zap.Error(err),
		)
		return nil, errors.NewInternalError("failed to get documents")
	}

	filteredDocs, err := s.filterDocumentsWithAccess(ctx, docs, filter.RequestingUserLogin)
	if err != nil {
		return nil, errors.NewInternalError("failed to filter documents by access")
	}

	s.logger.Info("Document list retrieved successfully",
		zap.String("requesting_user", filter.RequestingUserLogin),
		zap.Int("total_count", len(docs)),
		zap.Int("filtered_count", len(filteredDocs)),
	)

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.SetDocumentList(cacheCtx, cacheKey, filteredDocs); err != nil {
			s.logger.Error("Failed to cache document list",
				zap.String("cache_key", cacheKey),
				zap.Error(err),
			)
			log.Printf("Failed to cache document list: %v", err)
		} else {
			s.logger.Debug("Document list cached successfully",
				zap.String("cache_key", cacheKey),
				zap.Int("count", len(filteredDocs)),
			)
		}
	})

	return filteredDocs, nil
}

func (s *DocumentService) Delete(ctx context.Context, docID, userID string) error {
	s.logger.Debug("Deleting document",
		zap.String("doc_id", docID),
		zap.String("user_id", userID),
	)

	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		s.logger.Error("Document not found for deletion",
			zap.String("doc_id", docID),
			zap.Error(err),
		)
		return errors.NewNotFoundError("document not found")
	}

	if doc.OwnerID != userID {
		s.logger.Warn("User attempted to delete document they don't own",
			zap.String("doc_id", docID),
			zap.String("user_id", userID),
			zap.String("owner_id", doc.OwnerID),
		)
		return errors.NewForbiddenError("access denied")
	}

	if err := s.docRepo.Delete(ctx, docID); err != nil {
		s.logger.Error("Failed to delete document from database",
			zap.String("doc_id", docID),
			zap.Error(err),
		)
		return errors.NewInternalError("failed to delete document")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user for cache invalidation",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}

	s.logger.Info("Document deleted successfully",
		zap.String("doc_id", docID),
		zap.String("user_id", userID),
	)

	go s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.cache.InvalidateDocument(cacheCtx, docID); err != nil {
			s.logger.Error("Failed to invalidate document cache",
				zap.String("doc_id", docID),
				zap.Error(err),
			)
		} else {
			s.logger.Debug("Document cache invalidated",
				zap.String("doc_id", docID),
			)
		}

		if user != nil {
			if err := s.cache.InvalidateUserLists(cacheCtx, user.Login); err != nil {
				s.logger.Error("Failed to invalidate user lists",
					zap.String("user_login", user.Login),
					zap.Error(err),
				)
			} else {
				s.logger.Debug("User lists cache invalidated",
					zap.String("user_login", user.Login),
				)
			}
		}

		if doc.Grant != nil {
			for _, grantUserLogin := range *doc.Grant {
				if err := s.cache.InvalidateUserLists(cacheCtx, grantUserLogin); err != nil {
					s.logger.Error("Failed to invalidate user lists for granted user",
						zap.String("granted_user_login", grantUserLogin),
						zap.Error(err),
					)
				} else {
					s.logger.Debug("Granted user lists cache invalidated",
						zap.String("granted_user_login", grantUserLogin),
					)
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
		s.logger.Error("Failed to get user for access check",
			zap.String("user_login", userLogin),
			zap.Error(err),
		)
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
	numWorkers := runtime.NumCPU() / 2

	s.logger.Debug("Filtering documents with access",
		zap.String("user_login", userLogin),
		zap.Int("total_docs", len(docs)),
		zap.Int("workers", numWorkers),
	)

	jobs := make(chan *entities.Document, len(docs))
	results := make(chan *entities.Document, len(docs))

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for doc := range jobs {
				if hasAccess, err := s.checkAccess(ctx, doc, userLogin); err != nil {
					s.logger.Error("Error checking access for document",
						zap.String("doc_id", doc.ID),
						zap.Int("worker_id", workerID),
					)
				} else if hasAccess {
					select {
					case results <- doc:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
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

	s.logger.Debug("Document filtering completed",
		zap.String("user_login", userLogin),
		zap.Int("filtered_count", len(filteredDocs)),
	)

	return filteredDocs, nil
}

func (s *DocumentService) performCacheOperations(doc *entities.Document, ownerLogin string) {
	s.safeCacheOperation(func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.cache.SetDocument(cacheCtx, doc); err != nil {
			s.logger.Error("Failed to cache document",
				zap.String("doc_id", doc.ID),
				zap.Error(err),
			)
		} else {
			s.logger.Debug("Document cached successfully",
				zap.String("doc_id", doc.ID),
			)
		}

		if err := s.cache.InvalidateUserLists(cacheCtx, ownerLogin); err != nil {
			s.logger.Error("Failed to invalidate user lists",
				zap.String("user_login", ownerLogin),
				zap.Error(err),
			)
		} else {
			s.logger.Debug("User lists cache invalidated",
				zap.String("user_login", ownerLogin),
			)
		}

		if doc.Grant != nil {
			for _, grantUserLogin := range *doc.Grant {
				if err := s.cache.InvalidateUserLists(cacheCtx, grantUserLogin); err != nil {
					s.logger.Error("Failed to invalidate user lists for granted user",
						zap.String("granted_user_login", grantUserLogin),
						zap.Error(err),
					)
				} else {
					s.logger.Debug("Granted user lists cache invalidated",
						zap.String("granted_user_login", grantUserLogin),
					)
				}
			}
		}
	})
}

func (s *DocumentService) safeCacheOperation(operation func()) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic in cache operation",
				zap.Any("panic", r),
			)
		}
	}()
	operation()
}
