.DEFAULT_GOAL := help

.PHONY: help build run test test-quiet test-race test-integration test-cache-key \
	build-compose start stop logs \
	install-frontend run-frontend build-frontend preview-frontend

help:
	@echo "Available commands:"
	@echo "Backend:"
	@echo "  make build            - Build the multicheck binary"
	@echo "  make run              - Run the multicheck binary and format output with jq"
	@echo "  make test             - Run the unit tests (no Redis or network needed)"
	@echo "  make test-quiet       - Run the unit tests with summary only (minimal output)"
	@echo "  make test-race        - Run the unit tests under the race detector"
	@echo "  make test-integration - Run the integration tests (needs Redis and live DNS)"
	@echo "  make test-cache-key   - Run cache key integration tests"
	@echo ""
	@echo "Frontend:"
	@echo "  make install-frontend - Install frontend dependencies"
	@echo "  make run-frontend     - Start frontend development server"
	@echo "  make build-frontend   - Build frontend for production"
	@echo "  make preview-frontend - Preview production build locally"
	@echo ""
	@echo "Docker/docker:"
	@echo "  make build-compose  - Build and start the docker compose services (backend + redis)"	
	@echo "  make start          - Start the docker compose services"
	@echo "  make stop           - Stop the docker compose services"
	@echo "  make logs           - Follow logs from running containers"
build:
	@go build -ldflags "-s -w" -o ./bin/multicheck
run:
	@./bin/multicheck | jq

# Unit tests only: they run against the hermetic environment installed by TestMain,
# so they need neither a Redis server nor DNS access and are safe to run in CI.
test:
	@go test -v 2>&1 | sed \
		-e 's/^=== RUN/\x1b[1;34m▶\x1b[0m \x1b[1mRUN\x1b[0m/g' \
		-e 's/^--- PASS: \(.*\) (.*/\x1b[1;32m✓\x1b[0m \x1b[32mPASS:\x1b[0m \1/g' \
		-e 's/^--- FAIL: \(.*\) (.*/\x1b[1;31m✗\x1b[0m \x1b[31mFAIL:\x1b[0m \1/g' \
		-e 's/^PASS$$/\x1b[1;42m\x1b[97m PASS \x1b[0m All tests passed successfully!/g' \
		-e 's/^FAIL$$/\x1b[1;41m\x1b[97m FAIL \x1b[0m Some tests failed!/g' \
		-e 's/^ok  \(.*\)/\x1b[1;32m✓ OK\x1b[0m  \1/g' \
		-e 's/^FAIL\t\(.*\)/\x1b[1;31m✗ FAIL\x1b[0m\t\1/g' \
		-e 's/\t\(--- FAIL:\)/\t\x1b[1;31m\1\x1b[0m/g' \
		-e 's/Error Trace:/\x1b[1;31mError Trace:\x1b[0m/g' \
		-e 's/Error:/\x1b[1;31mError:\x1b[0m/g' \
		-e 's/Test:/\x1b[1;33mTest:\x1b[0m/g'

test-quiet:
	@go test -v 2>&1 | grep -E "^(RUN|PASS|FAIL|ok|===|---)" | sed \
		-e 's/^=== RUN/\x1b[1;34m  ▶\x1b[0m Running/g' \
		-e 's/^--- PASS: \(.*\) (.*/\x1b[1;32m  ✓\x1b[0m \1/g' \
		-e 's/^--- FAIL: \(.*\) (.*/\x1b[1;31m  ✗\x1b[0m \1/g' \
		-e 's/^PASS$$/\n\x1b[1;42m\x1b[97m SUCCESS \x1b[0m All tests passed!\n/g' \
		-e 's/^FAIL$$/\n\x1b[1;41m\x1b[97m FAILURE \x1b[0m Some tests failed!\n/g' \
		-e 's/^ok  \(.*\)/\n\x1b[1;32m✓ OK\x1b[0m  \1\n/g' \
		-e 's/^FAIL\t\(.*\)/\n\x1b[1;31m✗ FAIL\x1b[0m  \1\n/g'

test-race:
	@go test -race ./...

# Needs a reachable Redis instance and live DNS access to third-party DNSBL
# servers. Individual tests skip themselves when Redis is unavailable.
test-integration:
	@go test -tags integration -v ./...

build-compose:
	@export docker_IGNORE_CGROUPSV1_WARNING=1 && \
	docker compose up --build # --force-recreate

start:
	@docker compose up
stop:
	@docker compose down
logs:
	@docker compose logs -f

# Frontend targets
install-frontend:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install

run-frontend:
	@echo "Starting frontend development server..."
	@cd frontend && npm run dev

build-frontend:
	@echo "Building frontend for production..."
	@cd frontend && npm run build

preview-frontend:
	@echo "Starting frontend preview server (production build)..."
	@cd frontend && npm run preview

# Integration test for cache key functionality
test-cache-key:
	@echo "Running cache key integration tests..."
	@./test-cache-key.sh
