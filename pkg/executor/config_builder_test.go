package executor

import (
	"testing"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt(v int) *int             { return &v }
func ptrString(v string) *string    { return &v }

func defaultConfig() *config.Config {
	return &config.Config{
		DefaultTimeout: 300 * time.Second,
		MaxTimeout:     3600 * time.Second,
		MaxOutputBytes: 1048576,
	}
}

func TestBuildContainerConfig_Defaults(t *testing.T) {
	req := &JobRequest{
		Image:   "alpine:latest",
		Command: "echo hello",
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	// Security defaults
	if cc.User != "1000:1000" {
		t.Errorf("expected User '1000:1000', got %q", cc.User)
	}
	if !cc.ReadOnlyFS {
		t.Error("expected ReadOnlyFS to be true")
	}
	if len(cc.CapDrop) != 1 || cc.CapDrop[0] != "ALL" {
		t.Errorf("expected CapDrop [ALL], got %v", cc.CapDrop)
	}

	// Default CPU and memory
	if cc.CPULimit != 1.0 {
		t.Errorf("expected default CPULimit 1.0, got %f", cc.CPULimit)
	}
	if cc.MemoryLimit != 256*1048576 {
		t.Errorf("expected default MemoryLimit %d, got %d", 256*1048576, cc.MemoryLimit)
	}

	// NetworkOff should be true when NetworkAccess is false (default)
	if !cc.NetworkOff {
		t.Error("expected NetworkOff to be true when NetworkAccess is false")
	}

	// Command parsing
	if len(cc.Command) != 2 || cc.Command[0] != "echo" || cc.Command[1] != "hello" {
		t.Errorf("expected command [echo hello], got %v", cc.Command)
	}

	// Image
	if cc.Image != "alpine:latest" {
		t.Errorf("expected image 'alpine:latest', got %q", cc.Image)
	}
}

func TestBuildContainerConfig_WithLimits(t *testing.T) {
	req := &JobRequest{
		Image:         "myimage:v1",
		Command:       "sh -c 'echo test'",
		CPULimit:      ptrFloat64(2.5),
		MemoryLimitMB: ptrInt(512),
		NetworkAccess: true,
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	if cc.CPULimit != 2.5 {
		t.Errorf("expected CPULimit 2.5, got %f", cc.CPULimit)
	}
	if cc.MemoryLimit != 512*1048576 {
		t.Errorf("expected MemoryLimit %d, got %d", 512*1048576, cc.MemoryLimit)
	}
	if cc.NetworkOff {
		t.Error("expected NetworkOff to be false when NetworkAccess is true")
	}
}

func TestBuildContainerConfig_EnvVars(t *testing.T) {
	req := &JobRequest{
		Image:   "alpine",
		Command: "env",
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	envMap := make(map[string]bool)
	for _, e := range cc.Env {
		envMap[e] = true
	}
	if !envMap["FOO=bar"] {
		t.Error("expected FOO=bar in Env")
	}
	if !envMap["BAZ=qux"] {
		t.Error("expected BAZ=qux in Env")
	}
}

func TestBuildContainerConfig_Secrets_EnvVar(t *testing.T) {
	req := &JobRequest{
		Image:   "alpine",
		Command: "env",
		Secrets: []Secret{
			{Name: "DB_PASSWORD", Value: "secret123"},
			{Name: "API_KEY", Value: "key456"},
		},
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	envMap := make(map[string]bool)
	for _, e := range cc.Env {
		envMap[e] = true
	}
	if !envMap["DB_PASSWORD=secret123"] {
		t.Error("expected DB_PASSWORD=secret123 in Env")
	}
	if !envMap["API_KEY=key456"] {
		t.Error("expected API_KEY=key456 in Env")
	}
	if len(cc.Mounts) != 0 {
		t.Errorf("expected no mounts for env-only secrets, got %d", len(cc.Mounts))
	}
}

func TestBuildContainerConfig_Secrets_FileBased(t *testing.T) {
	filePath := "/etc/secrets/db.conf"
	req := &JobRequest{
		Image:   "alpine",
		Command: "cat /etc/secrets/db.conf",
		Secrets: []Secret{
			{Name: "DB_CONFIG", Value: "host=localhost", FilePath: &filePath},
		},
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	if len(cc.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(cc.Mounts))
	}
	mount := cc.Mounts[0]
	expectedMount := runtime.Mount{
		Source:   "/run/secrets/DB_CONFIG",
		Target:   "/etc/secrets/db.conf",
		ReadOnly: true,
	}
	if mount != expectedMount {
		t.Errorf("expected mount %+v, got %+v", expectedMount, mount)
	}

	// File-based secret should NOT appear as env var
	for _, e := range cc.Env {
		if e == "DB_CONFIG=host=localhost" {
			t.Error("file-based secret should not appear in Env")
		}
	}
}

func TestBuildContainerConfig_MixedSecrets(t *testing.T) {
	filePath := "/app/creds.json"
	req := &JobRequest{
		Image:   "alpine",
		Command: "run",
		Secrets: []Secret{
			{Name: "TOKEN", Value: "abc"},
			{Name: "CERT", Value: "cert-data", FilePath: &filePath},
		},
	}
	cfg := defaultConfig()

	cc := BuildContainerConfig(req, cfg)

	// One env var secret, one file-based
	envMap := make(map[string]bool)
	for _, e := range cc.Env {
		envMap[e] = true
	}
	if !envMap["TOKEN=abc"] {
		t.Error("expected TOKEN=abc in Env")
	}
	if len(cc.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(cc.Mounts))
	}
	if cc.Mounts[0].Target != "/app/creds.json" {
		t.Errorf("expected mount target /app/creds.json, got %q", cc.Mounts[0].Target)
	}
}

func TestBuildContainerConfig_CommandParsing(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
	}{
		{"simple", "echo hello", []string{"echo", "hello"}},
		{"single arg", "ls", []string{"ls"}},
		{"quoted spaces", `sh -c "echo hello world"`, []string{"sh", "-c", "echo hello world"}},
		{"single quoted", "sh -c 'echo hello'", []string{"sh", "-c", "echo hello"}},
		{"multiple spaces", "echo   hello", []string{"echo", "hello"}},
	}

	cfg := defaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &JobRequest{Image: "alpine", Command: tc.command}
			cc := BuildContainerConfig(req, cfg)
			if len(cc.Command) != len(tc.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tc.expected), len(cc.Command), cc.Command)
			}
			for i, arg := range tc.expected {
				if cc.Command[i] != arg {
					t.Errorf("arg[%d]: expected %q, got %q", i, arg, cc.Command[i])
				}
			}
		})
	}
}

func TestEffectiveTimeout_Default(t *testing.T) {
	req := &JobRequest{Image: "alpine", Command: "echo"}
	cfg := defaultConfig()

	timeout := EffectiveTimeout(req, cfg)
	if timeout != 300*time.Second {
		t.Errorf("expected 300s, got %v", timeout)
	}
}

func TestEffectiveTimeout_Specified(t *testing.T) {
	req := &JobRequest{
		Image:          "alpine",
		Command:        "echo",
		TimeoutSeconds: ptrInt(60),
	}
	cfg := defaultConfig()

	timeout := EffectiveTimeout(req, cfg)
	if timeout != 60*time.Second {
		t.Errorf("expected 60s, got %v", timeout)
	}
}

func TestEffectiveTimeout_ExceedsMax(t *testing.T) {
	req := &JobRequest{
		Image:          "alpine",
		Command:        "echo",
		TimeoutSeconds: ptrInt(7200),
	}
	cfg := defaultConfig()

	timeout := EffectiveTimeout(req, cfg)
	if timeout != 3600*time.Second {
		t.Errorf("expected 3600s (max), got %v", timeout)
	}
}

func TestEffectiveTimeout_ExactlyMax(t *testing.T) {
	req := &JobRequest{
		Image:          "alpine",
		Command:        "echo",
		TimeoutSeconds: ptrInt(3600),
	}
	cfg := defaultConfig()

	timeout := EffectiveTimeout(req, cfg)
	if timeout != 3600*time.Second {
		t.Errorf("expected 3600s, got %v", timeout)
	}
}
