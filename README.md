# opcp-serverless-brik

A serverless worker service that executes container-based jobs on demand. It exposes an HTTP API to submit jobs, retrieve results, and stream logs in real time via Server-Sent Events (SSE).

## Overview

This worker accepts job requests specifying a container image and command, runs them in an isolated container (Docker or Podman), and returns structured output including stdout, stderr, exit code, and execution duration. It enforces security constraints such as registry whitelisting, resource limits, read-only filesystems, and secret redaction.

Key characteristics:
- Single-job-at-a-time execution model (returns 409 if busy)
- Real-time log streaming via SSE
- Configurable timeouts and output size limits
- Support for both Docker and Podman runtimes
- Secret injection via environment variables or file mounts
- Registry whitelist enforcement
- Non-root container execution by default

## Architecture

```
cmd/worker/         → Application entry point
pkg/api/            → HTTP server & route handlers
pkg/config/         → Environment-based configuration
pkg/executor/       → Job submission, lifecycle, output management
pkg/health/         → Health and readiness probes
pkg/runtime/        → Container runtime abstraction (Docker/Podman)
pkg/stream/         → Log buffering for SSE streaming
pkg/validator/      → Request validation
```

## API Endpoints

| Method | Path             | Description                          |
|--------|------------------|--------------------------------------|
| POST   | `/jobs`          | Submit a new job for execution       |
| GET    | `/jobs/{id}`     | Get the result of a completed job    |
| GET    | `/jobs/{id}/logs`| Stream job logs via SSE              |
| GET    | `/health`        | Health check (runtime connectivity)  |
| GET    | `/ready`         | Readiness check (worker availability)|

### Submit a Job

```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "image": "alpine:latest",
    "command": "echo hello world",
    "timeout_seconds": 30,
    "network_access": false,
    "stream_logs": true
  }'
```

Response (202 Accepted):
```json
{"job_id": "abc123"}
```

### Get Job Result

```bash
curl http://localhost:8080/jobs/abc123
```

Response:
```json
{
  "job_id": "abc123",
  "status": "success",
  "exit_code": 0,
  "stdout": "hello world\n",
  "stderr": "",
  "stdout_truncated": false,
  "stderr_truncated": false,
  "duration_ms": 1234
}
```

### Stream Logs (SSE)

```bash
curl -N http://localhost:8080/jobs/abc123/logs
```

## Configuration

All configuration is done through environment variables:

| Variable             | Default                    | Description                                      |
|----------------------|----------------------------|--------------------------------------------------|
| `LISTEN_ADDR`        | `:8080`                    | Address and port to listen on                    |
| `RUNTIME`            | `docker`                   | Container runtime (`docker` or `podman`)         |
| `RUNTIME_SOCKET`     | Auto-detected              | Path to the runtime socket                       |
| `REGISTRY_WHITELIST` | _(empty — all allowed)_    | Comma-separated list of allowed registries       |
| `DEFAULT_TIMEOUT`    | `300` (seconds)            | Default job timeout                              |
| `MAX_TIMEOUT`        | `3600` (seconds)           | Maximum allowed job timeout                      |
| `MAX_OUTPUT_BYTES`   | `1048576` (1 MB)           | Maximum captured stdout/stderr size per stream   |
| `IMAGE_PULL_TIMEOUT` | `120` (seconds)            | Timeout for pulling container images             |

## Running

### Prerequisites

- Go 1.25+
- Docker or Podman installed and running

### Run Locally

```bash
go build -o worker ./cmd/worker/
./worker
```

The server starts on `:8080` by default.

### Run with Docker Compose

```bash
docker compose up --build
```

This starts the worker with Docker socket access and an Nginx reverse proxy with TLS on port 5001.

You can customize ports via environment variables:

```bash
HTTP_PORT=9090 HTTPS_PORT=9443 docker compose up --build
```

### Build the Docker Image

```bash
docker build -t opcp-serverless-brik .
```

### Run the Container Directly

```bash
docker run -d \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock:rw \
  -e RUNTIME=docker \
  opcp-serverless-brik
```

## Testing

```bash
go test ./...
```

## Project Structure

The application follows a clean separation of concerns:

- **config** — Loads and validates environment-based configuration
- **runtime** — Abstracts Docker/Podman operations (create, start, wait, remove, logs)
- **executor** — Orchestrates job lifecycle: image pull → container create → run → collect output
- **stream** — Buffers container output for real-time SSE delivery
- **validator** — Validates incoming job requests (required fields, whitelist, limits)
- **health** — Provides liveness and readiness probes for orchestrator integration
- **api** — HTTP server with routing, error handling, and SSE streaming

## License

See repository for license information.
