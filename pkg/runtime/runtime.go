package runtime

import (
	"context"
	"time"
)

// Runtime defines the interface for container runtime operations.
type Runtime interface {
	Ping(ctx context.Context) error
	PullImage(ctx context.Context, image string, auth *RegistryAuth) error
	ImageExists(ctx context.Context, image string) (bool, error)
	CreateContainer(ctx context.Context, config *ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	WaitContainer(ctx context.Context, id string) (int64, error)
	StopContainer(ctx context.Context, id string, timeout time.Duration) error
	KillContainer(ctx context.Context, id string, signal string) error
	GetLogs(ctx context.Context, id string) (stdout []byte, stderr []byte, err error)
	RemoveContainer(ctx context.Context, id string) error
}
