package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"forgejo.org/opcp-serverless-brik/pkg/executor"
	"forgejo.org/opcp-serverless-brik/pkg/validator"
)

// ErrorResponse is the standard error response envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the error code, message, and optional details.
type ErrorBody struct {
	Code    string                   `json:"code"`
	Message string                   `json:"message"`
	Details []validator.ValidationError `json:"details,omitempty"`
}

// handleSubmitJob processes POST /jobs requests.
func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	// Parse the JSON request body.
	var req executor.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request body is malformed or not valid JSON", nil)
		return
	}

	// Validate the request.
	validationErrors := validator.ValidateJobRequest(&req, s.whitelist)
	if len(validationErrors) > 0 {
		// Check if any error is a registry rejection (Field=="image" and message contains "whitelist").
		for _, ve := range validationErrors {
			if ve.Field == "image" && strings.Contains(ve.Message, "whitelist") {
				writeError(w, http.StatusForbidden, "REGISTRY_DENIED", "Image registry is not in the approved whitelist", validationErrors)
				return
			}
		}
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request validation failed", validationErrors)
		return
	}

	// Check if the executor can accept work.
	if !s.executor.IsReady() {
		writeError(w, http.StatusConflict, "WORKER_BUSY", "Worker is currently executing a job", nil)
		return
	}

	// Submit the job.
	jobID, err := s.executor.Submit(r.Context(), &req)
	if err != nil {
		// Race condition: became busy between IsReady check and Submit.
		writeError(w, http.StatusConflict, "WORKER_BUSY", "Worker is currently executing a job", nil)
		return
	}

	// Return 202 Accepted with the job ID.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

// handleGetResult processes GET /jobs/{id} requests.
func (s *Server) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Job ID is required", nil)
		return
	}

	result, found := s.executor.GetResult(id)
	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Job %q not found or not yet complete", id), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// handleStreamLogs processes GET /jobs/{id}/logs requests using Server-Sent Events.
func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Job ID is required", nil)
		return
	}

	// Verify the response writer supports flushing (required for SSE).
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming not supported", nil)
		return
	}

	// Subscribe to the log stream for this job.
	logCh := s.executor.StreamLogs(r.Context(), id)
	if logCh == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Job %q not found", id), nil)
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Stream log lines as SSE events.
	for line := range logCh {
		data, err := json.Marshal(line)
		if err != nil {
			slog.Error("failed to marshal log line", "error", err)
			continue
		}
		_, writeErr := fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
		if writeErr != nil {
			// Client disconnected.
			return
		}
		flusher.Flush()
	}

	// Send end-of-stream event with job status.
	endEvent := s.buildEndEvent(id)
	data, _ := json.Marshal(endEvent)
	fmt.Fprintf(w, "event: end\ndata: %s\n\n", data)
	flusher.Flush()
}

// sseEndEvent represents the end-of-stream SSE message.
type sseEndEvent struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

// buildEndEvent constructs the end-of-stream event from the job result.
func (s *Server) buildEndEvent(id string) sseEndEvent {
	result, found := s.executor.GetResult(id)
	if !found {
		return sseEndEvent{Status: "unknown"}
	}
	return sseEndEvent{
		Status:   result.Status,
		ExitCode: result.ExitCode,
	}
}

// handleHealth processes GET /health requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := s.health.Check(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// handleReady processes GET /ready requests.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	status := s.health.IsReady(s.executor)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// writeError writes a standard error response.
func writeError(w http.ResponseWriter, statusCode int, code, message string, details []validator.ValidationError) {
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}


