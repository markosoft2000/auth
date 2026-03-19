migrate-up: ## Database migration up
	@go run cmd/migrator/main.go up

migrate-down: ## Database migration down
	@go run cmd/migrator/main.go down

run:
	-@go run cmd/server/main.go

gen-proto:
	@buf dep update && buf generate