package handler

import (
	"context"
	"encoding/json"
	"errors"
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
type WebhookService interface {
	GetEndpoint(ctx context.Context, id string) (*domain.Endpoint, error)
	ListEndpoints(ctx context.Context, userID string) ([]*domain.Endpoint, error)
	CreateEndpoint(ctx context.Context, targetURL, userID, ttl string) (*domain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id string) error
	UpdateEndpointResponse(ctx context.Context, id string, status int, contentType string, headers map[string]string, body string) error
	RenameEndpoint(ctx context.Context, id, name string) error
	DeleteAllRequests(ctx context.Context, endpointID string) error
	UpdateEndpointNotify(ctx context.Context, id, email string) error
	IngestWebhook(ctx context.Context, endpointID string, r *http.Request) (*domain.WebhookRequest, *domain.Endpoint, error)
	ListRequests(ctx context.Context, endpointID string, limit, offset int) ([]*domain.WebhookRequest, error)
	GetRequest(ctx context.Context, requestID string) (*domain.WebhookRequest, error)
	GetRequestBody(ctx context.Context, requestID string) ([]byte, string, error)
	EnqueueReplay(ctx context.Context, requestID, targetURLOverride string) error
	GetLatestReplayResult(ctx context.Context, requestID string) (*domain.ReplayResult, error)
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
	r.HandleFunc("/hook/{id}", h.IngestWebhook)

	r.Route("/api", func(r chi.Router) {
		r.Get("/endpoints", h.ListEndpoints)
		r.Post("/endpoints", h.CreateEndpoint)
		r.Get("/endpoints/{id}", h.GetEndpoint)
		r.Delete("/endpoints/{id}", h.DeleteEndpoint)
		r.Patch("/endpoints/{id}/response", h.UpdateEndpointResponse)
		r.Patch("/endpoints/{id}/name", h.RenameEndpoint)
		r.Patch("/endpoints/{id}/notify", h.UpdateEndpointNotify)
		r.Get("/endpoints/{id}/requests", h.ListRequests)
		r.Delete("/endpoints/{id}/requests", h.DeleteAllRequests)
		r.Get("/endpoints/{id}/stream", h.StreamRequests)
		r.Get("/requests/{id}", h.GetRequest)
		r.Get("/requests/{id}/body", h.GetRequestBody)
		r.Post("/requests/{id}/replay", h.ReplayRequest)
		r.Get("/requests/{id}/replay/result", h.GetReplayResult)
	})
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// IngestWebhook handles /hook/{id} for all HTTP methods.
// Responds with the endpoint's custom response configuration.
func (h *Handler) IngestWebhook(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	_, ep, err := h.svc.IngestWebhook(r.Context(), endpointID, r)
	if err != nil {
		if errors.Is(err, service.ErrRateLimited) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded — max 5 req/s per endpoint")
			return
		}
		h.log.Error("ingest webhook", "endpoint_id", endpointID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Write custom response headers.
	for k, v := range ep.ResponseHeaders {
		w.Header().Set(k, v)
	}

	// Default to JSON if body is empty and no content-type override.
	ct := ep.ResponseContentType
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(ep.ResponseStatus)

	if ep.ResponseBody != "" {
		_, _ = w.Write([]byte(ep.ResponseBody))
	}
}

// ListEndpoints handles GET /api/endpoints.
func (h *Handler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.svc.ListEndpoints(r.Context(), "")
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
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetURL string `json:"target_url"`
		TTL       string `json:"ttl"` // "24h" | "7d" | "30d" | "never"
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	endpoint, err := h.svc.CreateEndpoint(r.Context(), body.TargetURL, "", body.TTL)
	if err != nil {
		h.log.Error("create endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}
	writeJSON(w, http.StatusCreated, endpoint)
}

// UpdateEndpointNotify handles PATCH /api/endpoints/{id}/notify.
func (h *Handler) UpdateEndpointNotify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.svc.UpdateEndpointNotify(r.Context(), id, body.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update notify email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DeleteAllRequests handles DELETE /api/endpoints/{id}/requests.
func (h *Handler) DeleteAllRequests(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteAllRequests(r.Context(), id); err != nil {
		h.log.Error("delete all requests", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
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

// RenameEndpoint handles PATCH /api/endpoints/{id}/name.
func (h *Handler) RenameEndpoint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.svc.RenameEndpoint(r.Context(), id, body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

// UpdateEndpointResponse handles PATCH /api/endpoints/{id}/response.
func (h *Handler) UpdateEndpointResponse(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		Status      int               `json:"status"`
		ContentType string            `json:"content_type"`
		Headers     map[string]string `json:"headers"`
		Body        string            `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Status < 100 || body.Status > 599 {
		writeError(w, http.StatusBadRequest, "status must be 100–599")
		return
	}
	if body.Headers == nil {
		body.Headers = map[string]string{}
	}

	if err := h.svc.UpdateEndpointResponse(r.Context(), id, body.Status, body.ContentType, body.Headers, body.Body); err != nil {
		h.log.Error("update endpoint response", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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
		reqs = []*domain.WebhookRequest{}
	}
	writeJSON(w, http.StatusOK, reqs)
}

// GetRequest handles GET /api/requests/{id}.
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
		TargetURL string `json:"target_url"`
	}
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

// GetReplayResult handles GET /api/requests/{id}/replay/result.
func (h *Handler) GetReplayResult(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	result, err := h.svc.GetLatestReplayResult(r.Context(), requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// StreamRequests handles GET /api/endpoints/{id}/stream (SSE).
func (h *Handler) StreamRequests(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch := h.hub.Subscribe(endpointID)
	defer h.hub.Unsubscribe(endpointID, ch)

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
