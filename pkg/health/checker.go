package health

import (
	"context"
	"fmt"
	"time"

	"forgejo.org/opcp-serverless-brik/pkg/executor"
	"forgejo.org/opcp-serverless-brik/pkg/runtime"
)

// Checker verifies runtime connectivity and executor readiness.
type Checker struct {
	runtime runtime.Runtime
	timeout time.Duration
}

// NewChecker creates a new Checker with the given runtime and timeout.
func NewChecker(rt runtime.Runtime, timeout time.Duration) *Checker {
	return &Checker{
		runtime: rt,
		timeout: timeout,
	}
}

// Check verifies connectivity to the container runtime within the configured timeout.
// Returns healthy status if runtime responds to ping, unhealthy otherwise.
func (c *Checker) Check(ctx context.Context) HealthStatus {
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if err := c.runtime.Ping(timeoutCtx); err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Runtime: fmt.Sprintf("runtime unavailable: %v", err),
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Runtime: "connected",
	}
}

// IsReady checks whether the executor can accept new work.
// Returns ready status if the executor is idle, busy with current job ID otherwise.
func (c *Checker) IsReady(exec *executor.Executor) ReadinessStatus {
	if exec.IsReady() {
		return ReadinessStatus{
			Status: "ready",
		}
	}

	return ReadinessStatus{
		Status: "busy",
		JobID:  exec.CurrentJobID(),
	}
}
