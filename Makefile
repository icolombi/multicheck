.DEFAULT_GOAL := help
help:
	@echo "Available commands:"
	@echo "Backend:"
	@echo "  make build          - Build the multicheck binary"
	@echo "  make run            - Run the multicheck binary and format output with jq"
	@echo "  make test           - Run tests for the multicheck package (verbose with colors)"
	@echo "  make test-quiet     - Run tests with summary only (minimal output)"
	@echo "  make test-cache-key - Run cache key integration tests"
	@echo ""
	@echo "Frontend:"
	@echo "  make install-frontend - Install frontend dependencies"
	@echo "  make run-frontend     - Start frontend development server"
	@echo "  make build-frontend   - Build frontend for production"
	@echo ""
	@echo "Docker/Podman:"
	@echo "  make build-compose  - Build and start the podman-compose services (backend + redis)"
	@echo "  make start          - Start the podman-compose services"
	@echo "  make stop           - Stop the podman-compose services"
	@echo "  make logs           - Follow logs from running containers"
build:
	@go build -o ./bin/multicheck
	@strip ./bin/multicheck
run:
	@./bin/multicheck | jq

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
	
build-compose:
	@export PODMAN_IGNORE_CGROUPSV1_WARNING=1 && \
	podman-compose up --build --force-recreate
start:
	@podman-compose up
stop:
	@podman-compose down
logs:
	@podman-compose logs -f

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

# Integration test for cache key functionality
test-cache-key:
	@echo "Running cache key integration tests..."
	@./test-cache-key.sh
