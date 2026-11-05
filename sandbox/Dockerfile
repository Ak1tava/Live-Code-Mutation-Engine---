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
# Warm cache is immutable in the image. The entrypoint copies it into bounded
# /tmp at runtime because Go also needs to write package results to GOCACHE.
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
    && printf 'package warm\n\nimport (\n\t_ "fmt"\n\t_ "strings"\n\t_ "strconv"\n\t_ "sort"\n\t_ "os"\n\t_ "bufio"\n\t_ "unicode"\n\t_ "errors"\n\t_ "math"\n\t_ "bytes"\n\t_ "io"\n\t_ "time"\n\t_ "regexp"\n\t_ "testing"\n\t_ "reflect"\n\t_ "encoding/json"\n)\n' > warm_test.go \
    && go test -c -o /dev/null . \
    && cd / && rm -rf /tmp/warm
ENV GOCACHE=/tmp/gocache
COPY --chmod=0555 sandbox/runner-entrypoint.sh /usr/local/bin/lcme-entrypoint
# A Windows checkout may convert the shell script to CRLF.
RUN sed -i 's/\r$//' /usr/local/bin/lcme-entrypoint
ENTRYPOINT ["/bin/sh", "/usr/local/bin/lcme-entrypoint"]
