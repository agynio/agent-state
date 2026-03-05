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

## Releases

Container images and Helm charts are published automatically when a semantic
version tag (`vX.Y.Z`) is pushed. To cut a release:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The release workflow builds and publishes the multi-architecture image to
`ghcr.io/agynio/agent-state` with tags `v1.2.3` and `latest`, and packages the
Helm chart to `oci://ghcr.io/agynio/charts` as `agent-state` version `1.2.3`.

## Helm chart usage

Install (or upgrade) the chart from GHCR. Provide a database URL either via an
existing secret (recommended) or inline value:

```bash
helm upgrade --install agent-state oci://ghcr.io/agynio/charts/agent-state \
  --version 1.2.3 \
  --namespace agent-state \
  --create-namespace \
  --set env.existingSecret=agent-state-db \
  --set env.secretKey=database-url
```

If you prefer to supply the connection string directly:

```bash
helm upgrade --install agent-state oci://ghcr.io/agynio/charts/agent-state \
  --version 1.2.3 \
  --set env.databaseUrl="postgres://user:pass@host:port/db?sslmode=verify-full"
```

Review `charts/agent-state/values.yaml` for all available configuration
options, including resource requests, replica counts, and autoscaling.
