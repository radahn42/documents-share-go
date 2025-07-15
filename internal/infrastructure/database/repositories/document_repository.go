package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type documentRepository struct {
	db *sqlx.DB
}

func NewDocumentRepository(db *sqlx.DB) repositories.DocumentRepository {
	return &documentRepository{db: db}
}

func (r *documentRepository) Create(ctx context.Context, doc *entities.Document) error {
	query := `INSERT INTO documents (id, name, owner_id, mime, is_file, is_public, file_path, json_data, "grant", created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.db.ExecContext(ctx, query,
		doc.ID, doc.Name, doc.OwnerID, doc.MIME, doc.IsFile, doc.IsPublic,
		doc.FilePath, doc.JSONData, doc.Grant, doc.CreatedAt, doc.UpdatedAt,
	)
	return err
}

func (r *documentRepository) GetByID(ctx context.Context, id string) (*entities.Document, error) {
	query := `SELECT id, name, owner_id, mime, is_file, is_public, file_path, json_data, "grant", created_at, updated_at
			FROM documents WHERE id = $1`

	var doc entities.Document
	err := r.db.GetContext(ctx, &doc, query, id)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

func (r *documentRepository) GetByOwner(ctx context.Context, filter *entities.DocumentFilter) ([]*entities.Document, error) {
	query := `SELECT id, name, owner_id, mime, is_file, is_public, file_path, json_data, "grant", created_at, updated_at
		FROM documents WHERE 1=1`
	args := []any{}
	argIndex := 1

	if filter.OwnerID != "" {
		query += fmt.Sprintf(" AND owner_id = $%d", argIndex)
		args = append(args, filter.OwnerID)
		argIndex++
	}

	if filter.Key != "" && filter.Value != "" {
		switch filter.Key {
		case "name":
			query += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
			args = append(args, "%"+filter.Value+"%")
			argIndex++
		case "mime":
			query += fmt.Sprintf(" AND mime = $%d", argIndex)
			args = append(args, filter.Value)
			argIndex++
		case "public":
			query += fmt.Sprintf(" AND is_public = $%d", argIndex)
			args = append(args, filter.Value == "true")
			argIndex++
		default:
			query += fmt.Sprintf(" AND json_data->>$%d = $%d", argIndex, argIndex+1)
			args = append(args, filter.Key, filter.Value)
			argIndex += 2
		}
	}

	query += " ORDER BY name ASC, created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
	}

	var docs []*entities.Document
	err := r.db.SelectContext(ctx, &docs, query, args...)
	if err != nil {
		return nil, err
	}

	return docs, nil
}

func (r *documentRepository) Update(ctx context.Context, doc *entities.Document) error {
	query := `UPDATE documents SET name = $1, mime = $2, is_file = $3, is_public = $4,
		file_path = $5, json_data = $6, "grant" = $7, updated_at = $8
		WHERE id = $9`

	_, err := r.db.ExecContext(ctx, query,
		doc.Name, doc.MIME, doc.IsFile, doc.IsPublic,
		doc.FilePath, doc.JSONData, doc.Grant, doc.UpdatedAt, doc.ID,
	)
	return err
}

func (r *documentRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM documents WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *documentRepository) CheckAccess(ctx context.Context, docID string, userID string) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM documents
		WHERE id = $1 AND (owner_id = $2 OR is_public = true OR $2 = ANY("grant"))
	)`

	var hasAccess bool
	err := r.db.GetContext(ctx, &hasAccess, query, docID, userID)
	return hasAccess, err
}
