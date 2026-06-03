package runtime

import (
	"fmt"

	"forgejo.org/opcp-serverless-brik/pkg/config"
)

// New creates the appropriate Runtime implementation based on the provided configuration.
// It returns an error if the configured runtime value is not supported.
func New(cfg *config.Config) (Runtime, error) {
	switch cfg.Runtime {
	case "docker":
		return NewDockerRuntime(cfg.RuntimeSocket)
	case "podman":
		return NewPodmanRuntime(cfg.RuntimeSocket)
	default:
		return nil, fmt.Errorf("unsupported runtime: %q", cfg.Runtime)
	}
}
