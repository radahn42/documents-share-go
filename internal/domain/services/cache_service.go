package services

import (
	"context"
	"document-server/internal/domain/entities"
	"encoding/json"
	"fmt"
	"time"
)

type CacheService interface {
	GetDocument(ctx context.Context, docID string) (*entities.Document, error)
	SetDocument(ctx context.Context, doc *entities.Document) error
	GetDocumentList(ctx context.Context, key string) ([]*entities.Document, error)
	SetDocumentList(ctx context.Context, key string, docs []*entities.Document) error
	InvalidateDocument(ctx context.Context, docID string) error
	InvalidatePrefix(ctx context.Context, prefix string) error
	GetListCacheKey(filter *entities.DocumentFilter) string
}

type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, duration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Keys(ctx context.Context, pattern string) ([]string, error)
}

type redisCacheService struct {
	client        RedisClient
	cacheDuration time.Duration
}

func NewRedisCacheService(client RedisClient, cacheDuration time.Duration) *redisCacheService {
	return &redisCacheService{
		client:        client,
		cacheDuration: cacheDuration,
	}
}

func (s *redisCacheService) GetDocument(ctx context.Context, docID string) (*entities.Document, error) {
	key := fmt.Sprintf("doc:%s", docID)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var doc entities.Document
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (s *redisCacheService) SetDocument(ctx context.Context, doc *entities.Document) error {
	key := fmt.Sprintf("doc:%s", doc.ID)
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, data, s.cacheDuration)
}

func (s *redisCacheService) GetDocumentList(ctx context.Context, key string) ([]*entities.Document, error) {
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var docs []*entities.Document
	if err := json.Unmarshal([]byte(data), &docs); err != nil {
		return nil, err
	}

	return docs, nil
}

func (s *redisCacheService) SetDocumentList(ctx context.Context, key string, docs []*entities.Document) error {
	data, err := json.Marshal(docs)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, data, s.cacheDuration)
}

func (s *redisCacheService) InvalidateDocument(ctx context.Context, docID string) error {
	key := fmt.Sprintf("doc:%s", docID)
	return s.client.Del(ctx, key)
}

func (s *redisCacheService) InvalidatePrefix(ctx context.Context, prefix string) error {
	pattern := fmt.Sprintf("%s*", prefix)
	keys, err := s.client.Keys(ctx, pattern)
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return s.client.Del(ctx, keys...)
	}

	return nil
}

func (s *redisCacheService) GetListCacheKey(filter *entities.DocumentFilter) string {
	return fmt.Sprintf(
		"docs:list:owner=%s:user=%s:key=%s:val=%s:limit=%d",
		filter.OwnerID,
		filter.RequestingUserID,
		filter.Key,
		filter.Value,
		filter.Limit,
	)
}
