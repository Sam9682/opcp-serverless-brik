package executor

import (
	"fmt"
	"strings"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/config"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

const (
	defaultCPULimit    = 1.0
	defaultMemoryMB    = 256
	bytesPerMB         = 1048576
	defaultUser        = "1000:1000"
	secretMountBaseDir = "/run/secrets"
)

// BuildContainerConfig maps a JobRequest and system config into a ContainerConfig
// suitable for passing to the container runtime.
func BuildContainerConfig(req *JobRequest, cfg *config.Config) *runtime.ContainerConfig {
	cc := &runtime.ContainerConfig{
		Image:      req.Image,
		ReadOnlyFS: true,
		User:       defaultUser,
		CapDrop:    []string{"ALL"},
	}

	// CPU limit: use request value or default.
	if req.CPULimit != nil {
		cc.CPULimit = *req.CPULimit
	} else {
		cc.CPULimit = defaultCPULimit
	}

	// Memory limit: convert MB to bytes.
	if req.MemoryLimitMB != nil {
		cc.MemoryLimit = int64(*req.MemoryLimitMB) * bytesPerMB
	} else {
		cc.MemoryLimit = int64(defaultMemoryMB) * bytesPerMB
	}

	// Network: NetworkOff is the negation of NetworkAccess.
	cc.NetworkOff = !req.NetworkAccess

	// Command: parse the command string into a slice.
	cc.Command = parseCommand(req.Command)

	// Environment variables from request.
	var env []string
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Secrets: env-var secrets go into Env, file-based secrets go into Mounts.
	var mounts []runtime.Mount
	for _, s := range req.Secrets {
		if s.FilePath != nil && *s.FilePath != "" {
			// File-based secret: mount as read-only file.
			mounts = append(mounts, runtime.Mount{
				Source:   fmt.Sprintf("%s/%s", secretMountBaseDir, s.Name),
				Target:   *s.FilePath,
				ReadOnly: true,
			})
		} else {
			// Env-var secret: add as NAME=VALUE to environment.
			env = append(env, fmt.Sprintf("%s=%s", s.Name, s.Value))
		}
	}

	cc.Env = env
	cc.Mounts = mounts

	return cc
}

// EffectiveTimeout computes the effective timeout for a job request according to:
// - If request specifies timeout_seconds and it is ≤ MaxTimeout: use that value
// - If request specifies timeout_seconds > MaxTimeout: clamp to MaxTimeout
// - If no timeout is specified: use DefaultTimeout from config
func EffectiveTimeout(req *JobRequest, cfg *config.Config) time.Duration {
	if req.TimeoutSeconds == nil {
		return cfg.DefaultTimeout
	}

	requested := time.Duration(*req.TimeoutSeconds) * time.Second
	if requested > cfg.MaxTimeout {
		return cfg.MaxTimeout
	}
	return requested
}

// parseCommand splits a command string into a slice of arguments.
// It performs basic shell-like parsing respecting single and double quotes.
func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch {
		case ch == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote
		case ch == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote
		case ch == ' ' && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
