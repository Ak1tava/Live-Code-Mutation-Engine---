ARG GO_VERSION=1.27.3

FROM golang:${GO_VERSION}-alpine AS build
ENV CGO_ENABLED=0
WORKDIR /src
# Only what cmd/runner needs: never COPY . . (the repo root may hold .env with
# secrets, work/ with untrusted code and web/node_modules).
COPY go.mod go.sum* ./
COPY cmd/runner ./cmd/runner
COPY internal/sandbox ./internal/sandbox
RUN go build -o /out/lcme-runner ./cmd/runner

FROM golang:${GO_VERSION}-alpine
COPY --from=build /out/lcme-runner /usr/local/bin/lcme-runner
# Same environment as at runtime, so the warmed cache keys match.
ENV GOCACHE=/opt/gocache \
    GOPATH=/tmp/go \
    HOME=/tmp \
    GOTOOLCHAIN=local \
    GOPROXY=off \
    GOFLAGS=-mod=readonly \
    CGO_ENABLED=0
# Warm the build cache with the standard packages student code is likely to use.
RUN mkdir /tmp/warm && cd /tmp/warm \
    && printf 'module warm\n\ngo 1.22\n' > go.mod \
    && printf 'package warm\n' > warm.go \
    && printf 'package warm\n\nimport (\n\t_ "fmt"\n\t_ "strings"\n\t_ "strconv"\n\t_ "sort"\n\t_ "os"\n\t_ "bufio"\n\t_ "unicode"\n\t_ "errors"\n\t_ "math"\n\t_ "bytes"\n\t_ "io"\n\t_ "time"\n\t_ "regexp"\n\t_ "testing"\n\t_ "reflect"\n)\n' > warm_test.go \
    && go test -c -o /dev/null . \
    && cd / && rm -rf /tmp/warm \
    && chown -R 65534:65534 /opt/gocache
