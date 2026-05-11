export CONFIG_PATH=configs/local.yaml
CONFIG_PATH = configs/local.yaml
TEST_CONFIG_PATH = configs/local_tests.yaml

PKGS = $(shell go list ./... | grep -v /vendor | grep -v grpc)

vet:
	@go vet $(PKGS) && echo "go vet: OK"

GOLANGCI_LINT_VERSION = v1.64.5
lint: ## Run golangci-lint
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not found. Installing $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@$(shell go env GOPATH)/bin/golangci-lint run ./...

fix:
	@if ! command -v fieldalignment >/dev/null 2>&1; then \
		go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest; \
	fi
	@fieldalignment -fix ./...

migrate-up: ## Database migration up
	@DB_DIRECT_HOST=localhost go run cmd/migrator/main.go --migrations-path=migrations -config=$(CONFIG_PATH) up

migrate-down: ## Database migration down
	@DB_DIRECT_HOST=localhost go run cmd/migrator/main.go --migrations-path=migrations -config=$(CONFIG_PATH) down

migrate-up-test: ## Run migrations for the test DB
	@go run cmd/migrator/main.go --migrations-path=tests/migrations -config=$(TEST_CONFIG_PATH) up

migrate-down-test: ## Run migrations for the test DB
	@go run cmd/migrator/main.go --migrations-path=tests/migrations -config=$(TEST_CONFIG_PATH) down

run-test:
	@echo "--- Building Server ---"
	@go build -o ./bin/auth-server ./cmd/server/main.go

	@echo "--- Preparation ---"

	@make migrate-up-test 

	@echo "--- Starting Test Server ---"

	@fuser -k 50001/tcp 8081/tcp 2>/dev/null || true

	# Start Server in background with test config
	@KAFKA_BOOTSTRAP_SERVERS="localhost:9092,localhost:9093,localhost:9094" ./bin/auth-server -config=$(shell pwd)/$(TEST_CONFIG_PATH) > /dev/null 2>&1 & \
	PID=$$!; \
	echo "Server PID: $$PID"; \
	sleep 2; \
	\
	echo "--- Running Tests ---"; \
	go test -v -count=1 ./tests/... -args -config=$(shell pwd)/$(TEST_CONFIG_PATH); \
	EXIT_CODE=$$?; \
	\
	echo "--- Cleaning Up ---"; \
	kill -15 $$PID; \
	wait $$PID; \
	exit $$EXIT_CODE

run:
	-@go run cmd/server/main.go

GEN_DIR = pkg/gen/grpc/auth
gen-proto:
	$(shell mkdir -p $(GEN_DIR))
	rm -rf $(GEN_DIR)/*.go
	@rm -rf proto/vendor
	@mkdir -p proto/vendor
	@rm -rf proto/sso/buf/*

	@buf dep update && buf generate proto
	@buf export . --output proto/vendor
	@cp -r proto/vendor/buf/ proto/sso/


# DOCKER
# Use the service name defined in docker-compose.yaml
DOCKER_APP_SERVICE = app
DOCKER_MIGRATOR_SERVICE = migrator

.PHONY: docker-up docker-down docker-reload docker-migrate-up docker-migrate-down

# Start everything
docker-up:
	docker compose up --build -d

# Stop everything
docker-down:
	docker compose down

# Rebuild the app and restart it without touching the DB
docker-reload:
	docker compose up --build -d --no-deps --force-recreate app

# Database migration up (inside Docker)
docker-migrate-up:
	docker compose run --rm $(DOCKER_MIGRATOR_SERVICE) up

# Database migration down (inside Docker)
docker-migrate-down:
	docker compose run --rm $(DOCKER_MIGRATOR_SERVICE) down

# Run tests inside Docker
docker-test:
	docker compose run --rm $(DOCKER_APP_SERVICE) go test -v -count=1 ./tests/...
