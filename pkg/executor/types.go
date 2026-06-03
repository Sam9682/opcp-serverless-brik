package executor

// JobRequest represents a job execution request from the server side.
type JobRequest struct {
	Image          string            `json:"image"`
	Command        string            `json:"command"`
	Env            map[string]string `json:"env,omitempty"`
	Secrets        []Secret          `json:"secrets,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	CPULimit       *float64          `json:"cpu_limit,omitempty"`
	MemoryLimitMB  *int              `json:"memory_limit_mb,omitempty"`
	NetworkAccess  bool              `json:"network_access"`
	StreamLogs     bool              `json:"stream_logs"`
	RegistryAuth   *RegistryAuth     `json:"registry_auth,omitempty"`
}

// Secret represents a secret to be injected into a container.
type Secret struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	FilePath *string `json:"file_path,omitempty"`
}

// RegistryAuth holds credentials for authenticating to a container registry.
type RegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ExecutionResult represents the structured output of a completed job.
type ExecutionResult struct {
	JobID           string `json:"job_id"`
	Status          string `json:"status"` // "success", "failed", "timeout", "error"
	ExitCode        *int   `json:"exit_code,omitempty"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	DurationMs      int64  `json:"duration_ms"`
	Error           string `json:"error,omitempty"`
}
