package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	appErrors "document-server/pkg/errors"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type documentRepository struct {
	pool *pgxpool.Pool
}

func NewDocumentRepository(pool *pgxpool.Pool) repositories.DocumentRepository {
	return &documentRepository{pool: pool}
}

const (
	baseSelectQuery  = `SELECT id, name, owner_id, mime, is_file, is_public, file_path, json_data, "grant", created_at, updated_at FROM documents`
	insertQuery      = `INSERT INTO documents (name, owner_id, mime, is_file, is_public, file_path, json_data, "grant") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	updateQuery      = `UPDATE documents SET name = $1, mime = $2, is_file = $3, is_public = $4, file_path = $5, json_data = $6, "grant" = $7, updated_at = $8 WHERE id = $9`
	deleteQuery      = `DELETE FROM documents WHERE id = $1`
	checkAccessQuery = `SELECT EXISTS(SELECT 1 FROM documents WHERE id = $1 AND (owner_id = $2 OR is_public = true OR $2 = ANY("grant")))`
)

func (r *documentRepository) Create(ctx context.Context, doc *entities.Document) error {
	_, err := r.pool.Exec(ctx, insertQuery,
		doc.Name, doc.OwnerID, doc.MIME, doc.IsFile,
		doc.IsPublic, doc.FilePath, doc.JSONData, doc.Grant,
	)
	return r.wrapError(err)
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
		return nil, appErrors.NewInternalError("failed to query documents")
	}
	defer rows.Close()

	return r.scanDocuments(rows)
}

func (r *documentRepository) Update(ctx context.Context, doc *entities.Document) error {
	_, err := r.pool.Exec(ctx, updateQuery,
		doc.Name, doc.MIME, doc.IsFile, doc.IsPublic,
		doc.FilePath, doc.JSONData, doc.Grant, doc.UpdatedAt, doc.ID,
	)
	return r.wrapError(err)
}

func (r *documentRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, deleteQuery, id)
	return r.wrapError(err)
}

func (r *documentRepository) CheckAccess(ctx context.Context, docID string, userID string) (bool, error) {
	var hasAccess bool
	err := r.pool.QueryRow(ctx, checkAccessQuery, docID, userID).Scan(&hasAccess)
	return hasAccess, err
}

func (r *documentRepository) buildFilterQuery(filter *entities.DocumentFilter) (string, []any) {
	var conditions []string
	var args []any
	argIndex := 1

	if filter.OwnerID != "" {
		if filter.RequestingUserID != "" && filter.RequestingUserID != filter.OwnerID {
			conditions = append(conditions, fmt.Sprintf(`owner_id = $%d AND (is_public = true OR $%d = ANY("grant"))`, argIndex, argIndex+1))
			args = append(args, filter.OwnerID, filter.RequestingUserID)
			argIndex += 2
		} else {
			conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIndex))
			args = append(args, filter.OwnerID)
			argIndex++
		}
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
			return nil, appErrors.NewInternalError("failed to scan document")
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
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
