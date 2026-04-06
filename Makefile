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

PROTO_DIR = proto
GEN_DIR = pkg/gen/grpc/auth
VENDOR_DIR = vendor/protovalidate/proto/protovalidate

gen-proto:
	$(shell mkdir -p $(GEN_DIR))
	rm -rf $(GEN_DIR)/*.go

	@buf dep update && buf generate

	protoc -I $(PROTO_DIR) \
	       -I $(VENDOR_DIR) \
	       $(PROTO_DIR)/sso/sso.proto \
	       --go_out=$(GEN_DIR) --go_opt=paths=source_relative \
	       --go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative
