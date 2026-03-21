package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/devutility/webhookplatform/internal/hub"
	"github.com/devutility/webhookplatform/internal/service"
	"github.com/go-chi/chi/v5"
)

// WebhookService is the interface the handler depends on.
// Defined here (at the consumer) following Go's interface convention.
type WebhookService interface {
	GetEndpoint(ctx context.Context, id string) (*domain.Endpoint, error)
	ListEndpoints(ctx context.Context) ([]*domain.Endpoint, error)
	CreateEndpoint(ctx context.Context, targetURL string) (*domain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id string) error
	IngestWebhook(ctx context.Context, endpointID string, r *http.Request) (*domain.WebhookRequest, error)
	ListRequests(ctx context.Context, endpointID string, limit, offset int) ([]*domain.WebhookRequest, error)
	GetRequest(ctx context.Context, requestID string) (*domain.WebhookRequest, error)
	GetRequestBody(ctx context.Context, requestID string) ([]byte, string, error)
	EnqueueReplay(ctx context.Context, requestID, targetURLOverride string) error
}

// Handler holds the HTTP handlers for the webhook platform.
type Handler struct {
	svc WebhookService
	hub *hub.SSEHub
	log *slog.Logger
}

func New(svc *service.WebhookService, h *hub.SSEHub, log *slog.Logger) *Handler {
	return &Handler{svc: svc, hub: h, log: log}
}

// Routes registers all routes on the given chi router.
func (h *Handler) Routes(r chi.Router) {
	// Webhook ingestion — accept ALL HTTP methods so senders can use any verb
	// and users can open the URL in a browser (GET) to test it.
	r.HandleFunc("/hook/{id}", h.IngestWebhook)

	// Management API.
	r.Route("/api", func(r chi.Router) {
		r.Get("/endpoints", h.ListEndpoints)
		r.Post("/endpoints", h.CreateEndpoint)
		r.Get("/endpoints/{id}", h.GetEndpoint)
		r.Delete("/endpoints/{id}", h.DeleteEndpoint)
		r.Get("/endpoints/{id}/requests", h.ListRequests)
		r.Get("/endpoints/{id}/stream", h.StreamRequests) // SSE live feed
		r.Get("/requests/{id}", h.GetRequest)
		r.Get("/requests/{id}/body", h.GetRequestBody)
		r.Post("/requests/{id}/replay", h.ReplayRequest)
	})
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// IngestWebhook handles POST /hook/{id}.
// It is the critical ingestion path — responds as fast as possible.
func (h *Handler) IngestWebhook(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	req, err := h.svc.IngestWebhook(r.Context(), endpointID, r)
	if err != nil {
		h.log.Error("ingest webhook", "endpoint_id", endpointID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"request_id":  req.ID,         // use this for /api/requests/{request_id}/replay
		"endpoint_id": req.EndpointID, // use this for /api/endpoints/{endpoint_id}/requests
		"status":      "received",
	})
}

// ListEndpoints handles GET /api/endpoints.
func (h *Handler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.svc.ListEndpoints(r.Context())
	if err != nil {
		h.log.Error("list endpoints", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	if endpoints == nil {
		endpoints = []*domain.Endpoint{}
	}
	writeJSON(w, http.StatusOK, endpoints)
}

// CreateEndpoint handles POST /api/endpoints.
// Name is auto-generated; only target_url is accepted (optional).
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetURL string `json:"target_url"`
	}
	// Body is optional — ignore decode errors.
	_ = json.NewDecoder(r.Body).Decode(&body)

	endpoint, err := h.svc.CreateEndpoint(r.Context(), body.TargetURL)
	if err != nil {
		h.log.Error("create endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}

	writeJSON(w, http.StatusCreated, endpoint)
}

// DeleteEndpoint handles DELETE /api/endpoints/{id}.
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteEndpoint(r.Context(), id); err != nil {
		h.log.Error("delete endpoint", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete endpoint")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetEndpoint handles GET /api/endpoints/{id}.
func (h *Handler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	endpoint, err := h.svc.GetEndpoint(r.Context(), id)
	if err != nil {
		h.log.Error("get endpoint", "id", id, "error", err)
		writeError(w, http.StatusNotFound, "endpoint not found")
		return
	}
	writeJSON(w, http.StatusOK, endpoint)
}

// ListRequests handles GET /api/endpoints/{id}/requests.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	reqs, err := h.svc.ListRequests(r.Context(), endpointID, limit, offset)
	if err != nil {
		h.log.Error("list requests", "endpoint_id", endpointID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	if reqs == nil {
		reqs = []*domain.WebhookRequest{} // return [] not null
	}
	writeJSON(w, http.StatusOK, reqs)
}

// GetRequest handles GET /api/requests/{id}.
// Returns full request metadata including headers and query params.
func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	req, err := h.svc.GetRequest(r.Context(), requestID)
	if err != nil {
		h.log.Error("get request", "request_id", requestID, "error", err)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// GetRequestBody handles GET /api/requests/{id}/body.
// Downloads the raw body from S3/MinIO and streams it back with the original content-type.
func (h *Handler) GetRequestBody(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")

	body, contentType, err := h.svc.GetRequestBody(r.Context(), requestID)
	if err != nil {
		h.log.Error("get request body", "request_id", requestID, "error", err)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// ReplayRequest handles POST /api/requests/{id}/replay.
func (h *Handler) ReplayRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")

	var body struct {
		TargetURL string `json:"target_url"` // optional override
	}
	// Ignore decode errors — body is optional.
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := h.svc.EnqueueReplay(r.Context(), requestID, body.TargetURL); err != nil {
		h.log.Error("enqueue replay", "request_id", requestID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":     "queued",
		"request_id": requestID,
	})
}

// StreamRequests handles GET /api/endpoints/{id}/stream.
// It opens an SSE connection and pushes new webhook requests to the browser in real time.
func (h *Handler) StreamRequests(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if present

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch := h.hub.Subscribe(endpointID)
	defer h.hub.Unsubscribe(endpointID, ch)

	// Send a heartbeat every 30 s to keep the connection alive through proxies.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
