package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all worker configuration loaded from environment variables.
type Config struct {
	ListenAddr        string        // default ":8080"
	Runtime           string        // "docker" or "podman"
	RuntimeSocket     string        // auto-detected or explicit
	RegistryWhitelist []string      // comma-separated list
	DefaultTimeout    time.Duration // 300s
	MaxTimeout        time.Duration // 3600s
	MaxOutputBytes    int64         // 1 MB
	ImagePullTimeout  time.Duration // 120s
}

// Load reads configuration from environment variables and returns a validated Config.
// It returns an error if the runtime value is not "docker" or "podman".
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:     envOrDefault("LISTEN_ADDR", ":8080"),
		Runtime:        envOrDefault("RUNTIME", "docker"),
		RuntimeSocket:  os.Getenv("RUNTIME_SOCKET"),
		DefaultTimeout: parseDurationEnv("DEFAULT_TIMEOUT", 300*time.Second),
		MaxTimeout:     parseDurationEnv("MAX_TIMEOUT", 3600*time.Second),
		MaxOutputBytes: parseIntEnv("MAX_OUTPUT_BYTES", 1_048_576),
		ImagePullTimeout: parseDurationEnv("IMAGE_PULL_TIMEOUT", 120*time.Second),
	}

	// Parse registry whitelist from comma-separated string.
	if raw := os.Getenv("REGISTRY_WHITELIST"); raw != "" {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cfg.RegistryWhitelist = append(cfg.RegistryWhitelist, trimmed)
			}
		}
	}

	// Validate runtime value.
	if cfg.Runtime != "docker" && cfg.Runtime != "podman" {
		return nil, fmt.Errorf("invalid RUNTIME value %q: must be \"docker\" or \"podman\"", cfg.Runtime)
	}

	// Auto-detect runtime socket if not explicitly set.
	if cfg.RuntimeSocket == "" {
		switch cfg.Runtime {
		case "docker":
			cfg.RuntimeSocket = "/var/run/docker.sock"
		case "podman":
			cfg.RuntimeSocket = "/run/podman/podman.sock"
		}
	}

	return cfg, nil
}

// envOrDefault returns the environment variable value or a default if unset/empty.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parseDurationEnv parses an environment variable as seconds into a time.Duration.
// Returns the default if the variable is unset or cannot be parsed.
func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultVal
	}
	return time.Duration(seconds) * time.Second
}

// parseIntEnv parses an environment variable as an int64.
// Returns the default if the variable is unset or cannot be parsed.
func parseIntEnv(key string, defaultVal int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
