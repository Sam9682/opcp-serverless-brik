package validator

import (
	"strings"
	"testing"

	"forgejo.org/opcp-serverless-brik/pkg/executor"
)

func intPtr(v int) *int         { return &v }
func float64Ptr(v float64) *float64 { return &v }

func TestValidateJobRequest_ValidRequest(t *testing.T) {
	req := &executor.JobRequest{
		Image:          "docker.io/library/alpine:latest",
		Command:        "echo hello",
		TimeoutSeconds: intPtr(60),
		CPULimit:       float64Ptr(1.0),
		MemoryLimitMB:  intPtr(256),
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateJobRequest_MissingImage(t *testing.T) {
	req := &executor.JobRequest{
		Image:   "",
		Command: "echo hello",
	}

	errs := ValidateJobRequest(req, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing image")
	}
	found := false
	for _, e := range errs {
		if e.Field == "image" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error on 'image' field")
	}
}

func TestValidateJobRequest_MissingCommand(t *testing.T) {
	req := &executor.JobRequest{
		Image:   "alpine",
		Command: "",
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing command")
	}
	found := false
	for _, e := range errs {
		if e.Field == "command" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error on 'command' field")
	}
}

func TestValidateJobRequest_NonWhitelistedRegistry(t *testing.T) {
	req := &executor.JobRequest{
		Image:   "evil.registry.com/malware:latest",
		Command: "echo hello",
	}
	whitelist := []string{"docker.io", "ghcr.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for non-whitelisted registry")
	}
	found := false
	for _, e := range errs {
		if e.Field == "image" && strings.Contains(e.Message, "whitelist") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected registry whitelist error")
	}
}

func TestValidateJobRequest_OutOfRangeTimeout(t *testing.T) {
	req := &executor.JobRequest{
		Image:          "docker.io/alpine",
		Command:        "echo",
		TimeoutSeconds: intPtr(5000),
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for out-of-range timeout")
	}
	found := false
	for _, e := range errs {
		if e.Field == "timeout_seconds" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error on 'timeout_seconds' field")
	}
}

func TestValidateJobRequest_OutOfRangeCPU(t *testing.T) {
	req := &executor.JobRequest{
		Image:    "docker.io/alpine",
		Command:  "echo",
		CPULimit: float64Ptr(10.0),
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for out-of-range CPU")
	}
	found := false
	for _, e := range errs {
		if e.Field == "cpu_limit" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error on 'cpu_limit' field")
	}
}

func TestValidateJobRequest_OutOfRangeMemory(t *testing.T) {
	req := &executor.JobRequest{
		Image:         "docker.io/alpine",
		Command:       "echo",
		MemoryLimitMB: intPtr(10000),
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for out-of-range memory")
	}
	found := false
	for _, e := range errs {
		if e.Field == "memory_limit_mb" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error on 'memory_limit_mb' field")
	}
}

func TestValidateSecretName_Valid(t *testing.T) {
	validNames := []string{"MY_SECRET", "A", "_SECRET", "A1_B2_C3", "SECRET_KEY_123"}
	for _, name := range validNames {
		if err := ValidateSecretName(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}
}

func TestValidateSecretName_Invalid(t *testing.T) {
	invalidNames := []string{"", "123ABC", "my_secret", "my-secret", "A B C", "1START"}
	for _, name := range invalidNames {
		if err := ValidateSecretName(name); err == nil {
			t.Errorf("expected %q to be invalid, got nil error", name)
		}
	}
}

func TestValidateSecrets_TooMany(t *testing.T) {
	secrets := make([]executor.Secret, 65)
	for i := range secrets {
		secrets[i] = executor.Secret{Name: "SECRET_A", Value: "val"}
	}

	errs := ValidateSecrets(secrets)
	if len(errs) == 0 {
		t.Fatal("expected error for too many secrets")
	}
}

func TestValidateSecrets_OversizedValue(t *testing.T) {
	bigValue := strings.Repeat("x", 65537)
	secrets := []executor.Secret{{Name: "MY_SECRET", Value: bigValue}}

	errs := ValidateSecrets(secrets)
	if len(errs) == 0 {
		t.Fatal("expected error for oversized secret value")
	}
}

func TestExtractRegistry(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		{"alpine", "docker.io"},
		{"library/alpine", "docker.io"},
		{"alpine:latest", "docker.io"},
		{"docker.io/library/alpine", "docker.io"},
		{"docker.io/library/alpine:3.18", "docker.io"},
		{"ghcr.io/owner/repo:tag", "ghcr.io"},
		{"registry.example.com/image:tag", "registry.example.com"},
		{"registry.example.com:5000/image:tag", "registry.example.com:5000"},
		{"localhost/my-image", "localhost"},
		{"myuser/myrepo", "docker.io"},
		{"myuser/myrepo:latest", "docker.io"},
	}

	for _, tt := range tests {
		got := extractRegistry(tt.image)
		if got != tt.expected {
			t.Errorf("extractRegistry(%q) = %q, want %q", tt.image, got, tt.expected)
		}
	}
}

func TestValidateJobRequest_EmptyWhitelist(t *testing.T) {
	req := &executor.JobRequest{
		Image:   "evil.registry.com/image:latest",
		Command: "echo hello",
	}

	// Empty whitelist means no restriction.
	errs := ValidateJobRequest(req, nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors with empty whitelist, got %v", errs)
	}

	errs = ValidateJobRequest(req, []string{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors with empty whitelist slice, got %v", errs)
	}
}

func TestValidateJobRequest_EnvVarsTooMany(t *testing.T) {
	env := make(map[string]string)
	for i := 0; i < 65; i++ {
		env[strings.Repeat("K", i+1)] = "value"
	}
	req := &executor.JobRequest{
		Image:   "docker.io/alpine",
		Command: "echo",
		Env:     env,
	}
	whitelist := []string{"docker.io"}

	errs := ValidateJobRequest(req, whitelist)
	if len(errs) == 0 {
		t.Fatal("expected validation error for too many env vars")
	}
}
