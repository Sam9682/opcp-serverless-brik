package executor

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

// mockRuntime implements runtime.Runtime for testing.
type mockRuntime struct {
	mu             sync.Mutex
	pingErr        error
	pullImageErr   error
	imageExistsVal bool
	imageExistsErr error
	createID       string
	createErr      error
	startErr       error
	waitExitCode   int64
	waitErr        error
	waitDelay      time.Duration
	stopErr        error
	killErr        error
	logsStdout     []byte
	logsStderr     []byte
	logsErr        error
	removeErr      error

	// Track calls for verification.
	pullCalled   bool
	createCalled bool
	startCalled  bool
	removeCalled bool
}

func (m *mockRuntime) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockRuntime) PullImage(ctx context.Context, image string, auth *runtime.RegistryAuth) error {
	m.mu.Lock()
	m.pullCalled = true
	m.mu.Unlock()
	return m.pullImageErr
}

func (m *mockRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	return m.imageExistsVal, m.imageExistsErr
}

func (m *mockRuntime) CreateContainer(ctx context.Context, cfg *runtime.ContainerConfig) (string, error) {
	m.mu.Lock()
	m.createCalled = true
	m.mu.Unlock()
	if m.createErr != nil {
		return "", m.createErr
	}
	if m.createID == "" {
		return "container-123", nil
	}
	return m.createID, nil
}

func (m *mockRuntime) StartContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	m.startCalled = true
	m.mu.Unlock()
	return m.startErr
}

func (m *mockRuntime) WaitContainer(ctx context.Context, id string) (int64, error) {
	if m.waitDelay > 0 {
		select {
		case <-time.After(m.waitDelay):
		case <-ctx.Done():
			return -1, ctx.Err()
		}
	}
	return m.waitExitCode, m.waitErr
}

func (m *mockRuntime) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	return m.stopErr
}

func (m *mockRuntime) KillContainer(ctx context.Context, id string, signal string) error {
	return m.killErr
}

func (m *mockRuntime) GetLogs(ctx context.Context, id string) ([]byte, []byte, error) {
	return m.logsStdout, m.logsStderr, m.logsErr
}

func (m *mockRuntime) RemoveContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	m.removeCalled = true
	m.mu.Unlock()
	return m.removeErr
}

func defaultTestConfig() *config.Config {
	return &config.Config{
		ListenAddr:       ":8080",
		Runtime:          "docker",
		RuntimeSocket:    "/var/run/docker.sock",
		DefaultTimeout:   300 * time.Second,
		MaxTimeout:       3600 * time.Second,
		MaxOutputBytes:   1048576,
		ImagePullTimeout: 120 * time.Second,
	}
}

func TestGenerateJobID(t *testing.T) {
	id := generateJobID()

	// UUID format: 8-4-4-4-12 hex chars
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(id) {
		t.Errorf("generateJobID() = %q, does not match UUID v4 format", id)
	}

	// Ensure uniqueness.
	id2 := generateJobID()
	if id == id2 {
		t.Error("generateJobID() produced duplicate IDs")
	}
}

func TestSubmit_Success(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-abc",
		logsStdout:     []byte("hello\n"),
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}

	jobID, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if jobID == "" {
		t.Fatal("Submit() returned empty job ID")
	}

	// Wait for the job to complete.
	time.Sleep(100 * time.Millisecond)

	result, ok := exec.GetResult(jobID)
	if !ok {
		t.Fatal("GetResult() returned false, expected result to be available")
	}
	if result.Status != "success" {
		t.Errorf("result.Status = %q, want %q", result.Status, "success")
	}
	if result.Stdout != "hello\n" {
		t.Errorf("result.Stdout = %q, want %q", result.Stdout, "hello\n")
	}
}

func TestSubmit_RejectsWhenBusy(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-busy",
		waitDelay:      5 * time.Second, // Job takes a while.
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "sleep 10",
	}

	// Submit first job.
	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("First Submit() error = %v", err)
	}

	// Give it a moment to start.
	time.Sleep(10 * time.Millisecond)

	// Submit second job — should be rejected.
	_, err = exec.Submit(context.Background(), req)
	if err == nil {
		t.Fatal("Second Submit() should have returned error when busy")
	}
}

func TestIsReady_InitiallyReady(t *testing.T) {
	rt := &mockRuntime{}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	if !exec.IsReady() {
		t.Error("IsReady() = false, want true when no job has been submitted")
	}
}

func TestIsReady_BusyDuringExecution(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-running",
		waitDelay:      5 * time.Second,
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "sleep 10",
	}

	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if exec.IsReady() {
		t.Error("IsReady() = true, want false while job is executing")
	}
}

func TestIsReady_ReadyAfterCompletion(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-done",
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo done",
	}

	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	// Wait for job to finish.
	time.Sleep(100 * time.Millisecond)

	if !exec.IsReady() {
		t.Error("IsReady() = false, want true after job completion")
	}
}

func TestSubmit_ImagePullFailure(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: false,
		pullImageErr:   fmt.Errorf("pull failed: image not found"),
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "nonexistent:latest",
		Command: "echo hello",
	}

	jobID, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	// Wait for job to complete.
	time.Sleep(100 * time.Millisecond)

	result, ok := exec.GetResult(jobID)
	if !ok {
		t.Fatal("GetResult() returned false after image pull failure")
	}
	if result.Status != "error" {
		t.Errorf("result.Status = %q, want %q", result.Status, "error")
	}
	if result.Error == "" {
		t.Error("result.Error should contain pull failure message")
	}
}

func TestSubmit_ContainerCreateFailure(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createErr:      fmt.Errorf("no space left on device"),
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}

	jobID, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	result, ok := exec.GetResult(jobID)
	if !ok {
		t.Fatal("GetResult() returned false after container create failure")
	}
	if result.Status != "error" {
		t.Errorf("result.Status = %q, want %q", result.Status, "error")
	}
}

func TestSubmit_NonZeroExitCode(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-failed",
		waitExitCode:   42,
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "exit 42",
	}

	jobID, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	result, ok := exec.GetResult(jobID)
	if !ok {
		t.Fatal("GetResult() returned false after exit 42")
	}
	if result.Status != "failed" {
		t.Errorf("result.Status = %q, want %q", result.Status, "failed")
	}
	if result.ExitCode == nil || *result.ExitCode != 42 {
		t.Errorf("result.ExitCode = %v, want 42", result.ExitCode)
	}
}

func TestSubmit_SecretRedaction(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-secrets",
		logsStdout:     []byte("connecting with password mysecret123"),
		logsStderr:     []byte("error: token=api-key-456"),
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo test",
		Secrets: []Secret{
			{Name: "PASSWORD", Value: "mysecret123"},
			{Name: "API_KEY", Value: "api-key-456"},
		},
	}

	jobID, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	result, ok := exec.GetResult(jobID)
	if !ok {
		t.Fatal("GetResult() returned false")
	}

	// Secrets must be redacted.
	if result.Stdout == "connecting with password mysecret123" {
		t.Error("Stdout should have secret redacted")
	}
	if result.Stderr == "error: token=api-key-456" {
		t.Error("Stderr should have secret redacted")
	}
}

func TestSubmit_SkipsImagePullWhenExists(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-local",
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}

	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	rt.mu.Lock()
	pulled := rt.pullCalled
	rt.mu.Unlock()

	if pulled {
		t.Error("PullImage should not be called when image exists locally")
	}
}

func TestSubmit_ContainerRemovedOnSuccess(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-cleanup",
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}

	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	rt.mu.Lock()
	removed := rt.removeCalled
	rt.mu.Unlock()

	if !removed {
		t.Error("RemoveContainer should be called after job completion")
	}
}

func TestSubmit_ContainerRemovedOnFailure(t *testing.T) {
	rt := &mockRuntime{
		imageExistsVal: true,
		createID:       "container-cleanup-fail",
		startErr:       fmt.Errorf("start failed"),
	}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}

	_, err := exec.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	rt.mu.Lock()
	removed := rt.removeCalled
	rt.mu.Unlock()

	if !removed {
		t.Error("RemoveContainer should be called even when start fails")
	}
}

func TestGetResult_UnknownJobID(t *testing.T) {
	rt := &mockRuntime{}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	result, ok := exec.GetResult("nonexistent-id")
	if ok || result != nil {
		t.Error("GetResult() should return nil, false for unknown job ID")
	}
}

func TestStreamLogs_ReturnsNilForUnknownJob(t *testing.T) {
	rt := &mockRuntime{}
	cfg := defaultTestConfig()
	exec := NewExecutor(rt, cfg)

	ch := exec.StreamLogs(context.Background(), "nonexistent-id")
	if ch != nil {
		t.Error("StreamLogs() should return nil for unknown job ID")
	}
}
