# Agent State Service

Go implementation of `AgentStateService` from [`agynio/api`](https://github.com/agynio/api).

## Prerequisites

- Go 1.25+
- Docker (the e2e tests start Postgres via docker run)
- [Buf CLI](https://buf.build/docs/installation) for protobuf code generation

## Getting started

```bash
# Install dependencies
go mod tidy

# Generate protobuf stubs
buf generate buf.build/agynio/api --path agynio/api/agent_state/v1

# Start Postgres locally (listens on localhost:55432)
docker run --name agent-state-postgres \
  -e POSTGRES_USER=agentstate \
  -e POSTGRES_PASSWORD=agentstate \
  -e POSTGRES_DB=agentstate \
  -p 55432:5432 \
  -d public.ecr.aws/docker/library/postgres:16-alpine

# Apply migrations and run the gRPC server
DATABASE_URL="postgres://agentstate:agentstate@localhost:55432/agentstate?sslmode=disable" \
  go run ./cmd/agent-state-service

# Stop Postgres when you're done
docker rm -f agent-state-postgres
```

## Testing

End-to-end coverage is provided by Go tests. The suite starts/stops Postgres
via Docker CLI commands, so the Docker daemon must be available.

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

Install (or upgrade) the chart from GHCR. You must provide a database URL via
an existing secret (recommended) or by supplying it directly. Exactly one of
`database.existingSecret.name` or `database.url` must be set:

```bash
helm upgrade --install agent-state oci://ghcr.io/agynio/charts/agent-state \
  --version 1.2.3 \
  --namespace agent-state \
  --create-namespace \
  --set database.existingSecret.name=agent-state-db \
  --set database.existingSecret.key=database-url
```

If you prefer to supply the connection string directly:

```bash
helm upgrade --install agent-state oci://ghcr.io/agynio/charts/agent-state \
  --version 1.2.3 \
  --set database.url="postgres://user:pass@host:port/db?sslmode=verify-full"
```

Review `charts/agent-state/values.yaml` for all available configuration
options, including resource requests, replica counts, and autoscaling.

When running `helm lint` or `helm template` locally, supply one of the
database options (for example, `--set database.url=dummy`) so rendering
passes validation.
