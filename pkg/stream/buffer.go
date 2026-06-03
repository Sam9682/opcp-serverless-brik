package stream

import (
	"context"
	"sync"
	"time"
)

// LogBuffer captures container output lines and supports concurrent subscription.
// Subscribers receive lines from the current read position and block until new
// lines are written or the buffer is closed.
type LogBuffer struct {
	mu     sync.Mutex
	lines  []LogLine
	closed bool
	notify chan struct{}
}

// NewLogBuffer creates a new LogBuffer ready to accept writes and subscriptions.
func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		notify: make(chan struct{}, 1),
	}
}

// Write appends a log line to the buffer and notifies any waiting subscribers.
func (b *LogBuffer) Write(stream string, data string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.lines = append(b.lines, LogLine{
		Timestamp: time.Now(),
		Stream:    stream,
		Data:      data,
	})

	// Signal subscribers that new data is available.
	select {
	case b.notify <- struct{}{}:
	default:
		// Already signalled, no need to send again.
	}
}

// Close marks the buffer as complete. No more writes will be accepted and
// all waiting subscribers will be unblocked.
func (b *LogBuffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	close(b.notify)
}

// Subscribe returns a channel that emits all log lines starting from the
// current buffer position. The channel is closed when the buffer is closed
// or the provided context is cancelled.
func (b *LogBuffer) Subscribe(ctx context.Context) <-chan LogLine {
	ch := make(chan LogLine, 64)

	go func() {
		defer close(ch)

		pos := 0
		for {
			// Read any available lines under the lock.
			b.mu.Lock()
			lines := b.lines[pos:]
			closed := b.closed
			b.mu.Unlock()

			// Send buffered lines to the subscriber.
			for _, line := range lines {
				select {
				case ch <- line:
					pos++
				case <-ctx.Done():
					return
				}
			}

			// If the buffer is closed and we've sent everything, we're done.
			if closed {
				return
			}

			// Wait for new data, closure, or context cancellation.
			select {
			case <-ctx.Done():
				return
			case _, ok := <-b.notify:
				if !ok {
					// Buffer was closed. Drain remaining lines.
					b.mu.Lock()
					remaining := b.lines[pos:]
					b.mu.Unlock()
					for _, line := range remaining {
						select {
						case ch <- line:
						case <-ctx.Done():
							return
						}
					}
					return
				}
			}
		}
	}()

	return ch
}
