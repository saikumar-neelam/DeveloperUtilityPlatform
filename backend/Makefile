.PHONY: up down logs run dev

## Start all infrastructure containers
up:
	docker compose up -d
	@echo ""
	@echo "Services:"
	@echo "  Postgres   → localhost:5432  (db: webhookdb, user: postgres, pass: postgres)"
	@echo "  NATS       → localhost:4222  (monitoring: http://localhost:8222)"
	@echo "  MinIO API  → localhost:9000  (access: minioadmin / minioadmin)"
	@echo "  MinIO UI   → http://localhost:9001"

## Stop all containers
down:
	docker compose down

## Tail logs from all containers
logs:
	docker compose logs -f

## Run the server (reads from .env if present)
run:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi && go run ./cmd/server

## Copy .env.example → .env and run
dev:
	@[ -f .env ] || cp .env.example .env
	@make up
	@echo "Waiting for services to be healthy..."
	@sleep 3
	@make run
