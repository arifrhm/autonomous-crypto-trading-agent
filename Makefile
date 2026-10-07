.PHONY: all build run test test-coverage lint migrate-up migrate-down docker-up docker-down clean

APP_NAME=agent
CMD_PATH=cmd/agent/main.go
MIGRATIONS_DIR=migrations
DB_URL="postgres://postgres:postgres@localhost:5432/crypto_agent?sslmode=disable"

all: test build

build:
	@echo "==> Building binary..."
	@mkdir -p bin
	go build -ldflags="-w -s" -o bin/$(APP_NAME) $(CMD_PATH)

run:
	@echo "==> Running application..."
	go run $(CMD_PATH)

test:
	@echo "==> Running unit tests..."
	go test -v -race -cover ./...

test-coverage:
	@echo "==> Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	@echo "==> Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed" && exit 1)
	golangci-lint run ./...

migrate-up:
	@echo "==> Applying database migrations..."
	migrate -path $(MIGRATIONS_DIR) -database $(DB_URL) -verbose up

migrate-down:
	@echo "==> Rolling back migrations..."
	migrate -path $(MIGRATIONS_DIR) -database $(DB_URL) -verbose down 1

docker-up:
	@echo "==> Starting local infrastructure (Postgres, Redis, Prometheus, Grafana)..."
	docker compose up -d

docker-down:
	@echo "==> Stopping infrastructure..."
	docker compose down

clean:
	@echo "==> Cleaning artifacts..."
	@rm -rf bin/ coverage.out coverage.html
