#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[apa]${NC} $1"; }
warn() { echo -e "${YELLOW}[apa]${NC} $1"; }
error() { echo -e "${RED}[apa ERROR]${NC} $1"; exit 1; }

check_prereqs() {
    log "Checking prerequisites..."
    ! command -v docker &> /dev/null && error "Docker is not installed"
    (docker compose version &> /dev/null || docker-compose --version &> /dev/null) || error "Docker Compose not available"
    [ -f .env ] || warn ".env file not found"
}

up() {
    check_prereqs
    log "Starting APA stack..."
    docker compose up -d --build
    log "Waiting for PostgreSQL..."
    local waited=0
    while [ $waited -lt 60 ]; do
        docker exec apa-postgres-1 pg_isready -U apa -d apa 2>/dev/null && break
        sleep 1
        waited=$((waited+1))
    done
    log "Services: Frontend http://localhost:3000, API http://localhost:8080"
    log "View: ./run.sh logs | Stop: ./run.sh down"
}

down() { log "Stopping..."; docker compose down; }

logs() { docker compose logs -f; }

dev-backend() { cd backend && go run ./cmd/server; }

dev-frontend() { cd frontend && npm run dev; }

lint() {
    cd backend && gofmt -l . && go vet ./...
    cd frontend && npm run lint 2>/dev/null || warn "Frontend lint not configured"
}

help() {
    echo "Usage: ./run.sh [command]"
    echo "  up              Start full stack"
    echo "  down            Stop stack"
    echo "  logs            Show logs"
    echo "  lint            Lint check"
    echo "  dev-backend     Go backend only"
    echo "  dev-frontend    Frontend only"
    echo "  help            Show this help"
}

case "${1:-up}" in up)   up ;; down)  down ;; logs) logs ;; lint) lint ;; dev-backend) dev-backend ;; dev-frontend) dev-frontend ;; help|--help|-h) help ;;

*)
    error "Unknown command: $1. Use './run.sh help'"
esac
