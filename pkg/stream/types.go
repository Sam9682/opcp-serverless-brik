package stream

import "time"

// LogLine represents a single line of output from a container.
type LogLine struct {
	Timestamp time.Time `json:"ts"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
	Data      string    `json:"data"`
}
