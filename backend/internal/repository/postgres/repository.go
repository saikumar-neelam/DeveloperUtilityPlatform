package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements domain.EndpointRepo and domain.RequestRepo.
type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── EndpointRepo ──────────────────────────────────────────────────────────────

func (r *Repository) CreateEndpoint(ctx context.Context, e *domain.Endpoint) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO endpoints (id, name, target_url, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		e.ID, e.Name, e.TargetURL, e.CreatedAt, e.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create endpoint: %w", err)
	}
	return nil
}

func (r *Repository) ListEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, target_url, created_at, expires_at FROM endpoints ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*domain.Endpoint
	for rows.Next() {
		var e domain.Endpoint
		if err := rows.Scan(&e.ID, &e.Name, &e.TargetURL, &e.CreatedAt, &e.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan endpoint: %w", err)
		}
		endpoints = append(endpoints, &e)
	}
	return endpoints, rows.Err()
}

func (r *Repository) GetEndpointByID(ctx context.Context, id string) (*domain.Endpoint, error) {
	var e domain.Endpoint
	err := r.db.QueryRow(ctx,
		`SELECT id, name, target_url, created_at, expires_at FROM endpoints WHERE id = $1`, id,
	).Scan(&e.ID, &e.Name, &e.TargetURL, &e.CreatedAt, &e.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("endpoint %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get endpoint: %w", err)
	}
	return &e, nil
}

func (r *Repository) ListExpiredEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, target_url, created_at, expires_at
		 FROM endpoints WHERE expires_at <= NOW() ORDER BY expires_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list expired endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*domain.Endpoint
	for rows.Next() {
		var e domain.Endpoint
		if err := rows.Scan(&e.ID, &e.Name, &e.TargetURL, &e.CreatedAt, &e.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan expired endpoint: %w", err)
		}
		endpoints = append(endpoints, &e)
	}
	return endpoints, rows.Err()
}

func (r *Repository) DeleteEndpoint(ctx context.Context, id string) error {
	// webhook_requests and replay_results are deleted via ON DELETE CASCADE on the FK.
	_, err := r.db.Exec(ctx, `DELETE FROM endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete endpoint: %w", err)
	}
	return nil
}

// ── RequestRepo ───────────────────────────────────────────────────────────────

func (r *Repository) SaveRequest(ctx context.Context, req *domain.WebhookRequest) error {
	headers, err := json.Marshal(req.Headers)
	if err != nil {
		return fmt.Errorf("postgres: marshal headers: %w", err)
	}
	queryParams, err := json.Marshal(req.QueryParams)
	if err != nil {
		return fmt.Errorf("postgres: marshal query params: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO webhook_requests
		 (id, endpoint_id, method, headers, query_params, content_type, body_size, s3_key, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		req.ID, req.EndpointID, req.Method,
		headers, queryParams,
		req.ContentType, req.BodySize, req.S3Key, req.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: save request: %w", err)
	}
	return nil
}

func (r *Repository) GetRequestByID(ctx context.Context, id string) (*domain.WebhookRequest, error) {
	var req domain.WebhookRequest
	var headersRaw, queryRaw []byte

	err := r.db.QueryRow(ctx,
		`SELECT id, endpoint_id, method, headers, query_params, content_type, body_size, s3_key, created_at
		 FROM webhook_requests WHERE id = $1`, id,
	).Scan(
		&req.ID, &req.EndpointID, &req.Method,
		&headersRaw, &queryRaw,
		&req.ContentType, &req.BodySize, &req.S3Key, &req.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("request %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get request: %w", err)
	}

	_ = json.Unmarshal(headersRaw, &req.Headers)
	_ = json.Unmarshal(queryRaw, &req.QueryParams)
	return &req, nil
}

func (r *Repository) ListRequestsByEndpoint(ctx context.Context, endpointID string, limit, offset int) ([]*domain.WebhookRequest, error) {
	// List query fetches only the columns needed for the table view.
	// Heavy fields (headers, query_params) are loaded on-demand via GetRequestByID.
	rows, err := r.db.Query(ctx,
		`SELECT id, endpoint_id, method, content_type, body_size, s3_key, created_at
		 FROM webhook_requests
		 WHERE endpoint_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		endpointID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list requests: %w", err)
	}
	defer rows.Close()

	var results []*domain.WebhookRequest
	for rows.Next() {
		var req domain.WebhookRequest
		if err := rows.Scan(
			&req.ID, &req.EndpointID, &req.Method,
			&req.ContentType, &req.BodySize, &req.S3Key, &req.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan request row: %w", err)
		}
		results = append(results, &req)
	}
	return results, rows.Err()
}

func (r *Repository) SaveReplayResult(ctx context.Context, result *domain.ReplayResult) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO replay_results (id, request_id, status_code, response_body, duration_ms, error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		result.ID, result.RequestID,
		result.StatusCode, result.ResponseBody,
		result.DurationMs, result.Error, result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: save replay result: %w", err)
	}
	return nil
}

// Compile-time interface checks.
var _ domain.EndpointRepo = (*Repository)(nil)
var _ domain.RequestRepo = (*Repository)(nil)
