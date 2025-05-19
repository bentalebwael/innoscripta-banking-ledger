.PHONY: build test lint run down clean migrate-up migrate-down

# Build both services
build:
	go build -o bin/api_gateway ./cmd/api_gateway
	go build -o bin/transaction_processor ./cmd/transaction_processor

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run linting
lint:
	golangci-lint run ./...

# Start all services with Docker Compose
run:
	docker-compose up -d

# Stop all services
down:
	docker-compose down

# Clean build artifacts
clean:
	rm -rf bin/

# Create a new migration file
migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations/postgres -seq $${name}

# Apply migrations
migrate-up:
	@export $$(cat configs/api_gateway.env | grep POSTGRES_URL | xargs) && \
	migrate -path migrations/postgres -database "$$POSTGRES_URL" up

# Rollback migrations
migrate-down:
	@export $$(cat configs/api_gateway.env | grep POSTGRES_URL | xargs) && \
	migrate -path migrations/postgres -database "$$POSTGRES_URL" down 1

# Generate API documentation
swagger:
	swag init -g cmd/api_gateway/main.go -o api/swagger

# Run services locally for development
run-dev:
	@mkdir -p bin
	@make build
	@echo "Starting services for development..."
	@echo "Starting API Gateway..."
	@CONFIG_FILE=./configs/api_gateway.env ./bin/api_gateway & echo $$! > ./bin/api_gateway.pid
	@echo "Starting Transaction Processor..."
	@CONFIG_FILE=./configs/transaction_processor.env ./bin/transaction_processor & echo $$! > ./bin/transaction_processor.pid
	@echo "Services are running. Use 'make stop-dev' to stop them."

# Stop development services
stop-dev:
	@echo "Stopping services..."
	@if [ -f "./bin/api_gateway.pid" ]; then kill $$(cat ./bin/api_gateway.pid) 2>/dev/null || true; rm ./bin/api_gateway.pid; fi
	@if [ -f "./bin/transaction_processor.pid" ]; then kill $$(cat ./bin/transaction_processor.pid) 2>/dev/null || true; rm ./bin/transaction_processor.pid; fi
	@echo "Services stopped."

# Help command
help:
	@echo "Available commands:"
	@echo "  make build            - Build both services"
	@echo "  make test             - Run tests"
	@echo "  make test-coverage    - Run tests with coverage"
	@echo "  make lint             - Run linting"
	@echo "  make run              - Start all services with Docker Compose"
	@echo "  make run-dev          - Run services locally for development"
	@echo "  make stop-dev         - Stop locally running services"
	@echo "  make down             - Stop all services"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make migrate-create   - Create a new migration file"
	@echo "  make migrate-up       - Apply migrations"
	@echo "  make migrate-down     - Rollback migrations"
	@echo "  make swagger          - Generate API documentation"