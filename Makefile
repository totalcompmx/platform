# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

GREMLINS_VERSION = v0.6.0
GREMLINS = go run github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)
MUTATION_PATH ?= .
MUTATION_OUTPUT ?= /tmp/totalcompmx-gremlins.json
MUTATION_WORKERS ?= 0
MUTATION_TIMEOUT_COEFFICIENT ?= 5
MUTATION_THRESHOLD_EFFICACY ?= 100
MUTATION_THRESHOLD_MCOVER ?= 100

## audit: run quality control checks
.PHONY: audit
audit: test
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)" 
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## test: run unit tests
.PHONY: test
test: coverage/unit test/go test/js

## test/go: run Go unit tests
.PHONY: test/go
test/go:
	go test -v -race -buildvcs ./...

## test/integration: run tests that require external services
.PHONY: test/integration
test/integration:
	go test -v -race -buildvcs -tags=integration ./...

## test/js: run JavaScript unit tests
.PHONY: test/js
test/js: frontend/node_modules
	npm --prefix frontend run typecheck
	npm --prefix frontend run test:coverage

## coverage/unit: require 100% coverage for all Go unit packages
.PHONY: coverage/unit
coverage/unit:
	go test ./... -covermode=count -coverprofile=/tmp/unit-coverage.out
	go tool cover -func=/tmp/unit-coverage.out | tail -n 1 | grep -q '100.0%'

## mutation/dry-run: discover Go mutants without executing mutation tests
.PHONY: mutation/dry-run
mutation/dry-run:
	$(GREMLINS) unleash $(MUTATION_PATH) \
		--dry-run \
		--workers=$(MUTATION_WORKERS) \
		--output-statuses=r \
		--output=$(MUTATION_OUTPUT)

## mutation: run Go mutation tests with configurable quality thresholds
.PHONY: mutation
mutation: test
	$(GREMLINS) unleash $(MUTATION_PATH) \
		--workers=$(MUTATION_WORKERS) \
		--timeout-coefficient=$(MUTATION_TIMEOUT_COEFFICIENT) \
		--threshold-efficacy=$(MUTATION_THRESHOLD_EFFICACY) \
		--threshold-mcover=$(MUTATION_THRESHOLD_MCOVER) \
		--output=$(MUTATION_OUTPUT)

## test/cover: run unit tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

## upgradeable: list direct dependencies that have upgrades available
.PHONY: upgradeable
upgradeable:
	@go run github.com/oligot/go-mod-upgrade@latest

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## tidy: tidy modfiles and format .go files
.PHONY: tidy
tidy:
	go mod tidy -v
	go fmt ./...

frontend/node_modules: frontend/package-lock.json frontend/package.json
	npm --prefix frontend ci

## install/frontend: install frontend dependencies
.PHONY: install/frontend
install/frontend:
	npm --prefix frontend ci

VITE_DEV_SERVER_URL ?= http://127.0.0.1:5173

## build/frontend: build frontend assets with Vite
.PHONY: build/frontend
build/frontend: frontend/node_modules
	npm --prefix frontend run build

## run/frontend: run Vite development server
.PHONY: run/frontend
run/frontend: frontend/node_modules
	npm --prefix frontend run dev

## run/backend-vite: run the Go application using the Vite development server
.PHONY: run/backend-vite
run/backend-vite:
	VITE_DEV_SERVER_URL=$(VITE_DEV_SERVER_URL) go run ./cmd/web

## build: build the cmd/web application
.PHONY: build
build: build/frontend
	go build -o=/tmp/bin/web ./cmd/web
	
## run: run the cmd/web application
.PHONY: run
run: build
	/tmp/bin/web

## run/live: run the application with reloading on file changes
.PHONY: run/live
run/live:
	go run github.com/cosmtrek/air@v1.43.0 \
		--build.cmd "make build" --build.bin "/tmp/bin/web" --build.delay "100" \
		--build.exclude_dir "assets/static/dist,frontend/node_modules" \
		--build.include_ext "go, tpl, tmpl, html, css, scss, js, ts, sql, jpeg, jpg, gif, png, bmp, svg, webp, ico" \
		--misc.clean_on_exit "true"


# ==================================================================================== #
# SQL MIGRATIONS
# ==================================================================================== #

## migrations/new name=$1: create a new database migration
.PHONY: migrations/new
migrations/new:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest create -seq -ext=.sql -dir=./assets/migrations ${name}

## migrations/up: apply all up database migrations
.PHONY: migrations/up
migrations/up:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./assets/migrations -database="postgres://${DB_DSN}" up

## migrations/down: apply all down database migrations
.PHONY: migrations/down
migrations/down:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./assets/migrations -database="postgres://${DB_DSN}" down

## migrations/goto version=$1: migrate to a specific version number
.PHONY: migrations/goto
migrations/goto:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./assets/migrations -database="postgres://${DB_DSN}" goto ${version}

## migrations/force version=$1: force database migration
.PHONY: migrations/force
migrations/force:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./assets/migrations -database="postgres://${DB_DSN}" force ${version}

## migrations/version: print the current in-use migration version
.PHONY: migrations/version
migrations/version:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./assets/migrations -database="postgres://${DB_DSN}" version
