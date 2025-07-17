package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	appErrors "document-server/pkg/errors"
	"document-server/pkg/logger"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type documentRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewDocumentRepository(pool *pgxpool.Pool) repositories.DocumentRepository {
	return &documentRepository{
		pool:   pool,
		logger: logger.Logger,
	}
}

const (
	baseSelectQuery = `SELECT id, name, owner_id, mime, is_file, is_public, file_path, json_data, "grant", created_at, updated_at FROM documents`
	insertQuery     = `INSERT INTO documents (name, owner_id, mime, is_file, is_public, file_path, json_data, "grant") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`
	deleteQuery = `DELETE FROM documents WHERE id = $1`
)

func (r *documentRepository) Create(ctx context.Context, doc *entities.Document) error {
	err := r.pool.QueryRow(ctx, insertQuery,
		doc.Name, doc.OwnerID, doc.MIME, doc.IsFile,
		doc.IsPublic, doc.FilePath, doc.JSONData, doc.Grant,
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		r.logger.Error("Database operation failed",
			zap.String("operation", "create_document"),
			zap.String("owner_id", doc.OwnerID),
			zap.Error(err),
		)
		return r.wrapError(err)
	}

	return nil
}

func (r *documentRepository) GetByID(ctx context.Context, id string) (*entities.Document, error) {
	var doc entities.Document
	err := r.pool.QueryRow(ctx, baseSelectQuery+" WHERE id = $1", id).Scan(
		&doc.ID, &doc.Name, &doc.OwnerID, &doc.MIME, &doc.IsFile, &doc.IsPublic,
		&doc.FilePath, &doc.JSONData, &doc.Grant, &doc.CreatedAt, &doc.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.NewNotFoundError("document not found")
		}
		r.logger.Error("Database operation failed",
			zap.String("operation", "get_document_by_id"),
			zap.String("doc_id", id),
			zap.Error(err),
		)
		return nil, appErrors.NewInternalError("database query failed")
	}

	return &doc, nil
}

func (r *documentRepository) GetByOwner(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error) {
	if filter == nil {
		return nil, appErrors.NewBadRequestError("filter cannot be nil")
	}

	query, args := r.buildFilterQuery(filter)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Database operation failed",
			zap.String("operation", "get_documents_by_owner"),
			zap.String("owner_id", filter.OwnerID),
			zap.Error(err),
		)
		return nil, appErrors.NewInternalError("failed to query documents")
	}
	defer rows.Close()

	docs, err := r.scanDocuments(rows)
	if err != nil {
		return nil, err
	}

	return docs, nil
}

func (r *documentRepository) Delete(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx, deleteQuery, id)
	if err != nil {
		r.logger.Error("Database operation failed",
			zap.String("operation", "delete_document"),
			zap.String("doc_id", id),
			zap.Error(err),
		)
		return r.wrapError(err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return appErrors.NewNotFoundError("document not found")
	}

	return nil
}

func (r *documentRepository) buildFilterQuery(filter *entities.DocumentFilter) (string, []any) {
	var conditions []string
	var args []any
	argIndex := 1

	if filter.OwnerID != "" {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIndex))
		args = append(args, filter.OwnerID)
		argIndex++
	}

	if filter.Key != "" && filter.Value != "" {
		condition, newArgs, newIndex := r.buildKeyValueFilter(filter.Key, filter.Value, argIndex)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, newArgs...)
			argIndex = newIndex
		}
	}

	query := baseSelectQuery
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY name ASC, created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
	}

	return query, args
}

func (r *documentRepository) buildKeyValueFilter(key, value string, startIndex int) (string, []any, int) {
	switch key {
	case "name":
		return fmt.Sprintf("name ILIKE $%d", startIndex), []any{"%" + value + "%"}, startIndex + 1
	case "mime":
		return fmt.Sprintf("mime = $%d", startIndex), []any{value}, startIndex + 1
	case "public":
		if publicBool, err := strconv.ParseBool(value); err == nil {
			return fmt.Sprintf("is_public = $%d", startIndex), []any{publicBool}, startIndex + 1
		}
		return "", nil, startIndex
	default:
		return fmt.Sprintf("json_data->>$%d = $%d", startIndex, startIndex+1), []any{key, value}, startIndex + 2
	}
}

func (r *documentRepository) scanDocuments(rows pgx.Rows) ([]*entities.Document, error) {
	var docs []*entities.Document

	for rows.Next() {
		doc := &entities.Document{}
		err := rows.Scan(
			&doc.ID, &doc.Name, &doc.OwnerID, &doc.MIME, &doc.IsFile, &doc.IsPublic,
			&doc.FilePath, &doc.JSONData, &doc.Grant, &doc.CreatedAt, &doc.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Database operation failed",
				zap.String("operation", "scan_document_row"),
				zap.Error(err),
			)
			return nil, appErrors.NewInternalError("failed to scan document")
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Database operation failed",
			zap.String("operation", "iterate_rows"),
			zap.Error(err),
		)
		return nil, appErrors.NewInternalError("rows iteration error")
	}

	return docs, nil
}

func (r *documentRepository) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return appErrors.NewNotFoundError("document not found")
	}

	return appErrors.NewInternalError("database operation failed")
}
