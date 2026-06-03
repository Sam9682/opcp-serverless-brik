package runtime

// DefaultPodmanSocket is the default Podman socket path.
const DefaultPodmanSocket = "/run/podman/podman.sock"

// Compile-time check that PodmanRuntime implements Runtime.
var _ Runtime = (*PodmanRuntime)(nil)

// PodmanRuntime implements Runtime using the Podman-compatible Docker API.
// Since Podman exposes a Docker-compatible REST API, this embeds DockerRuntime
// and connects to the Podman socket instead.
type PodmanRuntime struct {
	*DockerRuntime
	socketPath string
}

// NewPodmanRuntime creates a new PodmanRuntime connected to the given socket path.
// If socketPath is empty, it defaults to /run/podman/podman.sock.
func NewPodmanRuntime(socketPath string) (*PodmanRuntime, error) {
	if socketPath == "" {
		socketPath = DefaultPodmanSocket
	}
	dr, err := NewDockerRuntime(socketPath)
	if err != nil {
		return nil, err
	}
	return &PodmanRuntime{DockerRuntime: dr, socketPath: socketPath}, nil
}
