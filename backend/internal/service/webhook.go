package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/devutility/webhookplatform/internal/mailer"
	"github.com/devutility/webhookplatform/internal/ratelimit"
	"github.com/google/uuid"
)

// ── Endpoint name generation ──────────────────────────────────────────────────

var nameAdjectives = []string{
	"autumn", "bold", "brave", "bright", "calm", "clean", "crisp", "dark",
	"dawn", "deep", "dry", "dusk", "early", "fair", "fast", "fierce",
	"fine", "firm", "flat", "free", "fresh", "grand", "great", "hard",
	"high", "keen", "kind", "late", "lean", "light", "long", "loud",
	"mild", "mint", "neat", "new", "noble", "old", "pale", "plain",
	"prime", "proud", "pure", "quick", "quiet", "rare", "raw", "rich",
	"rough", "safe", "sage", "sharp", "shy", "slim", "slow", "smart",
	"soft", "solid", "stark", "still", "stone", "swift", "tall", "tame",
	"thin", "true", "vast", "warm", "wild", "wise", "young",
}

var nameNouns = []string{
	"bear", "bolt", "brook", "cliff", "cloud", "coast", "crane", "creek",
	"deer", "delta", "dove", "drake", "dune", "eagle", "echo", "falcon",
	"fern", "field", "finch", "fjord", "flame", "fleet", "flint", "flow",
	"foam", "ford", "forge", "fox", "frost", "gale", "gate", "glade",
	"glen", "grove", "gulf", "gust", "hawk", "haze", "heath", "helm",
	"heron", "hill", "horn", "hound", "iris", "isle", "jade", "jay",
	"kite", "lake", "lark", "leaf", "ledge", "loch", "loon", "lynx",
	"marsh", "mast", "mesa", "mill", "mink", "mist", "moose", "moss",
	"mount", "oak", "otter", "owl", "peak", "pine", "pond", "puma",
	"quail", "raven", "reef", "ridge", "rift", "river", "robin", "rock",
	"rush", "seal", "shore", "slate", "snake", "snow", "sparrow", "stone",
	"stork", "storm", "stream", "teal", "tern", "tide", "tiger", "trail",
	"vale", "vole", "wave", "wolf", "wood", "wren",
}

func generateEndpointName() string {
	adj := nameAdjectives[rand.Intn(len(nameAdjectives))]
	noun := nameNouns[rand.Intn(len(nameNouns))]
	num := rand.Intn(9000) + 1000 // 4-digit: 1000–9999
	return fmt.Sprintf("%s-%s-%d", adj, noun, num)
}

const (
	maxBodyBytes     = 10 << 20          // 10 MB per request
	endpointLifetime = 24 * time.Hour    // default endpoint TTL
	maxStoredReqs    = 500               // max requests kept per endpoint (rolling)
	rlRate           = 5.0               // sustained req/s per endpoint
	rlBurst          = 20               // burst capacity per endpoint
)

// ErrRateLimited is returned when an endpoint exceeds its ingestion rate.
var ErrRateLimited = fmt.Errorf("rate limit exceeded")

// Broadcaster pushes new webhook requests to connected SSE clients.
type Broadcaster interface {
	Broadcast(req *domain.WebhookRequest)
}

// WebhookService is the application-layer service for webhook operations.
// It depends only on domain interfaces — concrete implementations are injected via New.
type WebhookService struct {
	endpointRepo domain.EndpointRepo
	requestRepo  domain.RequestRepo
	storage      domain.PayloadStorage
	publisher    domain.EventPublisher
	replayer     domain.Replayer
	broadcaster  Broadcaster
	mailer       *mailer.Mailer
	limiter      ratelimit.RateLimiter
	log          *slog.Logger
}

func New(
	endpointRepo domain.EndpointRepo,
	requestRepo domain.RequestRepo,
	storage domain.PayloadStorage,
	publisher domain.EventPublisher,
	replayer domain.Replayer,
	broadcaster Broadcaster,
	m *mailer.Mailer,
	rl ratelimit.RateLimiter,
	log *slog.Logger,
) *WebhookService {
	return &WebhookService{
		endpointRepo: endpointRepo,
		requestRepo:  requestRepo,
		storage:      storage,
		publisher:    publisher,
		broadcaster:  broadcaster,
		replayer:     replayer,
		mailer:       m,
		limiter:      rl,
		log:          log,
	}
}

// bodyPreview returns up to 500 runes of body as a UTF-8 string for search.
func bodyPreview(b []byte) string {
	s := string(b)
	if utf8.RuneCountInString(s) <= 500 {
		return s
	}
	i := 0
	for n := 0; n < 500; n++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}

// ── Endpoint management ───────────────────────────────────────────────────────

func (s *WebhookService) ListEndpoints(ctx context.Context, userID string) ([]*domain.Endpoint, error) {
	endpoints, err := s.endpointRepo.ListEndpoints(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service: list endpoints: %w", err)
	}
	return endpoints, nil
}

// ttlDuration maps a TTL key to a duration. Zero means no expiry.
func ttlDuration(ttl string) time.Duration {
	switch ttl {
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "never":
		return 0
	default: // "24h" or empty
		return endpointLifetime
	}
}

func (s *WebhookService) CreateEndpoint(ctx context.Context, targetURL, userID, ttl string) (*domain.Endpoint, error) {
	now := time.Now().UTC()
	dur := ttlDuration(ttl)
	var expiresAt time.Time
	if dur == 0 {
		expiresAt = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC) // effectively never
	} else {
		expiresAt = now.Add(dur)
	}
	e := &domain.Endpoint{
		ID:                  uuid.NewString(),
		Name:                generateEndpointName(),
		TargetURL:           targetURL,
		UserID:              userID,
		CreatedAt:           now,
		ExpiresAt:           expiresAt,
		ResponseStatus:      200,
		ResponseContentType: "application/json",
		ResponseHeaders:     map[string]string{},
		ResponseBody:        "",
	}
	if err := s.endpointRepo.CreateEndpoint(ctx, e); err != nil {
		return nil, fmt.Errorf("service: create endpoint: %w", err)
	}
	s.log.Info("endpoint created", "id", e.ID, "name", e.Name, "ttl", ttl)
	return e, nil
}

// DeleteAllRequests removes all requests captured by an endpoint.
func (s *WebhookService) DeleteAllRequests(ctx context.Context, endpointID string) error {
	// Also purge S3 objects for this endpoint.
	if err := s.storage.DeleteByPrefix(ctx, endpointID+"/"); err != nil {
		s.log.Error("delete all requests: purge s3", "endpoint_id", endpointID, "error", err)
	}
	if err := s.requestRepo.DeleteAllRequests(ctx, endpointID); err != nil {
		return fmt.Errorf("service: delete all requests: %w", err)
	}
	s.log.Info("all requests deleted", "endpoint_id", endpointID)
	return nil
}

func (s *WebhookService) DeleteEndpoint(ctx context.Context, id string) error {
	// Purge all S3 objects for this endpoint (stored under prefix "{endpointID}/").
	if err := s.storage.DeleteByPrefix(ctx, id+"/"); err != nil {
		s.log.Error("delete endpoint: purge s3 objects", "endpoint_id", id, "error", err)
		// Non-fatal — proceed to delete from DB so the user isn't stuck.
	}
	if err := s.endpointRepo.DeleteEndpoint(ctx, id); err != nil {
		return fmt.Errorf("service: delete endpoint: %w", err)
	}
	s.log.Info("endpoint deleted", "id", id)
	return nil
}

func (s *WebhookService) GetEndpoint(ctx context.Context, id string) (*domain.Endpoint, error) {
	e, err := s.endpointRepo.GetEndpointByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: get endpoint: %w", err)
	}
	return e, nil
}

// RenameEndpoint updates the display name of an endpoint.
func (s *WebhookService) RenameEndpoint(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("service: name cannot be empty")
	}
	if err := s.endpointRepo.UpdateEndpointName(ctx, id, name); err != nil {
		return fmt.Errorf("service: rename endpoint: %w", err)
	}
	return nil
}

// UpdateEndpointNotify sets the notification email for an endpoint.
func (s *WebhookService) UpdateEndpointNotify(ctx context.Context, id, email string) error {
	if err := s.endpointRepo.UpdateEndpointNotify(ctx, id, email); err != nil {
		return fmt.Errorf("service: update endpoint notify: %w", err)
	}
	return nil
}

// UpdateEndpointResponse saves custom response config for an endpoint.
func (s *WebhookService) UpdateEndpointResponse(ctx context.Context, id string, status int, contentType string, headers map[string]string, body string) error {
	if err := s.endpointRepo.UpdateEndpointResponse(ctx, id, status, contentType, headers, body); err != nil {
		return fmt.Errorf("service: update endpoint response: %w", err)
	}
	s.log.Info("endpoint response updated", "id", id, "status", status)
	return nil
}

// GetLatestReplayResult returns the most recent replay result for a request.
func (s *WebhookService) GetLatestReplayResult(ctx context.Context, requestID string) (*domain.ReplayResult, error) {
	result, err := s.requestRepo.GetLatestReplayResult(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("service: get replay result: %w", err)
	}
	return result, nil
}

// ── Webhook ingestion ─────────────────────────────────────────────────────────

// IngestWebhook captures a raw HTTP request and persists it.
// Returns the saved request and the endpoint (for custom response).
func (s *WebhookService) IngestWebhook(ctx context.Context, endpointID string, r *http.Request) (*domain.WebhookRequest, *domain.Endpoint, error) {
	// Validate the endpoint exists before doing any expensive work.
	ep, err := s.endpointRepo.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return nil, nil, fmt.Errorf("service: ingest: %w", err)
	}

	// Rate limit: 5 req/s sustained, burst of 20, per endpoint.
	if !s.limiter.Allow(endpointID) {
		return nil, nil, ErrRateLimited
	}

	// Read body with a hard size cap.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("service: read body: %w", err)
	}

	id := uuid.NewString()
	s3Key := fmt.Sprintf("%s/%s/body", endpointID, id)
	contentType := r.Header.Get("Content-Type")

	// Upload body to S3.
	if err := s.storage.Upload(ctx, s3Key, body, contentType); err != nil {
		return nil, nil, fmt.Errorf("service: upload payload: %w", err)
	}

	req := &domain.WebhookRequest{
		ID:          id,
		EndpointID:  endpointID,
		Method:      r.Method,
		Headers:     r.Header,
		QueryParams: r.URL.Query(),
		ContentType: contentType,
		BodySize:    int64(len(body)),
		BodyPreview: bodyPreview(body),
		S3Key:       s3Key,
		CreatedAt:   time.Now().UTC(),
	}

	// Persist metadata to Postgres.
	if err := s.requestRepo.SaveRequest(ctx, req); err != nil {
		return nil, nil, fmt.Errorf("service: save request metadata: %w", err)
	}

	// Rolling cap: keep at most maxStoredReqs per endpoint; evict oldest.
	go func() {
		count, err := s.requestRepo.CountRequests(context.Background(), endpointID)
		if err == nil && count > maxStoredReqs {
			_ = s.requestRepo.DeleteOldestRequests(context.Background(), endpointID, count-maxStoredReqs)
		}
	}()

	// Send notification email asynchronously (non-blocking).
	if ep.NotifyEmail != "" {
		go func() {
			subject := fmt.Sprintf("[WebhookDB] New %s request on %s", r.Method, ep.Name)
			msgBody := fmt.Sprintf(
				"Endpoint: %s\nMethod: %s\nContent-Type: %s\nBody size: %d bytes\n\nPreview:\n%s",
				ep.Name, r.Method, contentType, len(body), bodyPreview(body),
			)
			if err := s.mailer.Send(ep.NotifyEmail, subject, msgBody); err != nil {
				s.log.Error("send notification email", "error", err, "to", ep.NotifyEmail)
			}
		}()
	}

	// Publish event to NATS (non-blocking: log error, don't fail the request).
	if err := s.publisher.PublishWebhookReceived(ctx, req); err != nil {
		s.log.Error("publish webhook event", "error", err, "request_id", id)
	}

	// Push to connected SSE clients instantly — no DB polling needed.
	s.broadcaster.Broadcast(req)

	s.log.Info("webhook ingested", "id", id, "endpoint_id", endpointID, "method", r.Method, "bytes", req.BodySize)
	return req, ep, nil
}

// ── Request listing ───────────────────────────────────────────────────────────

func (s *WebhookService) ListRequests(ctx context.Context, endpointID string, limit, offset int) ([]*domain.WebhookRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	reqs, err := s.requestRepo.ListRequestsByEndpoint(ctx, endpointID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("service: list requests: %w", err)
	}
	return reqs, nil
}

// ── Single request ────────────────────────────────────────────────────────────

// GetRequest returns full request metadata (headers + query params included).
func (s *WebhookService) GetRequest(ctx context.Context, requestID string) (*domain.WebhookRequest, error) {
	req, err := s.requestRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("service: get request: %w", err)
	}
	return req, nil
}

// ── Body retrieval ────────────────────────────────────────────────────────────

// GetRequestBody fetches the raw payload from S3/MinIO.
// Returns (body bytes, content-type, error).
func (s *WebhookService) GetRequestBody(ctx context.Context, requestID string) ([]byte, string, error) {
	req, err := s.requestRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return nil, "", fmt.Errorf("service: get request body: %w", err)
	}
	body, err := s.storage.Download(ctx, req.S3Key)
	if err != nil {
		return nil, "", fmt.Errorf("service: download body: %w", err)
	}
	return body, req.ContentType, nil
}

// ── Replay ────────────────────────────────────────────────────────────────────

// EnqueueReplay looks up the request and its endpoint's target URL,
// then submits a job to the async replay worker pool.
// targetURLOverride, when non-empty, takes precedence over the endpoint's stored URL.
func (s *WebhookService) EnqueueReplay(ctx context.Context, requestID, targetURLOverride string) error {
	req, err := s.requestRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("service: replay fetch request: %w", err)
	}

	targetURL := targetURLOverride
	if targetURL == "" {
		endpoint, err := s.endpointRepo.GetEndpointByID(ctx, req.EndpointID)
		if err != nil {
			return fmt.Errorf("service: replay fetch endpoint: %w", err)
		}
		targetURL = endpoint.TargetURL
	}

	if targetURL == "" {
		return fmt.Errorf("service: no target URL configured for replay")
	}

	if err := s.replayer.Enqueue(domain.ReplayJob{
		RequestID: requestID,
		TargetURL: targetURL,
	}); err != nil {
		return fmt.Errorf("service: enqueue replay: %w", err)
	}

	s.log.Info("replay enqueued", "request_id", requestID, "target_url", targetURL)
	return nil
}
