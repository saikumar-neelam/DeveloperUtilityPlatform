package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/devutility/webhookplatform/internal/handler"
	"github.com/devutility/webhookplatform/internal/hub"
	"github.com/devutility/webhookplatform/internal/mailer"
	"github.com/devutility/webhookplatform/internal/migration"
	"github.com/devutility/webhookplatform/internal/ratelimit"
	natspub "github.com/devutility/webhookplatform/internal/publisher/nats"
	"github.com/devutility/webhookplatform/internal/repository/postgres"
	s3store "github.com/devutility/webhookplatform/internal/storage/s3"
	"github.com/devutility/webhookplatform/internal/service"
	"github.com/devutility/webhookplatform/internal/worker"
	"github.com/joho/godotenv"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port        string
	PostgresDSN string
	NATSUrl     string
	S3Bucket    string
	AWSRegion   string
	// S3Endpoint is empty for real AWS S3; set to e.g. "http://localhost:9000" for MinIO.
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	// FrontendURL is used for CORS; defaults to localhost:3000 for development.
	FrontendURL string
	// JWTSecret signs and verifies auth tokens. Must be set in production.
	JWTSecret string
	// SMTP config for email notifications (all optional — disabled if SMTPHost is empty).
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	RedisURL string
}

func loadConfig() Config {
	return Config{
		Port:        env("PORT", "8080"),
		PostgresDSN: env("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/webhookdb?sslmode=disable"),
		NATSUrl:     env("NATS_URL", "nats://localhost:4222"),
		S3Bucket:    env("S3_BUCKET", "webhook-payloads"),
		AWSRegion:   env("AWS_REGION", "us-east-1"),
		S3Endpoint:  env("S3_ENDPOINT", ""),          // empty = real AWS S3
		S3AccessKey: env("AWS_ACCESS_KEY_ID", ""),
		S3SecretKey: env("AWS_SECRET_ACCESS_KEY", ""),
		FrontendURL: env("FRONTEND_URL", "http://localhost:3000"),
		JWTSecret:   env("JWT_SECRET", "change-me-in-production"),
		SMTPHost:    env("SMTP_HOST", ""),
		SMTPPort:    env("SMTP_PORT", "587"),
		SMTPUser:    env("SMTP_USER", ""),
		SMTPPass:    env("SMTP_PASS", ""),
		SMTPFrom:    env("SMTP_FROM", "notifications@debugflow.dev"),
		RedisURL:    env("REDIS_URL", "redis://redis:6379"),
	}
}

// ensureDatabase creates the target database if it does not already exist.
// It connects to the "postgres" maintenance database on the same host, so the
// target database doesn't need to exist yet.
func ensureDatabase(ctx context.Context, dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return fmt.Errorf("no database name in DSN")
	}

	// Connect to the maintenance database instead.
	u.Path = "/postgres"
	conn, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return fmt.Errorf("connect maintenance db: %w", err)
	}
	defer conn.Close()

	var exists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}

	if !exists {
		// Database name comes from our own DSN — safe to interpolate.
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
			return fmt.Errorf("create database %q: %w", dbName, err)
		}
	}
	return nil
}

func main() {
	// Load .env if present (no-op in production where env vars are set externally).
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// Non-fatal: log and continue — real envs may already be set.
		slog.Warn(".env load skipped", "reason", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg := loadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Dependencies ──────────────────────────────────────────────────────────

	// Postgres
	if err := ensureDatabase(ctx, cfg.PostgresDSN); err != nil {
		log.Error("ensure database", "error", err)
		os.Exit(1)
	}

	dbpool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(ctx); err != nil {
		log.Error("ping postgres", "error", err)
		os.Exit(1)
	}
	log.Info("postgres connected")

	// Run SQL migrations before anything else touches the schema.
	if err := migration.Run(ctx, dbpool, log); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// NATS
	pub, err := natspub.New(cfg.NATSUrl)
	if err != nil {
		log.Error("connect nats", "error", err)
		os.Exit(1)
	}
	defer pub.Close()
	log.Info("nats connected")

	// S3 / MinIO
	storageMode := "aws-s3"
	if cfg.S3Endpoint != "" {
		storageMode = "minio(" + cfg.S3Endpoint + ")"
	}
	store, err := s3store.New(ctx, s3store.Config{
		Bucket:    cfg.S3Bucket,
		Region:    cfg.AWSRegion,
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		log.Error("init storage", "error", err)
		os.Exit(1)
	}
	log.Info("storage ready", "bucket", cfg.S3Bucket, "mode", storageMode)

	// Repository (single struct satisfies both EndpointRepo and RequestRepo)
	repo := postgres.New(dbpool)

	// Replay worker pool (8 concurrent workers)
	replayWorker := worker.NewReplayWorker(8, repo, store, log)
	replayWorker.Start(ctx)
	log.Info("replay worker pool started", "concurrency", 8)

	// SSE hub
	sseHub := hub.New(log)

	// Mailer (nil if SMTP_HOST not set)
	ml := mailer.New(mailer.Config{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	})

	// Rate limiter — Redis sliding window (5 req/s per endpoint, 1-second window)
	rl, err := ratelimit.NewRedis(cfg.RedisURL, time.Second, 5)
	if err != nil {
		log.Warn("redis rate limiter unavailable, falling back to in-memory", "error", err)
	}
	var limiter ratelimit.RateLimiter
	if rl != nil {
		limiter = rl
		log.Info("rate limiter ready", "backend", "redis")
	} else {
		limiter = ratelimit.New(5, 20)
		log.Info("rate limiter ready", "backend", "in-memory")
	}

	// Service
	svc := service.New(repo, repo, store, pub, replayWorker, sseHub, ml, limiter, log)

	// Cleanup worker — deletes expired endpoints and their S3 objects every 5 min
	cleanupWorker := worker.NewCleanupWorker(repo, store, log)
	cleanupWorker.Start(ctx)
	log.Info("cleanup worker started")

	// Auth
	authSvc := service.NewAuth(repo, cfg.JWTSecret, log)
	authHandler := handler.NewAuth(authSvc)

	// Handler
	h := handler.New(svc, sseHub, log)

	// ── Router ────────────────────────────────────────────────────────────────

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.FrontendURL))
	// Note: middleware.Timeout is NOT applied globally because SSE connections
	// are long-lived. Regular API handlers are fast; no per-route timeout needed.

	// Auth routes (optional — accounts exist but are not enforced)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)
	r.Get("/auth/me", authHandler.Me)

	h.Routes(r)

	// ── HTTP server ───────────────────────────────────────────────────────────

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%s", cfg.Port),
		Handler:     r,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout must be 0 (unlimited) because SSE connections stream
		// indefinitely. Individual handlers use context deadlines instead.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCh
	log.Info("shutting down...")
	cancel() // stop replay workers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("server stopped")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// corsMiddleware allows the frontend origin to call the backend directly.
// Required so EventSource (SSE) can connect to port 8080 from port 3000.
func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
