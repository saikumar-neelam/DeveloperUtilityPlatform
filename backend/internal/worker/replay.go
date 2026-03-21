package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/devutility/webhookplatform/internal/domain"
	"github.com/google/uuid"
)

const (
	maxResponseBodyBytes = 1 << 20 // 1 MB
	httpTimeout          = 30 * time.Second
	maxRetries           = 3
)

// ReplayWorker is an async worker pool that re-sends captured webhook requests.
type ReplayWorker struct {
	jobs        chan domain.ReplayJob
	requestRepo domain.RequestRepo
	storage     domain.PayloadStorage
	httpClient  *http.Client
	log         *slog.Logger
	concurrency int
}

func NewReplayWorker(
	concurrency int,
	requestRepo domain.RequestRepo,
	storage domain.PayloadStorage,
	log *slog.Logger,
) *ReplayWorker {
	return &ReplayWorker{
		jobs:        make(chan domain.ReplayJob, 256),
		requestRepo: requestRepo,
		storage:     storage,
		httpClient:  &http.Client{Timeout: httpTimeout},
		log:         log,
		concurrency: concurrency,
	}
}

// Start launches the worker goroutines. It blocks until ctx is cancelled.
func (w *ReplayWorker) Start(ctx context.Context) {
	for i := 0; i < w.concurrency; i++ {
		go w.loop(ctx)
	}
}

// Enqueue adds a replay job to the queue. Returns an error if the queue is full.
func (w *ReplayWorker) Enqueue(job domain.ReplayJob) error {
	select {
	case w.jobs <- job:
		return nil
	default:
		return fmt.Errorf("replay queue full")
	}
}

func (w *ReplayWorker) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.jobs:
			w.process(ctx, job)
		}
	}
}

func (w *ReplayWorker) process(ctx context.Context, job domain.ReplayJob) {
	log := w.log.With("request_id", job.RequestID)

	req, err := w.requestRepo.GetRequestByID(ctx, job.RequestID)
	if err != nil {
		log.Error("replay: fetch request", "error", err)
		return
	}

	body, err := w.storage.Download(ctx, req.S3Key)
	if err != nil {
		log.Error("replay: download body", "error", err)
		return
	}

	targetURL := job.TargetURL
	result := &domain.ReplayResult{
		ID:        uuid.NewString(),
		RequestID: job.RequestID,
		CreatedAt: time.Now().UTC(),
	}

	statusCode, respBody, durationMs, sendErr := w.sendWithRetry(ctx, req.Method, targetURL, req.Headers, body)
	result.StatusCode = statusCode
	result.ResponseBody = respBody
	result.DurationMs = durationMs
	if sendErr != nil {
		result.Error = sendErr.Error()
		log.Warn("replay: send failed", "error", sendErr)
	} else {
		log.Info("replay: done", "status", statusCode, "duration_ms", durationMs)
	}

	if err := w.requestRepo.SaveReplayResult(ctx, result); err != nil {
		log.Error("replay: save result", "error", err)
	}
}

// sendWithRetry attempts to send the HTTP request up to maxRetries times
// using exponential backoff (1s, 2s, 4s).
func (w *ReplayWorker) sendWithRetry(
	ctx context.Context,
	method, targetURL string,
	headers map[string][]string,
	body []byte,
) (statusCode int, respBody string, durationMs int64, err error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return 0, "", 0, ctx.Err()
			case <-time.After(backoff):
			}
		}

		statusCode, respBody, durationMs, err = w.send(ctx, method, targetURL, headers, body)
		if err == nil {
			return
		}
		w.log.Warn("replay: attempt failed", "attempt", attempt+1, "error", err)
	}
	return
}

func (w *ReplayWorker) send(
	ctx context.Context,
	method, targetURL string,
	headers map[string][]string,
	body []byte,
) (int, string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", 0, fmt.Errorf("build request: %w", err)
	}

	// Restore original headers, skip hop-by-hop headers.
	for key, vals := range headers {
		for _, v := range vals {
			req.Header.Add(key, v)
		}
	}

	start := time.Now()
	resp, err := w.httpClient.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return 0, "", durationMs, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return resp.StatusCode, "", durationMs, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, string(respBytes), durationMs, nil
}

// Compile-time interface check.
var _ domain.Replayer = (*ReplayWorker)(nil)
