package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ── UserRepo ──────────────────────────────────────────────────────────────────

func (r *Repository) CreateUser(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash, plan, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.Name, u.PasswordHash, u.Plan, u.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, password_hash, plan, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Plan, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, password_hash, plan, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Plan, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get user by id: %w", err)
	}
	return &u, nil
}

// Compile-time interface check.
var _ domain.UserRepo = (*Repository)(nil)
