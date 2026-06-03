package runtime

// ContainerConfig holds the configuration for creating a container.
type ContainerConfig struct {
	Image       string
	Command     []string
	Env         []string
	CPULimit    float64 // fractional cores
	MemoryLimit int64   // bytes
	NetworkOff  bool
	ReadOnlyFS  bool
	User        string   // e.g. "1000:1000"
	CapDrop     []string // e.g. ["ALL"]
	Mounts      []Mount  // for file-based secrets
}

// Mount represents a file mount into a container.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// RegistryAuth holds credentials for authenticating to a container registry.
type RegistryAuth struct {
	Username string
	Password string
}
