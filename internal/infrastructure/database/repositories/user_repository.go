package repositories

import (
	"context"
	"document-server/internal/domain/entities"
	"document-server/internal/domain/repositories"

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
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByLogin(ctx context.Context, login string) (*entities.User, error) {
	query := `SELECT id, login, password, created_at, updated_at FROM users WHERE login = $1`

	var user entities.User
	row := r.pool.QueryRow(ctx, query, login)

	err := row.Scan(&user.ID, &user.Login, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *entities.User) error {
	query := `UPDATE users SET password = $1, updated_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, user.Password, user.UpdatedAt, user.ID)
	return err
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}
