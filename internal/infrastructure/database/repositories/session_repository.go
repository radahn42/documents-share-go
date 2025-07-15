package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"

	"github.com/jmoiron/sqlx"
)

type sessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(db *sqlx.DB) repositories.SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, session *entities.Session) error {
	query := `INSERT INTO sessions (id, user_id, token, expires_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, session.ID, session.UserID, session.Token, session.ExpiresAt, session.UpdatedAt)
	return err
}

func (r *sessionRepository) GetByToken(ctx context.Context, token string) (*entities.Session, error) {
	query := `SELECT id, user_id, token, expires_at, updated_at FROM sessions WHERE token = $1`

	var session entities.Session
	err := r.db.GetContext(ctx, &session, query, token)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *sessionRepository) Delete(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

func (r *sessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	_, err := r.db.ExecContext(ctx, query)
	return err
}
