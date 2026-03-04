.PHONY: all build run test clean docker-build docker-run frontend-dev frontend-build dev rustfs rustfs-stop rustfs-logs swagger swagger-fmt help ensure-embed-dir

# Default target
all: build

# Build the Go binary with embedded frontend
build: frontend-build
	@echo "Building enlace..."
	go build -ldflags="-X main.version=$(git describe --tags --dirty --always)" -o enlace ./cmd/enlace

# Build without frontend (for faster iteration during backend development)
build-backend:
	@echo "Building backend only..."
	go build -ldflags="-X main.version=$(git describe --tags --dirty --always)" -o enlace ./cmd/enlace

# Run the application locally
run: build
	./enlace

# Run backend only (assumes frontend is already built)
run-backend:
	go run ./cmd/enlace

# Live reload development (air + pnpm dev via Procfile.dev)
dev: frontend-install ensure-embed-dir
	goreman -f Procfile.dev start || overmind start -f Procfile.dev

# Ensure frontend/dist exists for Go embed (placeholder for dev/test)
ensure-embed-dir:
	@mkdir -p frontend/dist && touch frontend/dist/.gitkeep

# Run all tests
test: ensure-embed-dir
	go test ./... -v

# Run tests with coverage
test-coverage: ensure-embed-dir
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	rm -f enlace coverage.out coverage.html db/enlace.db ./enlace.db
	rm -rf frontend/dist
	rm -rf frontend/node_modules
	rm -rf uploads
	@$(MAKE) ensure-embed-dir

# Frontend development server
frontend-dev:
	cd frontend && pnpm dev

frontend-install:
	cd frontend && pnpm install

# Build frontend for production
frontend-build: frontend-install
	cd frontend && pnpm build

# Build Docker image
docker-build:
	docker build -t enlace:latest .

# Run Docker container
docker-run:
	docker run -p 8080:8080 enlace:latest

# Run with docker-compose
docker-up:
	docker-compose up -d

# Stop docker-compose
docker-down:
	docker-compose down

# View docker-compose logs
docker-logs:
	docker-compose logs -f

# Lint Go code
lint: ensure-embed-dir
	go vet ./...

# Format Go code
fmt:
	cd frontend && pnpm run format
	go fmt ./...

# Generate OpenAPI/Swagger docs
swagger:
	swag init -g cmd/enlace/main.go -o docs --parseDependency --parseInternal

# Format swagger annotations
swagger-fmt:
	swag fmt

# Install development dependencies
dev-setup:
	cd frontend && pnpm install
	go mod download

# Start rustfs (S3-compatible storage) for development
rustfs:
	docker-compose -f docker-compose-dev.yml up -d rustfs

# Stop rustfs
rustfs-stop:
	docker-compose -f docker-compose-dev.yml stop rustfs

# View rustfs logs
rustfs-logs:
	docker-compose -f docker-compose-dev.yml logs -f rustfs

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application with frontend"
	@echo "  build-backend  - Build backend only (faster)"
	@echo "  run            - Build and run the application"
	@echo "  run-backend    - Run backend with go run"
	@echo "  dev            - Live reload dev (air + pnpm dev)"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  frontend-dev   - Start frontend dev server"
	@echo "  frontend-build - Build frontend for production"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-up      - Start with docker-compose"
	@echo "  docker-down    - Stop docker-compose"
	@echo "  docker-logs    - View docker-compose logs"
	@echo "  rustfs         - Start rustfs S3 storage (dev)"
	@echo "  rustfs-stop    - Stop rustfs"
	@echo "  rustfs-logs    - View rustfs logs"
	@echo "  lint           - Run Go linter"
	@echo "  fmt            - Format Go code"
	@echo "  swagger        - Generate OpenAPI/Swagger docs"
	@echo "  swagger-fmt    - Format swagger annotations"
	@echo "  dev-setup      - Install development dependencies"
