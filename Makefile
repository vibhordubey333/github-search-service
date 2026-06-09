.PHONY: all proto deps build test test-unit test-integration lint clean run docker-build docker-run

PROTOC_GEN_GO_VERSION     := v1.34.2
PROTOC_GEN_GO_GRPC_VERSION := v1.4.0

all: deps proto build


# Pre-requisites: brew install protobuf
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

# ─── Generate proto files ─────────────────────────────────────────────────────
# Generates: api/proto/v1/search.pb.go  and  api/proto/v1/search_grpc.pb.go
proto:
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		api/proto/v1/search.proto

deps:
	go mod download
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/server ./cmd/server

run: build
	./bin/server

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
	rm -f api/proto/v1/search.pb.go api/proto/v1/search_grpc.pb.go

docker-build:
	docker build -t github-search-service -f deployments/Dockerfile .

docker-run: docker-build
	docker run -p 50051:50051 -p 9090:9090 --env-file .env github-search-service
