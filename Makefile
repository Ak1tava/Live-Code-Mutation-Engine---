.PHONY: image web build run test test-docker dev demo

GO_VERSION ?= 1.27.3

image:
	docker build -f sandbox/Dockerfile --build-arg GO_VERSION=$(GO_VERSION) -t lcme-runner:latest .
	docker build -f sandbox/import.Dockerfile -t lcme-importer:latest .

web:
	cd web && npm ci && npm run build && touch dist/.gitkeep

build: web
	go build -o bin/lcme ./cmd/server
	go build -o bin/lcme-runner ./cmd/runner

run:
	./scripts/run.sh

test:
	go test ./...

test-docker:
	LCME_DOCKER=1 go test ./internal/sandbox/... ./internal/project/...

dev: build
	./scripts/dev.sh

# Trusted fixture only. Keep Docker as the production default.
demo: build
	LCME_RUNNER=local LCME_ADDR=127.0.0.1:8080 LCME_PUBLIC_URL=http://127.0.0.1:8080 ./bin/lcme
