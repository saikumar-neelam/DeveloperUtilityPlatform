package domain

import (
	"context"
	"time"
)

// ── Core types ────────────────────────────────────────────────────────────────

type Endpoint struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TargetURL string    `json:"target_url"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`

	// Custom response — what the endpoint returns to the webhook sender.
	ResponseStatus      int               `json:"response_status"`
	ResponseContentType string            `json:"response_content_type"`
	ResponseHeaders     map[string]string `json:"response_headers"`
	ResponseBody        string            `json:"response_body"`

	// Notification email — send an email when a request is received.
	NotifyEmail string `json:"notify_email"`
}

type WebhookRequest struct {
	ID          string              `json:"id"`
	EndpointID  string              `json:"endpoint_id"`
	Method      string              `json:"method"`
	Headers     map[string][]string `json:"headers"`
	QueryParams map[string][]string `json:"query_params"`
	ContentType string              `json:"content_type"`
	BodySize    int64               `json:"body_size"`
	BodyPreview string              `json:"body_preview"`
	S3Key       string              `json:"s3_key"`
	CreatedAt   time.Time           `json:"created_at"`
}

type ReplayResult struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `json:"response_body"`
	DurationMs   int64     `json:"duration_ms"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ReplayJob is the unit of work the async replay worker consumes.
type ReplayJob struct {
	RequestID string
	// TargetURL overrides the endpoint's target URL when non-empty.
	TargetURL string
}

// ── Repository interfaces ─────────────────────────────────────────────────────

// EndpointRepo persists and retrieves webhook endpoints.
type EndpointRepo interface {
	CreateEndpoint(ctx context.Context, e *Endpoint) error
	GetEndpointByID(ctx context.Context, id string) (*Endpoint, error)
	// ListEndpoints returns all endpoints owned by the given user.
	ListEndpoints(ctx context.Context, userID string) ([]*Endpoint, error)
	// ListExpiredEndpoints returns all endpoints whose expires_at is in the past.
	ListExpiredEndpoints(ctx context.Context) ([]*Endpoint, error)
	// DeleteEndpoint removes an endpoint and its associated requests (cascade).
	DeleteEndpoint(ctx context.Context, id string) error
	// UpdateEndpointResponse persists the custom response configuration.
	UpdateEndpointResponse(ctx context.Context, id string, status int, contentType string, headers map[string]string, body string) error
	// UpdateEndpointName renames an endpoint.
	UpdateEndpointName(ctx context.Context, id, name string) error
	// UpdateEndpointNotify sets the notification email for an endpoint.
	UpdateEndpointNotify(ctx context.Context, id, email string) error
}

// RequestRepo persists and retrieves captured webhook requests and replay results.
type RequestRepo interface {
	SaveRequest(ctx context.Context, r *WebhookRequest) error
	GetRequestByID(ctx context.Context, id string) (*WebhookRequest, error)
	ListRequestsByEndpoint(ctx context.Context, endpointID string, limit, offset int) ([]*WebhookRequest, error)
	SaveReplayResult(ctx context.Context, result *ReplayResult) error
	// GetLatestReplayResult returns the most recent replay result for a request.
	GetLatestReplayResult(ctx context.Context, requestID string) (*ReplayResult, error)
	// DeleteAllRequests removes every request captured by an endpoint.
	DeleteAllRequests(ctx context.Context, endpointID string) error
	// CountRequests returns the number of requests for an endpoint.
	CountRequests(ctx context.Context, endpointID string) (int, error)
	// DeleteOldestRequests removes the oldest n requests for an endpoint.
	DeleteOldestRequests(ctx context.Context, endpointID string, n int) error
}

// ── Storage interface ─────────────────────────────────────────────────────────

// PayloadStorage stores and retrieves raw webhook body bytes (S3, GCS, etc.).
type PayloadStorage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) error
	Download(ctx context.Context, key string) ([]byte, error)
	// DeleteByPrefix removes all objects whose key starts with prefix.
	DeleteByPrefix(ctx context.Context, prefix string) error
}

// ── Publisher interface ───────────────────────────────────────────────────────

// EventPublisher notifies downstream consumers of ingested webhooks.
type EventPublisher interface {
	PublishWebhookReceived(ctx context.Context, req *WebhookRequest) error
}

// ── Replayer interface ────────────────────────────────────────────────────────

// Replayer accepts replay jobs and dispatches them asynchronously.
type Replayer interface {
	Enqueue(job ReplayJob) error
	Start(ctx context.Context)
}
