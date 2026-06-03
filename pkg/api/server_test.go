package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/executor"
	"forgejo.org/opcp-serverless-brik/pkg/health"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

// mockRuntime implements runtime.Runtime for testing.
type mockRuntime struct {
	pingErr error
}

func (m *mockRuntime) Ping(ctx context.Context) error              { return m.pingErr }
func (m *mockRuntime) PullImage(ctx context.Context, image string, auth *runtime.RegistryAuth) error {
	return nil
}
func (m *mockRuntime) ImageExists(ctx context.Context, image string) (bool, error) { return true, nil }
func (m *mockRuntime) CreateContainer(ctx context.Context, cfg *runtime.ContainerConfig) (string, error) {
	return "container-123", nil
}
func (m *mockRuntime) StartContainer(ctx context.Context, id string) error { return nil }
func (m *mockRuntime) WaitContainer(ctx context.Context, id string) (int64, error) {
	return 0, nil
}
func (m *mockRuntime) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}
func (m *mockRuntime) KillContainer(ctx context.Context, id string, signal string) error { return nil }
func (m *mockRuntime) GetLogs(ctx context.Context, id string) ([]byte, []byte, error) {
	return []byte("hello"), nil, nil
}
func (m *mockRuntime) RemoveContainer(ctx context.Context, id string) error { return nil }

func newTestServer(whitelist []string) *Server {
	cfg := &config.Config{
		ListenAddr:       ":0",
		Runtime:          "docker",
		DefaultTimeout:   300 * time.Second,
		MaxTimeout:       3600 * time.Second,
		MaxOutputBytes:   1_048_576,
		ImagePullTimeout: 120 * time.Second,
	}
	rt := &mockRuntime{}
	exec := executor.NewExecutor(rt, cfg)
	hc := health.NewChecker(rt, 3*time.Second)
	return NewServer(exec, hc, cfg, whitelist)
}

func TestSubmitJob_MalformedJSON(t *testing.T) {
	srv := newTestServer(nil)
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	srv.handleSubmitJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}

func TestSubmitJob_MissingRequiredFields(t *testing.T) {
	srv := newTestServer(nil)
	body := `{"image":"","command":""}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleSubmitJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", resp.Error.Code)
	}
	if len(resp.Error.Details) < 2 {
		t.Errorf("expected at least 2 validation errors, got %d", len(resp.Error.Details))
	}
}

func TestSubmitJob_RegistryRejected(t *testing.T) {
	srv := newTestServer([]string{"docker.io", "ghcr.io"})
	body := `{"image":"evil.registry.com/hack:latest","command":"echo hello"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleSubmitJob(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "REGISTRY_DENIED" {
		t.Errorf("expected REGISTRY_DENIED, got %s", resp.Error.Code)
	}
}

func TestSubmitJob_Success(t *testing.T) {
	srv := newTestServer(nil)
	body := `{"image":"alpine:latest","command":"echo hello"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleSubmitJob(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["job_id"] == "" {
		t.Error("expected non-empty job_id")
	}
}

func TestSubmitJob_WorkerBusy(t *testing.T) {
	srv := newTestServer(nil)

	// Submit the first job.
	body := `{"image":"alpine:latest","command":"sleep 100"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSubmitJob(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("first submit: expected 202, got %d", w.Code)
	}

	// Submit a second job - should get 409.
	req2 := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w2 := httptest.NewRecorder()
	srv.handleSubmitJob(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second submit: expected 409, got %d", w2.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "WORKER_BUSY" {
		t.Errorf("expected WORKER_BUSY, got %s", resp.Error.Code)
	}
}

func TestGetResult_NotFound(t *testing.T) {
	srv := newTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/jobs/nonexistent-id", nil)
	req.SetPathValue("id", "nonexistent-id")
	w := httptest.NewRecorder()

	srv.handleGetResult(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetResult_Success(t *testing.T) {
	srv := newTestServer(nil)

	// Submit a job.
	body := `{"image":"alpine:latest","command":"echo hello"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSubmitJob(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("submit: expected 202, got %d", w.Code)
	}

	var submitResp map[string]string
	json.NewDecoder(w.Body).Decode(&submitResp)
	jobID := submitResp["job_id"]

	// Wait for the job to complete (mock runtime returns immediately).
	time.Sleep(100 * time.Millisecond)

	// Get the result.
	getReq := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID, nil)
	getReq.SetPathValue("id", jobID)
	getW := httptest.NewRecorder()
	srv.handleGetResult(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getW.Code)
	}

	var result executor.ExecutionResult
	if err := json.NewDecoder(getW.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	if result.JobID != jobID {
		t.Errorf("expected job_id %s, got %s", jobID, result.JobID)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %s", result.Status)
	}
}

func TestStreamLogs_NotFound(t *testing.T) {
	srv := newTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/jobs/nonexistent/logs", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	srv.handleStreamLogs(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStreamLogs_SSEHeaders(t *testing.T) {
	srv := newTestServer(nil)

	// Submit a job.
	body := `{"image":"alpine:latest","command":"echo hello","stream_logs":true}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSubmitJob(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("submit: expected 202, got %d", w.Code)
	}

	var submitResp map[string]string
	json.NewDecoder(w.Body).Decode(&submitResp)
	jobID := submitResp["job_id"]

	// Give a moment for the log buffer to be set up.
	time.Sleep(50 * time.Millisecond)

	// Stream logs.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	streamReq := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID+"/logs", nil).WithContext(ctx)
	streamReq.SetPathValue("id", jobID)
	streamW := httptest.NewRecorder()

	srv.handleStreamLogs(streamW, streamReq)

	if streamW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", streamW.Code)
	}

	if ct := streamW.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := streamW.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}
	if conn := streamW.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %s", conn)
	}

	// Verify the stream contains SSE events.
	output := streamW.Body.String()
	if !strings.Contains(output, "event: end") {
		t.Errorf("expected end event in output, got: %s", output)
	}
}

func TestHealth_Healthy(t *testing.T) {
	srv := newTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp health.HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected healthy, got %s", resp.Status)
	}
}

func TestReady_Ready(t *testing.T) {
	srv := newTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	srv.handleReady(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp health.ReadinessStatus
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ready" {
		t.Errorf("expected ready, got %s", resp.Status)
	}
}

func TestReady_Busy(t *testing.T) {
	srv := newTestServer(nil)

	// Submit a job to make it busy.
	body := `{"image":"alpine:latest","command":"sleep 100"}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSubmitJob(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("submit: expected 202, got %d", w.Code)
	}

	// Check readiness.
	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyW := httptest.NewRecorder()
	srv.handleReady(readyW, readyReq)

	if readyW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", readyW.Code)
	}

	var resp health.ReadinessStatus
	if err := json.NewDecoder(readyW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "busy" {
		t.Errorf("expected busy, got %s", resp.Status)
	}
}

func TestSubmitJob_OutOfRangeFields(t *testing.T) {
	srv := newTestServer(nil)
	timeout := 0
	body, _ := json.Marshal(executor.JobRequest{
		Image:          "alpine:latest",
		Command:        "echo hello",
		TimeoutSeconds: &timeout,
	})
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.handleSubmitJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}


