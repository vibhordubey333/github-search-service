# GitHub Search Service

A simple gRPC service to search through GitHub repositories.

![Dev Testing](./readme-images/image.png)

## Getting Started

### Prerequisites
- Docker
- `brew install protobuf`
- [grpcurl](https://github.com/fullstorydev/grpcurl) (for testing the API)

### Configuration
To run the service, you'll need a GitHub Personal Access Token. Create a `.env` file in the root directory:

```env
GITHUB_TOKEN=your_github_token_here
# Log level: debug, info, warn, error, dpanic, panic, fatal
LOG_LEVEL=info

GITHUB_BASE_URL=https://api.github.com

REQUEST_TIMEOUT=10s

MAX_CONCURRENCY=5
```

## Running the App

### Using Docker
The easiest way to get it running is using Docker:

```bash
make docker-run
```

### Running Locally
If you want to run it directly on your machine, make sure you have Go installed and your `.env` file or environment variables set up, then run:

```bash
make run
```

## Testing the API

Once the service is up and running, you can test it using `grpcurl`:

```bash
grpcurl -plaintext \
  -import-path . \
  -proto api/proto/v1/search.proto \
  -d '{"search_term": "filename:main.go language:go"}' \
  localhost:50051 \
  github.search.v1.GithubSearchService/Search
```
