# Implementation Plan: Serverless Worker API

## Overview

Implement a lightweight Go HTTP service that executes containerized workloads via Docker/Podman. The implementation follows a bottom-up dependency order: foundational packages (config, runtime) first, then core logic (validator, executor, stream, health), then the HTTP layer (api), and finally integration wiring and tests.

## Tasks

- [x] 1. Set up project structure and core types
  - [x] 1.1 Initialize Go module and directory structure
    - Create `go.mod` with module path and Go 1.22+ requirement
    - Add dependencies: `github.com/docker/docker/client`, `github.com/flyingmutant/rapid`
    - Create directories: `cmd/worker/`, `pkg/api/`, `pkg/validator/`, `pkg/executor/`, `pkg/runtime/`, `pkg/stream/`, `pkg/health/`, `pkg/config/`
    - Create `cmd/worker/main.go` with a placeholder `main()` that prints startup
    - _Requirements: 10.4_

  - [x] 1.2 Define shared data model types
    - Create `pkg/executor/types.go` with `JobRequest`, `Secret`, `RegistryAuth`, `ExecutionResult` structs with JSON tags
    - Create `pkg/runtime/types.go` with `ContainerConfig`, `Mount`, `RegistryAuth` structs
    - Create `pkg/stream/types.go` with `LogLine` struct
    - Create `pkg/health/types.go` with `HealthStatus`, `ReadinessStatus` structs
    - Create `pkg/validator/types.go` with `ValidationError` struct
    - _Requirements: 1.2, 4.1, 5.1, 5.4, 7.1_

- [x] 2. Implement configuration package
  - [x] 2.1 Implement `pkg/config` environment variable loading
    - Create `pkg/config/config.go` with `Config` struct and `Load()` function
    - Parse `LISTEN_ADDR` (default `:8080`), `RUNTIME` ("docker"/"podman"), `RUNTIME_SOCKET` (auto-detect), `REGISTRY_WHITELIST` (comma-separated), `DEFAULT_TIMEOUT` (300s), `MAX_TIMEOUT` (3600s), `MAX_OUTPUT_BYTES` (1MB), `IMAGE_PULL_TIMEOUT` (120s)
    - Validate runtime value is "docker" or "podman", return error otherwise
    - _Requirements: 6.3, 6.4, 3.4, 3.5, 9.5_

  - [ ]* 2.2 Write unit tests for config loading
    - Test valid config with all env vars set
    - Test default values when env vars are absent
    - Test invalid runtime value returns error
    - Test whitelist parsing (empty, single, multiple entries)
    - _Requirements: 6.3, 6.4_

- [x] 3. Implement runtime adapter package
  - [x] 3.1 Define the `Runtime` interface
    - Create `pkg/runtime/runtime.go` with the `Runtime` interface: `Ping`, `PullImage`, `ImageExists`, `CreateContainer`, `StartContainer`, `WaitContainer`, `StopContainer`, `KillContainer`, `GetLogs`, `RemoveContainer`
    - All methods accept `context.Context` as first parameter
    - _Requirements: 6.6_

  - [x] 3.2 Implement `DockerRuntime`
    - Create `pkg/runtime/docker.go` implementing `Runtime` interface using `github.com/docker/docker/client`
    - Implement `Ping` using `client.Ping(ctx)`
    - Implement `PullImage` with registry auth encoding and timeout context
    - Implement `ImageExists` using `client.ImageInspectWithRaw`
    - Implement `CreateContainer` mapping `ContainerConfig` to Docker API types (HostConfig with CPU, memory, network, read-only FS, cap drop, user, mounts)
    - Implement `StartContainer`, `WaitContainer`, `StopContainer`, `KillContainer`
    - Implement `GetLogs` reading stdout/stderr from container logs
    - Implement `RemoveContainer` with force option
    - _Requirements: 6.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

  - [x] 3.3 Implement `PodmanRuntime`
    - Create `pkg/runtime/podman.go` implementing `Runtime` interface
    - Use Docker client with custom socket path (`/run/podman/podman.sock` or configured path)
    - Implement all methods identically to DockerRuntime but connecting to Podman socket
    - _Requirements: 6.2_

  - [x] 3.4 Implement runtime factory function
    - Create `pkg/runtime/factory.go` with `New(cfg *config.Config) (Runtime, error)`
    - Select DockerRuntime or PodmanRuntime based on config
    - Return error for invalid runtime value
    - _Requirements: 6.3, 6.4_

  - [ ]* 3.5 Write unit tests for runtime factory
    - Test factory returns DockerRuntime for "docker"
    - Test factory returns PodmanRuntime for "podman"
    - Test factory returns error for invalid runtime string
    - _Requirements: 6.3, 6.4_

- [ ] 4. Implement request validator package
  - [x] 4.1 Implement `pkg/validator` validation logic
    - Create `pkg/validator/validator.go` with `ValidateJobRequest(req *JobRequest, whitelist []string) []ValidationError`
    - Validate image reference: required, max 512 chars, registry component in whitelist
    - Validate command: required, max 1024 chars
    - Validate env vars: max 64, key max 256 chars, value max 4096 chars
    - Validate timeout: 1-3600 range if provided
    - Validate CPU limit: 0.1-4.0 range if provided
    - Validate memory limit: 16-8192 MB range if provided
    - Validate secrets: max 64, name matches `^[A-Z_][A-Z0-9_]{0,255}$`, value max 64 KB
    - Implement `ValidateSecretName(name string) error` and `ValidateSecrets(secrets []Secret) []ValidationError`
    - Extract registry from image reference (handle docker.io default, library prefix)
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 7.2, 7.6_

  - [ ]* 4.2 Write property tests for validator (Properties 1-4, 9)
    - **Property 1: Valid requests pass validation** — Generate random valid JobRequests within all constraints, assert zero validation errors
    - **Validates: Requirements 1.1, 1.2**
    - **Property 2: Missing required fields produce 400 errors** — Generate requests with empty/nil image or command, assert at least one error identifying the field
    - **Validates: Requirements 1.3**
    - **Property 3: Non-whitelisted registries are rejected** — Generate image refs with registries not in whitelist, assert rejection error
    - **Validates: Requirements 1.4**
    - **Property 4: Out-of-range fields produce specific errors** — Generate requests with out-of-range timeout/CPU/memory, assert error naming the field
    - **Validates: Requirements 1.5**
    - **Property 9: Secret validation rejects invalid names and exceeding limits** — Generate invalid secret names and oversized payloads, assert validation errors
    - **Validates: Requirements 7.2, 7.6**

- [x] 5. Checkpoint - Ensure foundational packages compile and tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 6. Implement log streamer package
  - [x] 6.1 Implement `pkg/stream` log buffer and subscription
    - Create `pkg/stream/buffer.go` with `LogBuffer` struct
    - Implement `Write(stream string, data string)` with mutex-protected append and notify
    - Implement `Close()` to mark stream ended and signal subscribers
    - Implement `Subscribe(ctx context.Context) <-chan LogLine` returning a channel that receives lines from current position, blocks on notify, and closes on context cancel or buffer close
    - _Requirements: 8.1, 8.2_

  - [ ]* 6.2 Write unit tests for log buffer
    - Test Write appends lines and notifies subscribers
    - Test Subscribe receives existing and new lines
    - Test Close signals end-of-stream to subscribers
    - Test context cancellation stops subscription
    - _Requirements: 8.1, 8.3_

- [ ] 7. Implement job executor package
  - [x] 7.1 Implement container config builder
    - Create `pkg/executor/config_builder.go` with `BuildContainerConfig(req *JobRequest, cfg *config.Config) *ContainerConfig`
    - Map CPU limit (fractional cores), memory limit (MB → bytes), network access (negated → NetworkOff)
    - Set security defaults: User "1000:1000", ReadOnlyFS true, CapDrop ["ALL"]
    - Map environment variables from request
    - Map secrets: env-var secrets as Env entries, file-based secrets as read-only Mounts
    - Set command from request (shell parse or pass-through)
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 7.1, 7.3_

  - [ ]* 7.2 Write property tests for config builder (Properties 5, 6, 8)
    - **Property 5: Container config enforces security invariants and maps resources correctly** — Generate valid requests, assert UID ≥ 1000, ReadOnlyFS true, CapDrop ALL, CPU/memory/network mapping correct
    - **Validates: Requirements 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8**
    - **Property 6: Effective timeout computation** — Generate optional timeouts including edge cases, assert correct default/clamping behavior
    - **Validates: Requirements 3.4, 3.5**
    - **Property 8: Secret injection produces correct container config** — Generate secrets with/without file_path, assert env vars and mounts match
    - **Validates: Requirements 7.1, 7.3**

  - [x] 7.3 Implement output capture and truncation
    - Create `pkg/executor/output.go` with `CaptureOutput(stdout, stderr []byte, maxBytes int64) (outStr, errStr string, outTruncated, errTruncated bool)`
    - If length ≤ maxBytes, return full content with truncated=false
    - If length > maxBytes, return last maxBytes (tail) with truncated=true
    - _Requirements: 4.2, 4.3_

  - [ ]* 7.4 Write property test for output truncation (Property 7)
    - **Property 7: Output tail truncation** — Generate random byte streams 0-5 MB, assert tail semantics and truncation flag correctness
    - **Validates: Requirements 4.2, 4.3**

  - [x] 7.5 Implement secret redaction
    - Create `pkg/executor/redaction.go` with `RedactSecrets(output string, secrets []Secret, placeholder string) string`
    - Replace all occurrences of any secret value in output with the placeholder
    - Ensure no secret value remains as substring after redaction
    - _Requirements: 7.4_

  - [ ]* 7.6 Write property test for secret redaction (Property 10)
    - **Property 10: Secret redaction removes all secret values from output** — Generate random outputs with embedded secret values, assert none remain after redaction
    - **Validates: Requirements 7.4**

  - [x] 7.7 Implement the `Executor` struct and `Submit` method
    - Create `pkg/executor/executor.go` with `Executor` struct (runtime, mutex, current job)
    - Implement `Submit(ctx context.Context, req *JobRequest) (string, error)`:
      - Check readiness (mutex), reject with error if busy
      - Generate job ID (UUID)
      - Create RunningJob, set as current
      - Launch goroutine: pull image → create container → start → attach log stream → wait → collect output → truncate → redact → build result → cleanup (defer remove container) → clear current job
      - Handle timeout: context with deadline, SIGTERM → 10s → SIGKILL
      - Handle image pull with separate timeout (120s)
      - Return job ID immediately (async execution)
    - Implement `GetResult(id string) (*ExecutionResult, bool)`
    - Implement `IsReady() bool`
    - Implement `StreamLogs(ctx context.Context, id string, ch chan<- LogLine) error`
    - _Requirements: 1.1, 2.1, 2.9, 3.1, 3.2, 3.3, 4.1, 4.4, 7.5, 8.3, 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ] 8. Implement health checker package
  - [x] 8.1 Implement `pkg/health` checker
    - Create `pkg/health/checker.go` with `Checker` struct (runtime, timeout)
    - Implement `Check(ctx context.Context) HealthStatus`: ping runtime with 3s timeout, return healthy/unhealthy
    - Implement `IsReady(executor *Executor) ReadinessStatus`: check executor.IsReady(), return ready/busy with job ID
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [ ]* 8.2 Write unit tests for health checker
    - Test healthy when runtime ping succeeds
    - Test unhealthy when runtime ping fails or times out
    - Test ready when no job executing
    - Test busy when job executing (with job ID)
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 9. Checkpoint - Ensure core packages compile and tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Implement HTTP API layer
  - [x] 10.1 Implement HTTP server and routing
    - Create `pkg/api/server.go` with `Server` struct and `NewServer(executor, health, config)` constructor
    - Register routes using `http.ServeMux`: `POST /jobs`, `GET /jobs/{id}`, `GET /jobs/{id}/logs`, `GET /health`, `GET /ready`
    - Implement `Start(ctx context.Context) error` and `Shutdown(ctx context.Context) error` with graceful shutdown
    - _Requirements: 1.1, 5.1, 5.4, 8.1_

  - [x] 10.2 Implement job submission handler (`POST /jobs`)
    - Create `pkg/api/handlers.go` with `handleSubmitJob` method
    - Parse JSON body, return 400 on malformed JSON (requirement 1.6)
    - Call validator, return 400 with validation errors (requirement 1.3, 1.5) or 403 for registry rejection (requirement 1.4)
    - Check executor readiness, return 409 Conflict if busy
    - Call executor.Submit, return 202 Accepted with `{"job_id": "..."}` response
    - _Requirements: 1.1, 1.3, 1.4, 1.5, 1.6_

  - [x] 10.3 Implement job result handler (`GET /jobs/{id}`)
    - Implement `handleGetResult` method
    - Call executor.GetResult, return 404 if job not found
    - Return 200 with ExecutionResult JSON
    - _Requirements: 4.1_

  - [x] 10.4 Implement log streaming handler (`GET /jobs/{id}/logs`)
    - Implement `handleStreamLogs` method with SSE (Server-Sent Events)
    - Set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
    - Subscribe to log buffer, write `event: log\ndata: {...}\n\n` for each LogLine
    - On job completion, write `event: end\ndata: {"status":"...","exit_code":...}\n\n`
    - Handle client disconnect gracefully (context cancel)
    - Use `http.Flusher` to flush after each event
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

  - [x] 10.5 Implement health and readiness handlers
    - Implement `handleHealth` calling health.Check, return 200 with HealthStatus JSON
    - Implement `handleReady` calling health.IsReady, return 200 with ReadinessStatus JSON (status "ready" or "busy")
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [ ]* 10.6 Write unit tests for HTTP handlers
    - Test POST /jobs with valid request returns 202
    - Test POST /jobs with malformed JSON returns 400
    - Test POST /jobs with missing fields returns 400 with field names
    - Test POST /jobs with non-whitelisted registry returns 403
    - Test POST /jobs when busy returns 409
    - Test GET /jobs/{id} with valid ID returns result
    - Test GET /jobs/{id} with unknown ID returns 404
    - Test GET /health returns healthy/unhealthy status
    - Test GET /ready returns ready/busy status
    - Test SSE event formatting for log stream
    - _Requirements: 1.1, 1.3, 1.4, 1.5, 1.6, 4.1, 5.1, 5.4, 8.1_

- [ ] 11. Wire main entry point
  - [x] 11.1 Implement `cmd/worker/main.go`
    - Load config via `config.Load()`, exit on error
    - Create runtime via factory, exit on error
    - Verify runtime connectivity (Ping), exit with message if unreachable (requirement 6.5)
    - Create executor, health checker, and server
    - Set up structured logging with `log/slog` (JSON handler, filter secret values)
    - Start HTTP server, handle OS signals (SIGTERM, SIGINT) for graceful shutdown
    - _Requirements: 6.4, 6.5, 10.3_

- [~] 12. Checkpoint - Ensure full application compiles and unit/property tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Integration tests and container image
  - [ ]* 13.1 Write integration tests for full job lifecycle
    - Create `test/integration/job_test.go` (build tag `integration`)
    - Test happy path: submit `alpine` job with `echo hello`, poll result, verify stdout/status
    - Test failed exit: run `sh -c "exit 42"`, verify exit_code=42, status="failed"
    - Test timeout: run `sleep 600` with timeout=2s, verify status="timeout"
    - Test network isolation: run `ping -c 1 8.8.8.8` with network_access=false, verify failure
    - Test concurrent submission rejection: submit while busy, verify 409
    - Test log streaming: enable stream_logs, connect SSE, verify labeled lines and end event
    - Test image not found: reference non-existent image, verify error result
    - _Requirements: 1.1, 2.8, 3.2, 3.3, 4.1, 5.5, 8.1, 8.2, 9.2_

  - [~] 13.2 Create Dockerfile for worker image
    - Create multi-stage `Dockerfile`: builder stage (golang:1.22-alpine) compiles static binary with CGO_DISABLED=1
    - Final stage uses `gcr.io/distroless/static-debian12` or `scratch`
    - Copy binary, set non-root USER, expose port 8080
    - Target compressed image size ≤ 50 MB
    - _Requirements: 10.4_

  - [ ]* 13.3 Write smoke tests for resource constraints
    - Create `test/smoke/resource_test.go` (build tag `smoke`)
    - Test startup time: build and start container, measure time to first healthy response ≤ 2s
    - Test idle memory: after startup, verify RSS ≤ 32 MB
    - Test image size: verify compressed Docker image ≤ 50 MB
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [~] 14. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Integration tests require a running Docker daemon (use Docker-in-Docker in CI)
- The implementation uses Go 1.22+ `net/http` routing with `{id}` path parameters (ServeMux pattern matching)
- All property tests use `github.com/flyingmutant/rapid` with minimum 100 iterations

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "2.1"] },
    { "id": 2, "tasks": ["2.2", "3.1"] },
    { "id": 3, "tasks": ["3.2", "3.3", "3.4"] },
    { "id": 4, "tasks": ["3.5", "4.1"] },
    { "id": 5, "tasks": ["4.2", "6.1"] },
    { "id": 6, "tasks": ["6.2", "7.1"] },
    { "id": 7, "tasks": ["7.2", "7.3", "7.5"] },
    { "id": 8, "tasks": ["7.4", "7.6", "7.7"] },
    { "id": 9, "tasks": ["8.1"] },
    { "id": 10, "tasks": ["8.2", "10.1"] },
    { "id": 11, "tasks": ["10.2", "10.3", "10.4", "10.5"] },
    { "id": 12, "tasks": ["10.6", "11.1"] },
    { "id": 13, "tasks": ["13.1", "13.2"] },
    { "id": 14, "tasks": ["13.3"] }
  ]
}
```
