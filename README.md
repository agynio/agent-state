# Agent State Service

Go implementation of `AgentStateService` from [`agynio/api`](https://github.com/agynio/api).

## Prerequisites

- Go 1.25+
- Docker (the e2e tests start Postgres via docker-compose)
- [Buf CLI](https://buf.build/docs/installation) for protobuf code generation

## Getting started

```bash
# Install dependencies
go mod tidy

# Generate protobuf stubs
buf generate buf.build/agynio/api --path agynio/api/agent_state/v1

# Start Postgres locally (listens on localhost:55432)
docker compose up -d

# Apply migrations and run the gRPC server
DATABASE_URL="postgres://agentstate:agentstate@localhost:55432/agentstate?sslmode=disable" \
  go run ./cmd/agent-state-service
```

## Testing

End-to-end coverage is provided by Go tests. The suite automatically ensures a
docker-compose binary is available (downloading one if required).

```bash
go test ./...
```

## Continuous Integration

GitHub Actions run `buf generate`, `go build`, and the full test suite (including
docker-backed e2e tests) on every push and pull request.
