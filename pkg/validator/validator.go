package validator

import (
	"fmt"
	"regexp"
	"strings"

	"forgejo.org/opcp-serverless-brik/pkg/executor"
)

const (
	maxImageLen     = 512
	maxCommandLen   = 1024
	maxEnvVars      = 64
	maxEnvKeyLen    = 256
	maxEnvValueLen  = 4096
	minTimeout      = 1
	maxTimeout      = 3600
	minCPU          = 0.1
	maxCPU          = 4.0
	minMemoryMB     = 16
	maxMemoryMB     = 8192
	maxSecrets      = 64
	maxSecretValue  = 65536 // 64 KB
)

// secretNameRegex matches valid secret names: uppercase letters, digits, underscores,
// not starting with a digit, maximum 256 characters total.
var secretNameRegex = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,255}$`)

// ValidateJobRequest validates a job execution request against all constraints.
// It returns a slice of validation errors (empty if the request is valid).
func ValidateJobRequest(req *executor.JobRequest, whitelist []string) []ValidationError {
	var errs []ValidationError

	// Validate image reference: required, max 512 chars, registry in whitelist.
	if req.Image == "" {
		errs = append(errs, ValidationError{Field: "image", Message: "image reference is required"})
	} else {
		if len(req.Image) > maxImageLen {
			errs = append(errs, ValidationError{Field: "image", Message: fmt.Sprintf("image reference must not exceed %d characters", maxImageLen)})
		}
		if len(whitelist) > 0 {
			registry := extractRegistry(req.Image)
			if !isRegistryWhitelisted(registry, whitelist) {
				errs = append(errs, ValidationError{Field: "image", Message: fmt.Sprintf("registry %q is not in the approved whitelist", registry)})
			}
		}
	}

	// Validate command: required, max 1024 chars.
	if req.Command == "" {
		errs = append(errs, ValidationError{Field: "command", Message: "command is required"})
	} else if len(req.Command) > maxCommandLen {
		errs = append(errs, ValidationError{Field: "command", Message: fmt.Sprintf("command must not exceed %d characters", maxCommandLen)})
	}

	// Validate environment variables: max 64, key max 256 chars, value max 4096 chars.
	if len(req.Env) > maxEnvVars {
		errs = append(errs, ValidationError{Field: "env", Message: fmt.Sprintf("number of environment variables must not exceed %d", maxEnvVars)})
	} else {
		for key, value := range req.Env {
			if len(key) > maxEnvKeyLen {
				errs = append(errs, ValidationError{Field: "env", Message: fmt.Sprintf("environment variable key %q must not exceed %d characters", key, maxEnvKeyLen)})
			}
			if len(value) > maxEnvValueLen {
				errs = append(errs, ValidationError{Field: "env", Message: fmt.Sprintf("environment variable value for key %q must not exceed %d characters", key, maxEnvValueLen)})
			}
		}
	}

	// Validate timeout: 1-3600 range if provided.
	if req.TimeoutSeconds != nil {
		t := *req.TimeoutSeconds
		if t < minTimeout || t > maxTimeout {
			errs = append(errs, ValidationError{Field: "timeout_seconds", Message: fmt.Sprintf("must be between %d and %d", minTimeout, maxTimeout)})
		}
	}

	// Validate CPU limit: 0.1-4.0 range if provided.
	if req.CPULimit != nil {
		cpu := *req.CPULimit
		if cpu < minCPU || cpu > maxCPU {
			errs = append(errs, ValidationError{Field: "cpu_limit", Message: fmt.Sprintf("must be between %.1f and %.1f", minCPU, maxCPU)})
		}
	}

	// Validate memory limit: 16-8192 MB range if provided.
	if req.MemoryLimitMB != nil {
		mem := *req.MemoryLimitMB
		if mem < minMemoryMB || mem > maxMemoryMB {
			errs = append(errs, ValidationError{Field: "memory_limit_mb", Message: fmt.Sprintf("must be between %d and %d", minMemoryMB, maxMemoryMB)})
		}
	}

	// Validate secrets.
	secretErrs := ValidateSecrets(req.Secrets)
	errs = append(errs, secretErrs...)

	return errs
}

// ValidateSecretName validates that a secret name matches the required pattern:
// uppercase letters, digits, and underscores, not starting with a digit, max 256 chars.
func ValidateSecretName(name string) error {
	if !secretNameRegex.MatchString(name) {
		return fmt.Errorf("secret name %q must match pattern ^[A-Z_][A-Z0-9_]{0,255}$", name)
	}
	return nil
}

// ValidateSecrets validates a slice of secrets against all constraints:
// max 64 secrets, valid names, values not exceeding 64 KB.
func ValidateSecrets(secrets []executor.Secret) []ValidationError {
	var errs []ValidationError

	if len(secrets) > maxSecrets {
		errs = append(errs, ValidationError{Field: "secrets", Message: fmt.Sprintf("number of secrets must not exceed %d", maxSecrets)})
		return errs
	}

	for i, s := range secrets {
		if err := ValidateSecretName(s.Name); err != nil {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("secrets[%d].name", i),
				Message: fmt.Sprintf("invalid secret name: %s", err.Error()),
			})
		}
		if len(s.Value) > maxSecretValue {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("secrets[%d].value", i),
				Message: fmt.Sprintf("secret value must not exceed %d bytes", maxSecretValue),
			})
		}
	}

	return errs
}

// extractRegistry extracts the registry component from a container image reference.
// It handles:
//   - Fully qualified: registry.example.com/image:tag → registry.example.com
//   - Docker Hub shorthand: library/alpine → docker.io
//   - Docker Hub bare name: alpine → docker.io
//   - Docker Hub with docker.io prefix: docker.io/library/alpine → docker.io
func extractRegistry(image string) string {
	// Remove tag and digest.
	ref := image
	if atIdx := strings.Index(ref, "@"); atIdx != -1 {
		ref = ref[:atIdx]
	}
	if colonIdx := strings.LastIndex(ref, ":"); colonIdx != -1 {
		// Only strip if the colon is after the last slash (tag separator, not port).
		if slashIdx := strings.LastIndex(ref, "/"); colonIdx > slashIdx {
			ref = ref[:colonIdx]
		}
	}

	// Split by slashes.
	parts := strings.Split(ref, "/")

	// If there's only one part (e.g., "alpine"), it's a Docker Hub image.
	if len(parts) == 1 {
		return "docker.io"
	}

	// The first part is the registry if it contains a dot or colon (port),
	// or is "localhost".
	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}

	// Otherwise it's a Docker Hub user/image (e.g., "library/alpine", "user/repo").
	return "docker.io"
}

// isRegistryWhitelisted checks if the given registry is in the whitelist.
func isRegistryWhitelisted(registry string, whitelist []string) bool {
	for _, allowed := range whitelist {
		if strings.EqualFold(registry, allowed) {
			return true
		}
	}
	return false
}
