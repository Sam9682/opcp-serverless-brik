package health

// HealthStatus represents the response from the health endpoint.
type HealthStatus struct {
	Status  string `json:"status"`            // "healthy" or "unhealthy"
	Runtime string `json:"runtime,omitempty"` // connectivity detail
}

// ReadinessStatus represents the response from the readiness endpoint.
type ReadinessStatus struct {
	Status string `json:"status"`           // "ready" or "busy"
	JobID  string `json:"job_id,omitempty"` // current job if busy
}
