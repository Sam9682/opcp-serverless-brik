# Requirements Document

## Introduction

The Serverless Worker API is a lightweight Docker-based web API application that acts as the worker/executor component in a serverless execution platform. It is launched by the server side to execute containerized workloads (source code or Docker containers fetched from git or Docker registries). The worker receives execution instructions, runs the specified container with appropriate resource constraints and security isolation, and reports results (exit code, stdout, stderr) back to the caller.

This component is intentionally minimal — it exposes a small HTTP API surface, delegates execution to a container runtime (Docker or Podman), enforces security and resource limits, and returns structured results.

## Glossary

- **Worker_API**: The lightweight HTTP server that receives job execution requests, orchestrates container execution, and returns results.
- **Job**: A unit of work containing the container image, command, environment variables, resource limits, and timeout to execute.
- **Container_Runtime**: The underlying engine (Docker or Podman) used to execute containers. Abstracted behind a unified interface.
- **Server_Side**: The upstream orchestration system (out of scope) that creates jobs and launches Worker_API instances in a pool.
- **Execution_Result**: The structured output of a completed job, including exit code, stdout, stderr, and execution metadata.
- **Registry_Whitelist**: A configurable list of approved container registries from which images may be pulled.
- **Secret_Injection**: The mechanism by which secrets are provided to executed containers via environment variables or mounted files.

## Requirements

### Requirement 1: Accept Job Execution Requests

**User Story:** As the Server_Side, I want to submit a job execution request to the Worker_API, so that a containerized workload is executed and results are returned.

#### Acceptance Criteria

1. WHEN a valid job execution request is received, THE Worker_API SHALL respond with a 202 Accepted status and begin job execution.
2. THE Worker_API SHALL accept job requests containing: container image reference (max 512 characters), command to execute (max 1024 characters), environment variables (max 64 variables, each key max 256 characters, each value max 4096 characters), timeout duration (1 to 3600 seconds), CPU limit (0.1 to 4.0 CPU cores), memory limit (16 MB to 8192 MB), and network access flag (boolean).
3. IF a job execution request is missing required fields (image reference or command), THEN THE Worker_API SHALL return a 400 error response with a description of the missing fields.
4. IF a job execution request specifies an image from a registry not in the Registry_Whitelist, THEN THE Worker_API SHALL reject the request with a 403 error response indicating the registry is not approved.
5. IF a job execution request contains field values outside their valid ranges (timeout, CPU limit, or memory limit), THEN THE Worker_API SHALL return a 400 error response indicating which fields are invalid and their acceptable ranges.
6. IF a job execution request body is malformed or not parseable, THEN THE Worker_API SHALL return a 400 error response indicating the request format is invalid.

### Requirement 2: Execute Containers via Container Runtime

**User Story:** As the Server_Side, I want the Worker_API to execute a container using the specified image and command, so that the workload runs in an isolated environment.

#### Acceptance Criteria

1. IF the specified container image is not already present locally, WHEN a job is accepted, THE Worker_API SHALL pull the image from the registry before starting execution.
2. WHEN the image is available, THE Worker_API SHALL create and start a container with the specified command, environment variables, and resource limits.
3. THE Worker_API SHALL execute containers as a non-root user (UID >= 1000) inside the container.
4. THE Worker_API SHALL mount the container filesystem as read-only.
5. THE Worker_API SHALL drop all Linux capabilities without exception when creating the container.
6. WHILE a job is executing, THE Worker_API SHALL enforce the specified CPU limit (expressed in fractional CPU cores, e.g., 0.5 for half a core) on the container.
7. WHILE a job is executing, THE Worker_API SHALL enforce the specified memory limit (expressed in megabytes) on the container, terminating the container process if the limit is exceeded.
8. WHERE network access is not explicitly enabled in the job request, THE Worker_API SHALL disable network access for the container.
9. IF the container fails to start (runtime error, invalid command, or image incompatibility), THEN THE Worker_API SHALL return an error in the Execution_Result with a descriptive message indicating the container start failure reason.

### Requirement 3: Enforce Execution Timeout

**User Story:** As the Server_Side, I want the Worker_API to enforce execution timeouts, so that runaway jobs do not consume resources indefinitely.

#### Acceptance Criteria

1. WHILE a job is executing, THE Worker_API SHALL monitor elapsed wall-clock execution time against the specified timeout duration.
2. IF a job exceeds the specified timeout duration, THEN THE Worker_API SHALL send SIGTERM to the container, wait up to 10 seconds for graceful shutdown, and then force-kill (SIGKILL) the container if still running.
3. WHEN a job is terminated due to timeout, THE Worker_API SHALL return an Execution_Result with status "timeout" and include any output captured before termination.
4. IF no timeout is specified in the job request, THEN THE Worker_API SHALL apply a default timeout of 300 seconds.
5. THE Worker_API SHALL enforce a maximum timeout of 3600 seconds regardless of the value specified in the job request.

### Requirement 4: Return Execution Results

**User Story:** As the Server_Side, I want to receive structured results from the Worker_API after job execution, so that I can process and store the outcome.

#### Acceptance Criteria

1. WHEN a job execution completes (success or failure), THE Worker_API SHALL return an Execution_Result containing: exit code (integer), stdout output, stderr output, execution duration, and completion status with one of the following values: "success" (container exited with code 0), "failed" (container exited with non-zero code), "timeout" (terminated due to timeout), or "error" (infrastructure failure prevented execution).
2. THE Worker_API SHALL capture up to 1 MB of stdout and 1 MB of stderr from the executed container.
3. IF stdout or stderr exceeds 1 MB, THEN THE Worker_API SHALL retain the last 1 MB of output (tail), discard earlier content, and set a boolean truncation flag to true for the affected stream in the Execution_Result.
4. WHEN a job execution completes, THE Worker_API SHALL include the wall-clock execution duration in milliseconds (integer, starting from container start to container exit or termination) in the Execution_Result.

### Requirement 5: Provide Health and Status Endpoints

**User Story:** As the Server_Side, I want to check the Worker_API health and readiness, so that I can route jobs to healthy workers and detect failures.

#### Acceptance Criteria

1. THE Worker_API SHALL expose a health endpoint that returns a status of "healthy" when the Worker_API process is running and connectivity to the Container_Runtime is confirmed, or "unhealthy" otherwise.
2. WHEN the health endpoint is called, THE Worker_API SHALL verify connectivity to the Container_Runtime within 3 seconds and report the connectivity result as part of the health status.
3. IF the Container_Runtime does not respond within 3 seconds during a health check, THEN THE Worker_API SHALL return an unhealthy status indicating Container_Runtime unavailability.
4. THE Worker_API SHALL expose a readiness endpoint that returns a status of "ready" when the worker can accept a new job, or "busy" when a job is currently executing.
5. WHILE a job is currently executing, THE Worker_API SHALL report the readiness endpoint as "busy" (not ready for new work).
6. THE Worker_API SHALL respond to health and readiness endpoint requests within 5 seconds.

### Requirement 6: Support Container Runtime Abstraction

**User Story:** As an operator, I want the Worker_API to support both Docker and Podman runtimes, so that I can deploy on infrastructure using either runtime.

#### Acceptance Criteria

1. THE Worker_API SHALL support Docker as a Container_Runtime for all container operations (image pull, container create, start, stop, and remove).
2. THE Worker_API SHALL support Podman as a Container_Runtime for all container operations (image pull, container create, start, stop, and remove).
3. THE Worker_API SHALL select the Container_Runtime based on a configuration value of either "docker" or "podman" provided at startup.
4. IF the runtime configuration value is not one of the supported values ("docker" or "podman"), THEN THE Worker_API SHALL fail to start and report an error message indicating the invalid runtime value.
5. IF the configured Container_Runtime is not reachable at startup, THEN THE Worker_API SHALL fail to start and report an error message indicating the runtime is unavailable.
6. THE Worker_API SHALL use a unified runtime interface so that job execution produces identical Execution_Result structure and content regardless of the specific Container_Runtime in use.

### Requirement 7: Inject Secrets into Containers

**User Story:** As the Server_Side, I want to pass secrets to executed containers, so that jobs can authenticate to external services without exposing credentials in the job specification.

#### Acceptance Criteria

1. WHEN a job request includes secrets, THE Worker_API SHALL inject each secret as an environment variable into the container, where the environment variable name matches the secret name and the value matches the secret value.
2. IF a job request includes a secret with a name that is not a valid environment variable identifier (uppercase letters, digits, and underscores, not starting with a digit, maximum 256 characters), THEN THE Worker_API SHALL reject the request with a 400 error response indicating the invalid secret name.
3. WHERE a job request specifies file-based secrets, THE Worker_API SHALL mount each secret value as a read-only file at the path specified in the job request inside the container.
4. THE Worker_API SHALL exclude secret values from the Execution_Result and from any Worker_API diagnostic logs, replacing occurrences with a redaction placeholder.
5. IF secret injection fails (environment variable cannot be set or file cannot be mounted at the specified path), THEN THE Worker_API SHALL abort the job and return an error in the Execution_Result indicating the secret injection failure reason without revealing the secret value.
6. THE Worker_API SHALL accept a maximum of 64 secrets per job request, with each secret value not exceeding 64 KB in size.

### Requirement 8: Stream Execution Logs

**User Story:** As the Server_Side, I want to optionally stream logs from a running job, so that users can observe execution progress in real time.

#### Acceptance Criteria

1. WHERE log streaming is enabled in the job request, THE Worker_API SHALL provide a streaming endpoint that emits stdout and stderr lines within 2 seconds of the output being produced by the container, with each emitted line labeled to indicate whether it originated from stdout or stderr.
2. WHEN the job completes, THE Worker_API SHALL close the log stream and send a structured end-of-stream message that is distinguishable from log content and indicates the final job completion status.
3. IF the log stream connection is interrupted, THEN THE Worker_API SHALL continue executing the job to completion and discard further stream output for that connection.
4. WHERE log streaming is enabled in the job request, IF the client does not connect to the streaming endpoint before the job completes, THEN THE Worker_API SHALL complete the job normally and close the stream endpoint.

### Requirement 9: Image Pull Management

**User Story:** As the Server_Side, I want the Worker_API to manage image pulls efficiently, so that execution starts promptly and only approved images are used.

#### Acceptance Criteria

1. WHEN the specified image is already present locally, THE Worker_API SHALL skip the pull operation and use the local image.
2. IF an image pull fails (network error, authentication failure, image not found), THEN THE Worker_API SHALL return an error in the Execution_Result with a message indicating the pull failure reason.
3. WHEN pulling an image from a private registry, THE Worker_API SHALL authenticate using credentials provided in the job request or worker configuration.
4. IF credentials are required for a private registry and are not provided in the job request or worker configuration, THEN THE Worker_API SHALL return an error in the Execution_Result indicating that registry credentials are missing.
5. IF an image pull does not complete within 120 seconds, THEN THE Worker_API SHALL cancel the pull operation and return a timeout error in the Execution_Result.

### Requirement 10: Lightweight Resource Footprint

**User Story:** As an operator, I want the Worker_API to have minimal resource consumption when idle, so that a pool of workers can be maintained cost-effectively.

#### Acceptance Criteria

1. WHILE no job is executing and at least 5 seconds have elapsed since the last job completed or since startup, THE Worker_API SHALL consume no more than 32 MB of resident memory.
2. WHILE no job is executing and at least 5 seconds have elapsed since the last job completed or since startup, THE Worker_API SHALL consume no more than 2% of a single CPU core on average.
3. WHEN the container launches, THE Worker_API SHALL be ready to accept job requests on its HTTP port and return a healthy response on the readiness endpoint within 2 seconds.
4. THE Worker_API SHALL be deployable as a container image no larger than 50 MB in compressed size.
