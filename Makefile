include .env

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags="-s" -o=./bin/api ./cmd/api

## build/kubernetes: build the kind kubernetes cluster and create kubeconfig
.PHONY: build/kubernetes
build/kubernetes:
	@echo 'Creating kind kubernetes cluster...'
	kind create cluster --config ./railcloneKindCluster.yml
	@echo ''
	kind get kubeconfig --name=railclone-kind > kubeconfig

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/api flags=$1: run the cmd/api application
.PHONY: run/api
run/api:
	@go run ./cmd/api -db-dsn=${POSTGRES_DSN} -cors-allowed-origins=${CORS_ALLOWED_ORIGINS} -kubeconfig=${KUBECONFIG} -smtp-username=${SMTP_USERNAME} -smtp-password=${SMTP_PASSWORD} ${flags}

## psql: connect to postgres database
.PHONY: psql
psql:
	@psql $(POSTGRES_DSN)

## migrations/new name=$1: create a new database migration
PHONY: migrations/new
migrations/new:
	@echo 'Creating migration files for ${name}...'
	@migrate create -seq -ext=.sql -dir=./migrations ${name}

## migrations/up: apply all up database migrations
.PHONY: migrations/up
migrations/up: confirm
	@echo 'Running up migrations...'
	@migrate -path ./migrations -database ${POSTGRES_DSN} up

## migrations/down: apply all down database migrations
.PHONY: migrations/down
migrations/down: confirm
	@echo 'Running down migrations...'
	@migrate -path ./migrations -database ${POSTGRES_DSN} down

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit: vendor
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

## vendor: tidy and vendor dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
	@echo 'Vendoring dependencies...'
	go mod vendor
