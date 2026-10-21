ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: db db-down migrate-up migrate-down seed run test

db: 
	docker compose up -d db

db-down: 
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

seed:
	docker compose exec -T db psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -v ON_ERROR_STOP=1 < seed/seed.sql

run:
	go run ./cmd/api

test:
	go test ./... -race -count=1