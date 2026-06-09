
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# -ldflags="-w -s": strip debug infos and symbol table -> make binary smaller
# CGO_ENABLED=0: pure go binary, no c deps -> no glibc needed
# GOOS=linux: make it explicit (very importent when build on macos) also using cache mounts for go build and go mod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

### Stage 2
FROM gcr.io/distroless/static-debian12

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/server /server

# run as non-root user
USER 65532:65532

EXPOSE 50051 9090

ENTRYPOINT ["/server"]