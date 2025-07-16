package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"
	appErrors "document-server/pkg/errors"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) repositories.UserRepository {
	return &userRepository{pool: pool}
}

func (r *userRepository) Create(ctx context.Context, user *entities.User) error {
	query := `INSERT INTO users (login, password) VALUES ($1, $2)`
	_, err := r.pool.Exec(ctx, query, user.Login, user.Password)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	query := `SELECT id, login, password, created_at, updated_at FROM users WHERE id = $1`

	var user entities.User
	row := r.pool.QueryRow(ctx, query, id)

	err := row.Scan(&user.ID, &user.Login, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.NewNotFoundError("user not found")
		}
		return nil, appErrors.NewInternalError("user query failed")
	}
	return &user, nil
}

func (r *userRepository) GetByLogin(ctx context.Context, login string) (*entities.User, error) {
	query := `SELECT id, login, password, created_at, updated_at FROM users WHERE login = $1`

	var user entities.User
	row := r.pool.QueryRow(ctx, query, login)

	err := row.Scan(&user.ID, &user.Login, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.NewNotFoundError("user not found")
		}
		return nil, appErrors.NewInternalError("user query failed")
	}
	return &user, nil
}
