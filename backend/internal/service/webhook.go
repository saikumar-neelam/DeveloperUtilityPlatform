package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/devutility/webhookplatform/internal/domain"
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
	maxBodyBytes    = 10 << 20      // 10 MB
	endpointLifetime = 24 * time.Hour // free-tier endpoint TTL
)

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
	log          *slog.Logger
}

func New(
	endpointRepo domain.EndpointRepo,
	requestRepo domain.RequestRepo,
	storage domain.PayloadStorage,
	publisher domain.EventPublisher,
	replayer domain.Replayer,
	broadcaster Broadcaster,
	log *slog.Logger,
) *WebhookService {
	return &WebhookService{
		endpointRepo: endpointRepo,
		requestRepo:  requestRepo,
		storage:      storage,
		publisher:    publisher,
		broadcaster:  broadcaster,
		replayer:     replayer,
		log:          log,
	}
}

// ── Endpoint management ───────────────────────────────────────────────────────

func (s *WebhookService) ListEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	endpoints, err := s.endpointRepo.ListEndpoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: list endpoints: %w", err)
	}
	return endpoints, nil
}

func (s *WebhookService) CreateEndpoint(ctx context.Context, targetURL string) (*domain.Endpoint, error) {
	now := time.Now().UTC()
	e := &domain.Endpoint{
		ID:        uuid.NewString(),
		Name:      generateEndpointName(),
		TargetURL: targetURL,
		CreatedAt: now,
		ExpiresAt: now.Add(endpointLifetime),
	}
	if err := s.endpointRepo.CreateEndpoint(ctx, e); err != nil {
		return nil, fmt.Errorf("service: create endpoint: %w", err)
	}
	s.log.Info("endpoint created", "id", e.ID, "name", e.Name)
	return e, nil
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

// ── Webhook ingestion ─────────────────────────────────────────────────────────

// IngestWebhook captures a raw HTTP request and persists it.
// It stores metadata in Postgres, the body in S3, and publishes an event to NATS.
func (s *WebhookService) IngestWebhook(ctx context.Context, endpointID string, r *http.Request) (*domain.WebhookRequest, error) {
	// Validate the endpoint exists before doing any expensive work.
	if _, err := s.endpointRepo.GetEndpointByID(ctx, endpointID); err != nil {
		return nil, fmt.Errorf("service: ingest: %w", err)
	}

	// Read body with a hard size cap.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("service: read body: %w", err)
	}

	id := uuid.NewString()
	s3Key := fmt.Sprintf("%s/%s/body", endpointID, id)
	contentType := r.Header.Get("Content-Type")

	// Upload body to S3.
	if err := s.storage.Upload(ctx, s3Key, body, contentType); err != nil {
		return nil, fmt.Errorf("service: upload payload: %w", err)
	}

	req := &domain.WebhookRequest{
		ID:          id,
		EndpointID:  endpointID,
		Method:      r.Method,
		Headers:     r.Header,
		QueryParams: r.URL.Query(),
		ContentType: contentType,
		BodySize:    int64(len(body)),
		S3Key:       s3Key,
		CreatedAt:   time.Now().UTC(),
	}

	// Persist metadata to Postgres.
	if err := s.requestRepo.SaveRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("service: save request metadata: %w", err)
	}

	// Publish event to NATS (non-blocking: log error, don't fail the request).
	if err := s.publisher.PublishWebhookReceived(ctx, req); err != nil {
		s.log.Error("publish webhook event", "error", err, "request_id", id)
	}

	// Push to connected SSE clients instantly — no DB polling needed.
	s.broadcaster.Broadcast(req)

	s.log.Info("webhook ingested", "id", id, "endpoint_id", endpointID, "method", r.Method, "bytes", req.BodySize)
	return req, nil
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
