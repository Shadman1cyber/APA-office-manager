.PHONY: dev-backend dev-frontend lint up down build

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

lint:
	cd backend && gofmt -l . && go vet ./...

up:
	docker compose up --build

down:
	docker compose down

build:
	docker compose build
