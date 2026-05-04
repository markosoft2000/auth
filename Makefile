migrate-up: ## Database migration up
	@go run cmd/migrator/main.go up

migrate-down: ## Database migration down
	@go run cmd/migrator/main.go down

migrate-up-test: ## Run migrations for the test DB
	@CONFIG_PATH=$(TEST_CONFIG_PATH) go run cmd/migrator/main.go --migrations-path=tests/migrations up

migrate-down-test: ## Run migrations for the test DB
	@CONFIG_PATH=$(TEST_CONFIG_PATH) go run cmd/migrator/main.go --migrations-path=tests/migrations down

run-test:
	@echo "--- Building Server ---"
	@go build -o ./bin/auth-server ./cmd/server/main.go

	@echo "--- Preparation ---"

	@make migrate-up-test 

	@echo "--- Starting Test Server ---"
	# Start Server in background with test config
	@CONFIG_PATH=$(shell pwd)/$(TEST_CONFIG_PATH) ./bin/auth-server & \
	PID=$$!; \
	echo "Server PID: $$PID"; \
	sleep 2; \
	\
	echo "--- Running Tests ---"; \
	CONFIG_PATH=$(shell pwd)/$(TEST_CONFIG_PATH) go test -v -count=1 ./tests/...; \
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
