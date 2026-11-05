.PHONY: image web build run test test-docker dev

image:
	docker build -f sandbox/Dockerfile --build-arg GO_VERSION=1.27.3 -t lcme-runner:latest .

web:
	cd web && npm install && npm run build

build:
	go build -o bin/lcme ./cmd/server
	go build -o bin/lcme-runner ./cmd/runner

run:
	./bin/lcme

test:
	go test ./...

test-docker:
	LCME_DOCKER=1 go test ./internal/sandbox/...

dev:
	go run ./cmd/server & cd web && npx vite dev; wait
