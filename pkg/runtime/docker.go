package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// imagePullTimeout is the maximum duration allowed for pulling an image.
const imagePullTimeout = 120 * time.Second

// DockerRuntime implements Runtime using the Docker Engine API.
type DockerRuntime struct {
	client *client.Client
}

// NewDockerRuntime creates a new DockerRuntime connected to the given socket path.
func NewDockerRuntime(socketPath string) (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+socketPath),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &DockerRuntime{client: cli}, nil
}

func (d *DockerRuntime) Ping(ctx context.Context) error {
	_, err := d.client.Ping(ctx)
	return err
}

func (d *DockerRuntime) PullImage(ctx context.Context, img string, auth *RegistryAuth) error {
	pullCtx, cancel := context.WithTimeout(ctx, imagePullTimeout)
	defer cancel()

	opts := image.PullOptions{}
	if auth != nil && auth.Username != "" {
		authConfig := registry.AuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}
		encoded, err := encodeAuthConfig(authConfig)
		if err != nil {
			return fmt.Errorf("encoding registry auth: %w", err)
		}
		opts.RegistryAuth = encoded
	}

	reader, err := d.client.ImagePull(pullCtx, img, opts)
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", img, err)
	}
	defer reader.Close()

	// Drain the reader to complete the pull operation.
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("reading pull response for %s: %w", img, err)
	}

	return nil
}

func (d *DockerRuntime) ImageExists(ctx context.Context, img string) (bool, error) {
	_, _, err := d.client.ImageInspectWithRaw(ctx, img)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DockerRuntime) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	// Map ContainerConfig to Docker API container config.
	containerCfg := &container.Config{
		Image: config.Image,
		Cmd:   config.Command,
		Env:   config.Env,
		User:  config.User,
	}

	// Build host config with resource limits and security settings.
	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: int64(config.CPULimit * 1e9),
			Memory:   config.MemoryLimit,
		},
		ReadonlyRootfs: config.ReadOnlyFS,
		CapDrop:        config.CapDrop,
	}

	// Disable network if requested.
	if config.NetworkOff {
		hostCfg.NetworkMode = "none"
	}

	// Add mounts for file-based secrets.
	if len(config.Mounts) > 0 {
		mounts := make([]mount.Mount, 0, len(config.Mounts))
		for _, m := range config.Mounts {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   m.Source,
				Target:   m.Target,
				ReadOnly: m.ReadOnly,
			})
		}
		hostCfg.Mounts = mounts
	}

	resp, err := d.client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return resp.ID, nil
}

func (d *DockerRuntime) StartContainer(ctx context.Context, id string) error {
	return d.client.ContainerStart(ctx, id, container.StartOptions{})
}

func (d *DockerRuntime) WaitContainer(ctx context.Context, id string) (int64, error) {
	waitCh, errCh := d.client.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case result := <-waitCh:
		if result.Error != nil {
			return result.StatusCode, fmt.Errorf("container wait error: %s", result.Error.Message)
		}
		return result.StatusCode, nil
	case err := <-errCh:
		return -1, err
	}
}

func (d *DockerRuntime) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	timeoutSeconds := int(timeout.Seconds())
	opts := container.StopOptions{
		Timeout: &timeoutSeconds,
	}
	return d.client.ContainerStop(ctx, id, opts)
}

func (d *DockerRuntime) KillContainer(ctx context.Context, id string, signal string) error {
	return d.client.ContainerKill(ctx, id, signal)
}

func (d *DockerRuntime) GetLogs(ctx context.Context, id string) ([]byte, []byte, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	reader, err := d.client.ContainerLogs(ctx, id, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("getting container logs: %w", err)
	}
	defer reader.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	if err != nil {
		return nil, nil, fmt.Errorf("reading container logs: %w", err)
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

func (d *DockerRuntime) RemoveContainer(ctx context.Context, id string) error {
	return d.client.ContainerRemove(ctx, id, container.RemoveOptions{
		Force: true,
	})
}

// encodeAuthConfig encodes a registry auth config to base64 JSON for the Docker API.
func encodeAuthConfig(authConfig registry.AuthConfig) (string, error) {
	jsonBytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(jsonBytes), nil
}
