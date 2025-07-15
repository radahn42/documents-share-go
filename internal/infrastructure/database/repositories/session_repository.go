package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

type sessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) repositories.SessionRepository {
	return &sessionRepository{pool: pool}
}

func (r *sessionRepository) Create(ctx context.Context, session *entities.Session) error {
	query := `INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)`

	_, err := r.pool.Exec(ctx, query, session.UserID, session.Token, session.ExpiresAt)
	return err
}

func (r *sessionRepository) GetByToken(ctx context.Context, token string) (*entities.Session, error) {
	query := `SELECT id, user_id, token, expires_at, updated_at FROM sessions WHERE token = $1`

	var session entities.Session
	row := r.pool.QueryRow(ctx, query, token)

	err := row.Scan(&session.ID, &session.UserID, &session.Token, &session.ExpiresAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *sessionRepository) Delete(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.pool.Exec(ctx, query, token)
	return err
}

func (r *sessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	_, err := r.pool.Exec(ctx, query)
	return err
}
