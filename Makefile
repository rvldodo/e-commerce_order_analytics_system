include .env
MIGRATIONS_PATH = ./migrations

APP_NAME=e-commerce_order_analytics_system

.PHONY: tidy build build-cli build-cron run-api run-cli run-cron migration migration-up migration-down migration-force migration-version

tidy:
	@go mod tidy

build:
	@echo "Building $(APP_NAME) API..."
	@go build -o bin/api ./cmd/api

build-cli:
	@echo "Building $(APP_NAME) CLI..."
	@go build -o bin/cli ./cmd/cli

build-cron:
	@echo "Building $(APP_NAME) Cron..."
	@go build -o bin/cron ./cmd/cron

run-api: build
	@./bin/api

run-cli: build-cli
	@./bin/cli $(ARGS)

run-cron: build-cron
	@./bin/cron

.PHONY: migrate-create
migration:
	@migrate create -seq -ext sql -dir ${MIGRATIONS_PATH} ${filter-out $@,$(MAKECMDGOALS)}

.PHONY: migrate-up
migration-up:
	@migrate -path=${MIGRATIONS_PATH} -database=${DB_MIGRATOR_ADDR} up

.PHONY: migrate-down
migration-down:
	@migrate -path=${MIGRATIONS_PATH} -database=${DB_MIGRATOR_ADDR} down ${filter-out $@,$(MAKECMDGOALS)}

.PHONY: migration-force
migration-force:
	@migrate -path=${MIGRATIONS_PATH} -database=${DB_MIGRATOR_ADDR} force ${VERSION}

.PHONY: migration-version
migration-version:
	@migrate -path=${MIGRATIONS_PATH} -database=${DB_MIGRATOR_ADDR} version
