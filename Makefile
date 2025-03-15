.PHONY: install run-backend run-frontend run clean docker-build docker-run docker-stop docker-logs lint lint-install lint-fix docker

# Install dependencies
install:
	@echo "Installing backend dependencies..."
	go mod tidy
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Install golangci-lint
lint-install:
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing goimports..."
	go install golang.org/x/tools/cmd/goimports@latest

# Run golangci-lint
lint:
	@echo "Running goimports..."
	goimports -w .
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Run golangci-lint with auto-fix
lint-fix:
	@echo "Running goimports..."
	goimports -w .
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix ./...

# Run backend server
run-backend:
	@echo "Starting backend server..."
	go run main.go

# Run frontend development server
run-frontend:
	@echo "Starting frontend development server..."
	cd frontend && npm start

# Run both backend and frontend (in separate terminals)
run:
	@echo "Please run the backend and frontend in separate terminals:"
	@echo "  make run-backend"
	@echo "  make run-frontend"
	@echo "Or use Docker for a simpler setup:"
	@echo "  make docker"

# Clean up
clean:
	@echo "Cleaning up..."
	rm -rf frontend/node_modules
	rm -rf frontend/build

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker-compose build

docker-run:
	@echo "Starting Docker container..."
	docker-compose up -d
	@echo "Application is running at http://localhost:8080"

docker-stop:
	@echo "Stopping Docker container..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f

# Run in Docker (build and run)
docker:
	@echo "Building and running in Docker..."
	$(MAKE) docker-build
	$(MAKE) docker-run
