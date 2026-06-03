package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
	"forgejo.org/opcp-serverless-brik/pkg/stream"
)

const (
	// redactionPlaceholder is the string used to replace secret values in output.
	redactionPlaceholder = "***REDACTED***"

	// gracefulShutdownTimeout is the time to wait after SIGTERM before SIGKILL.
	gracefulShutdownTimeout = 10 * time.Second
)

// Executor orchestrates the full job lifecycle: pull → create → start → monitor → collect → cleanup.
// It processes exactly one job at a time.
type Executor struct {
	runtime runtime.Runtime
	config  *config.Config
	mu      sync.Mutex
	current *RunningJob
}

// RunningJob holds the state of a currently executing or completed job.
type RunningJob struct {
	ID        string
	Request   *JobRequest
	Result    *ExecutionResult
	LogBuffer *stream.LogBuffer
	Cancel    context.CancelFunc
	Done      chan struct{}
}

// NewExecutor creates a new Executor with the given runtime and configuration.
func NewExecutor(rt runtime.Runtime, cfg *config.Config) *Executor {
	return &Executor{
		runtime: rt,
		config:  cfg,
	}
}

// Submit accepts a job request and begins asynchronous execution.
// It returns the job ID immediately. If the executor is busy, it returns an error.
func (e *Executor) Submit(ctx context.Context, req *JobRequest) (string, error) {
	e.mu.Lock()
	if e.current != nil {
		select {
		case <-e.current.Done:
			// Previous job is complete, we can accept a new one.
		default:
			e.mu.Unlock()
			return "", fmt.Errorf("worker busy: job %s is currently executing", e.current.ID)
		}
	}

	jobID := generateJobID()
	logBuffer := stream.NewLogBuffer()

	jobCtx, cancel := context.WithCancel(context.Background())

	job := &RunningJob{
		ID:        jobID,
		Request:   req,
		LogBuffer: logBuffer,
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}
	e.current = job
	e.mu.Unlock()

	// Launch the execution goroutine.
	go e.executeJob(jobCtx, job)

	return jobID, nil
}

// GetResult returns the execution result for the given job ID.
// Returns the result and true if the job is done, or nil and false otherwise.
func (e *Executor) GetResult(id string) (*ExecutionResult, bool) {
	e.mu.Lock()
	job := e.current
	e.mu.Unlock()

	if job == nil || job.ID != id {
		return nil, false
	}

	// Check if the job is done.
	select {
	case <-job.Done:
		return job.Result, job.Result != nil
	default:
		return nil, false
	}
}

// IsReady returns true when no job is currently running and the executor
// can accept new work.
func (e *Executor) IsReady() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current == nil {
		return true
	}

	select {
	case <-e.current.Done:
		return true
	default:
		return false
	}
}

// CurrentJobID returns the ID of the current job, or empty string if none.
func (e *Executor) CurrentJobID() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current == nil {
		return ""
	}
	return e.current.ID
}

// StreamLogs returns a channel that emits log lines for the given job ID.
// Returns nil if the job is not found or not the current job.
func (e *Executor) StreamLogs(ctx context.Context, id string) <-chan stream.LogLine {
	e.mu.Lock()
	job := e.current
	e.mu.Unlock()

	if job == nil || job.ID != id {
		return nil
	}

	return job.LogBuffer.Subscribe(ctx)
}

// executeJob runs the full container lifecycle for a job.
func (e *Executor) executeJob(ctx context.Context, job *RunningJob) {
	defer close(job.Done)
	defer job.LogBuffer.Close()
	defer job.Cancel()

	start := time.Now()

	// Compute effective timeout for the job.
	timeout := EffectiveTimeout(job.Request, e.config)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()

	// Step 1: Image pull (with separate timeout).
	if err := e.pullImageIfNeeded(ctx, job); err != nil {
		job.Result = &ExecutionResult{
			JobID:      job.ID,
			Status:     "error",
			Error:      fmt.Sprintf("image pull failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
		return
	}

	// Step 2: Build container config.
	containerCfg := BuildContainerConfig(job.Request, e.config)

	// Step 3: Create container.
	containerID, err := e.runtime.CreateContainer(timeoutCtx, containerCfg)
	if err != nil {
		job.Result = &ExecutionResult{
			JobID:      job.ID,
			Status:     "error",
			Error:      fmt.Sprintf("container create failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
		return
	}

	// Defer container removal (force) to prevent leaks.
	defer func() {
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rmCancel()
		if rmErr := e.runtime.RemoveContainer(rmCtx, containerID); rmErr != nil {
			slog.Error("failed to remove container", "container_id", containerID, "error", rmErr)
		}
	}()

	// Step 4: Start container.
	if err := e.runtime.StartContainer(timeoutCtx, containerID); err != nil {
		job.Result = &ExecutionResult{
			JobID:      job.ID,
			Status:     "error",
			Error:      fmt.Sprintf("container start failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
		return
	}

	// Record the actual execution start time (after container starts).
	execStart := time.Now()

	// Step 5: Set up log streaming in a separate goroutine.
	go e.streamContainerLogs(timeoutCtx, containerID, job.LogBuffer)

	// Step 6: Wait for container to finish or timeout.
	exitCode, waitErr := e.runtime.WaitContainer(timeoutCtx, containerID)

	var timedOut bool
	if waitErr != nil && timeoutCtx.Err() != nil {
		// Timeout occurred — execute graceful shutdown sequence.
		timedOut = true
		e.handleTimeout(containerID)
	}

	duration := time.Since(execStart).Milliseconds()

	// Step 7: Collect output.
	logCtx, logCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer logCancel()
	stdout, stderr, logErr := e.runtime.GetLogs(logCtx, containerID)
	if logErr != nil {
		slog.Error("failed to collect container logs", "container_id", containerID, "error", logErr)
	}

	// Step 8: Apply output truncation.
	outStr, errStr, outTruncated, errTruncated := CaptureOutput(stdout, stderr, e.config.MaxOutputBytes)

	// Step 9: Apply secret redaction.
	outStr = RedactSecrets(outStr, job.Request.Secrets, redactionPlaceholder)
	errStr = RedactSecrets(errStr, job.Request.Secrets, redactionPlaceholder)

	// Step 10: Build execution result.
	result := &ExecutionResult{
		JobID:           job.ID,
		Stdout:          outStr,
		Stderr:          errStr,
		StdoutTruncated: outTruncated,
		StderrTruncated: errTruncated,
		DurationMs:      duration,
	}

	if timedOut {
		result.Status = "timeout"
		ec := int(exitCode)
		result.ExitCode = &ec
	} else if waitErr != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("container wait failed: %v", waitErr)
	} else {
		ec := int(exitCode)
		result.ExitCode = &ec
		if exitCode == 0 {
			result.Status = "success"
		} else {
			result.Status = "failed"
		}
	}

	job.Result = result
}

// pullImageIfNeeded checks if the image exists locally and pulls it if not.
func (e *Executor) pullImageIfNeeded(ctx context.Context, job *RunningJob) error {
	// Check if image already exists locally.
	exists, err := e.runtime.ImageExists(ctx, job.Request.Image)
	if err != nil {
		return fmt.Errorf("checking image existence: %w", err)
	}
	if exists {
		return nil
	}

	// Pull with a separate timeout from config.
	pullCtx, pullCancel := context.WithTimeout(ctx, e.config.ImagePullTimeout)
	defer pullCancel()

	// Convert registry auth if provided.
	var auth *runtime.RegistryAuth
	if job.Request.RegistryAuth != nil {
		auth = &runtime.RegistryAuth{
			Username: job.Request.RegistryAuth.Username,
			Password: job.Request.RegistryAuth.Password,
		}
	}

	if err := e.runtime.PullImage(pullCtx, job.Request.Image, auth); err != nil {
		if pullCtx.Err() != nil {
			return fmt.Errorf("image pull timed out after %v", e.config.ImagePullTimeout)
		}
		return err
	}

	return nil
}

// handleTimeout performs the graceful shutdown sequence: SIGTERM → wait 10s → SIGKILL.
func (e *Executor) handleTimeout(containerID string) {
	termCtx, termCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout+5*time.Second)
	defer termCancel()

	// Send SIGTERM via StopContainer with graceful timeout.
	if err := e.runtime.StopContainer(termCtx, containerID, gracefulShutdownTimeout); err != nil {
		// If stop fails, force kill.
		slog.Warn("stop container failed, sending SIGKILL", "container_id", containerID, "error", err)
		killCtx, killCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer killCancel()
		if killErr := e.runtime.KillContainer(killCtx, containerID, "SIGKILL"); killErr != nil {
			slog.Error("failed to kill container", "container_id", containerID, "error", killErr)
		}
	}
}

// streamContainerLogs reads logs from the runtime and writes them to the log buffer.
func (e *Executor) streamContainerLogs(ctx context.Context, containerID string, buf *stream.LogBuffer) {
	stdout, stderr, err := e.runtime.GetLogs(ctx, containerID)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("failed to stream container logs", "container_id", containerID, "error", err)
		}
		return
	}

	// Write stdout lines to buffer.
	if len(stdout) > 0 {
		buf.Write("stdout", string(stdout))
	}
	// Write stderr lines to buffer.
	if len(stderr) > 0 {
		buf.Write("stderr", string(stderr))
	}
}

// generateJobID creates a new UUID v4 string in the format 8-4-4-4-12 hex.
func generateJobID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])

	// Set version 4 (bits 12-15 of time_hi_and_version).
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant (bits 6-7 of clock_seq_hi_and_reserved).
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	var buf [36]byte
	hex.Encode(buf[0:8], uuid[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], uuid[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], uuid[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], uuid[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], uuid[10:16])

	return string(buf[:])
}
