.DEFAULT_GOAL := help
help:
	@echo "Available commands:"
	@echo "  make build    - Build the multicheck binary"
	@echo "  make run      - Run the multicheck binary and format output with jq"
	@echo "  make test     - Run tests for the multicheck package"
	@echo "  make build-compose - Build and start the podman-compose services"
	@echo "  make start    - Start the podman-compose services"
	@echo "  make stop     - Stop the podman-compose services"
	@echo "  make logs     - Follow logs from running containers"
build:
	@go build -o ./bin/multicheck
	@strip ./bin/multicheck
run:
	@./bin/multicheck | jq

test:
	@go test -v
build-compose:
	@export PODMAN_IGNORE_CGROUPSV1_WARNING=1 && \
	podman-compose up --build --force-recreate
start:
	@podman-compose up
stop:
	@podman-compose down
