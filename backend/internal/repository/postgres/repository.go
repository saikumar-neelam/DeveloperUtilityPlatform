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
	headers, _ := json.Marshal(e.ResponseHeaders)
	var userID *string
	if e.UserID != "" {
		userID = &e.UserID
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO endpoints
		 (id, name, target_url, user_id, created_at, expires_at,
		  response_status, response_content_type, response_headers, response_body, notify_email)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		e.ID, e.Name, e.TargetURL, userID, e.CreatedAt, e.ExpiresAt,
		e.ResponseStatus, e.ResponseContentType, headers, e.ResponseBody, e.NotifyEmail,
	)
	if err != nil {
		return fmt.Errorf("postgres: create endpoint: %w", err)
	}
	return nil
}

func (r *Repository) ListEndpoints(ctx context.Context, userID string) ([]*domain.Endpoint, error) {
	var rows pgx.Rows
	var err error
	if userID == "" {
		rows, err = r.db.Query(ctx,
			`SELECT id, name, target_url, user_id, created_at, expires_at,
			        response_status, response_content_type, response_headers, response_body, notify_email
			 FROM endpoints ORDER BY created_at DESC`,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, name, target_url, user_id, created_at, expires_at,
			        response_status, response_content_type, response_headers, response_body, notify_email
			 FROM endpoints WHERE user_id = $1 ORDER BY created_at DESC`, userID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*domain.Endpoint
	for rows.Next() {
		e, err := scanEndpoint(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan endpoint: %w", err)
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

func (r *Repository) GetEndpointByID(ctx context.Context, id string) (*domain.Endpoint, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, target_url, user_id, created_at, expires_at,
		        response_status, response_content_type, response_headers, response_body, notify_email
		 FROM endpoints WHERE id = $1`, id,
	)
	e, err := scanEndpoint(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("endpoint %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get endpoint: %w", err)
	}
	return e, nil
}

func (r *Repository) ListExpiredEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, target_url, user_id, created_at, expires_at,
		        response_status, response_content_type, response_headers, response_body, notify_email
		 FROM endpoints WHERE expires_at <= NOW() ORDER BY expires_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list expired endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*domain.Endpoint
	for rows.Next() {
		e, err := scanEndpoint(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan expired endpoint: %w", err)
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

func (r *Repository) DeleteEndpoint(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete endpoint: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEndpointName(ctx context.Context, id, name string) error {
	_, err := r.db.Exec(ctx, `UPDATE endpoints SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return fmt.Errorf("postgres: update endpoint name: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEndpointNotify(ctx context.Context, id, email string) error {
	_, err := r.db.Exec(ctx, `UPDATE endpoints SET notify_email = $2 WHERE id = $1`, id, email)
	if err != nil {
		return fmt.Errorf("postgres: update endpoint notify: %w", err)
	}
	return nil
}

func (r *Repository) UpdateEndpointResponse(ctx context.Context, id string, status int, contentType string, headers map[string]string, body string) error {
	headersJSON, _ := json.Marshal(headers)
	_, err := r.db.Exec(ctx,
		`UPDATE endpoints
		 SET response_status = $2, response_content_type = $3,
		     response_headers = $4, response_body = $5
		 WHERE id = $1`,
		id, status, contentType, headersJSON, body,
	)
	if err != nil {
		return fmt.Errorf("postgres: update endpoint response: %w", err)
	}
	return nil
}

func (r *Repository) DeleteAllRequests(ctx context.Context, endpointID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM webhook_requests WHERE endpoint_id = $1`, endpointID)
	if err != nil {
		return fmt.Errorf("postgres: delete all requests: %w", err)
	}
	return nil
}

func (r *Repository) CountRequests(ctx context.Context, endpointID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM webhook_requests WHERE endpoint_id = $1`, endpointID,
	).Scan(&n)
	return n, err
}

func (r *Repository) DeleteOldestRequests(ctx context.Context, endpointID string, n int) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM webhook_requests
		WHERE id IN (
			SELECT id FROM webhook_requests
			WHERE endpoint_id = $1
			ORDER BY created_at ASC
			LIMIT $2
		)`, endpointID, n,
	)
	return err
}

// scanEndpoint scans a row into an Endpoint using the provided scan function.
func scanEndpoint(scan func(...any) error) (*domain.Endpoint, error) {
	var e domain.Endpoint
	var headersRaw []byte
	var userID *string
	err := scan(
		&e.ID, &e.Name, &e.TargetURL, &userID, &e.CreatedAt, &e.ExpiresAt,
		&e.ResponseStatus, &e.ResponseContentType, &headersRaw, &e.ResponseBody, &e.NotifyEmail,
	)
	if err != nil {
		return nil, err
	}
	if userID != nil {
		e.UserID = *userID
	}
	if len(headersRaw) > 0 {
		_ = json.Unmarshal(headersRaw, &e.ResponseHeaders)
	}
	if e.ResponseHeaders == nil {
		e.ResponseHeaders = map[string]string{}
	}
	return &e, nil
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
		 (id, endpoint_id, method, headers, query_params, content_type, body_size, body_preview, s3_key, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		req.ID, req.EndpointID, req.Method,
		headers, queryParams,
		req.ContentType, req.BodySize, req.BodyPreview, req.S3Key, req.CreatedAt,
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
		`SELECT id, endpoint_id, method, headers, query_params, content_type, body_size, body_preview, s3_key, created_at
		 FROM webhook_requests WHERE id = $1`, id,
	).Scan(
		&req.ID, &req.EndpointID, &req.Method,
		&headersRaw, &queryRaw,
		&req.ContentType, &req.BodySize, &req.BodyPreview, &req.S3Key, &req.CreatedAt,
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
	rows, err := r.db.Query(ctx,
		`SELECT id, endpoint_id, method, content_type, body_size, body_preview, s3_key, created_at
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
			&req.ContentType, &req.BodySize, &req.BodyPreview, &req.S3Key, &req.CreatedAt,
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

func (r *Repository) GetLatestReplayResult(ctx context.Context, requestID string) (*domain.ReplayResult, error) {
	var res domain.ReplayResult
	err := r.db.QueryRow(ctx,
		`SELECT id, request_id, status_code, response_body, duration_ms, error, created_at
		 FROM replay_results
		 WHERE request_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1`, requestID,
	).Scan(&res.ID, &res.RequestID, &res.StatusCode, &res.ResponseBody, &res.DurationMs, &res.Error, &res.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no replay result for request %q", requestID)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get replay result: %w", err)
	}
	return &res, nil
}

// Compile-time interface checks.
var _ domain.EndpointRepo = (*Repository)(nil)
var _ domain.RequestRepo = (*Repository)(nil)
