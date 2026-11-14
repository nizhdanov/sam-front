SHELL := /bin/bash
SERVICES := $(notdir $(wildcard services/*))
COMPOSE  := docker compose
BUF      := go run github.com/bufbuild/buf/cmd/buf@v1.75.0

.DEFAULT_GOAL := help
.PHONY: help env keys tokens up down build logs ps seed reference proto fmt vet lint smoke

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
	@echo; echo "  services: $(SERVICES)"

env: ## Create .env from .env.example if missing
	@test -f .env || cp .env.example .env

keys: ## Generate the JWT key pair into ./secrets
	go run ./tools/devtoken -generate

tokens: ## Print demo JWTs for the hardcoded user and admin
	@go run ./tools/devtoken

up: env keys ## Build and start the stack
	$(COMPOSE) up -d --build

down: ## Stop the stack (add V=1 to drop volumes)
	$(COMPOSE) down $(if $(V),-v,)

build: ## Build all service binaries into ./bin
	@mkdir -p bin
	@for s in $(SERVICES); do echo ">> build $$s"; CGO_ENABLED=0 go build -trimpath -o bin/$$s ./services/$$s/cmd/$$s || exit 1; done

logs: ## Follow logs (SERVICE=name for one)
	$(COMPOSE) logs -f $(SERVICE)

ps: ## Show container status
	$(COMPOSE) ps

seed: ## Load demo data into license_db (idempotent)
	$(COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U license -d license_db -f /seed/license.sql
	@curl -sf -o /dev/null -X POST -H "Authorization: Bearer $$(go run ./tools/devtoken | sed -n 5p)" \
		http://localhost:$${GATEWAY_PORT:-8080}/api/v1/compliance/recalculate && echo "compliance recalculated"

reference: ## Enter reference data (agreements, catalog, contracts, rules) via the admin API
	./scripts/load-reference.sh

proto: ## Lint and generate gRPC code into gen/go
	$(BUF) lint
	$(BUF) generate

fmt: ## Format code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

lint: vet ## Run linters available locally
	$(BUF) lint

smoke: ## Run the end-to-end smoke scenario against the running stack
	./scripts/smoke.sh
