package stream

import (
	"context"
	"testing"
	"time"
)

func TestLogBuffer_WriteAndSubscribe(t *testing.T) {
	buf := NewLogBuffer()

	// Write some lines before subscribing.
	buf.Write("stdout", "line1")
	buf.Write("stderr", "line2")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := buf.Subscribe(ctx)

	// Should receive existing lines.
	line := <-ch
	if line.Stream != "stdout" || line.Data != "line1" {
		t.Fatalf("expected stdout/line1, got %s/%s", line.Stream, line.Data)
	}
	line = <-ch
	if line.Stream != "stderr" || line.Data != "line2" {
		t.Fatalf("expected stderr/line2, got %s/%s", line.Stream, line.Data)
	}

	// Write after subscribe.
	buf.Write("stdout", "line3")
	line = <-ch
	if line.Stream != "stdout" || line.Data != "line3" {
		t.Fatalf("expected stdout/line3, got %s/%s", line.Stream, line.Data)
	}

	// Close the buffer.
	buf.Close()

	// Channel should be closed after draining.
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after buffer close")
	}
}

func TestLogBuffer_CloseSignalsSubscribers(t *testing.T) {
	buf := NewLogBuffer()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := buf.Subscribe(ctx)

	// Close immediately.
	buf.Close()

	// Channel should close.
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestLogBuffer_ContextCancellation(t *testing.T) {
	buf := NewLogBuffer()

	ctx, cancel := context.WithCancel(context.Background())
	ch := buf.Subscribe(ctx)

	// Cancel context.
	cancel()

	// Channel should eventually close.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // success
			}
		case <-timeout:
			t.Fatal("channel not closed after context cancel")
		}
	}
}

func TestLogBuffer_WriteAfterClose(t *testing.T) {
	buf := NewLogBuffer()
	buf.Write("stdout", "before")
	buf.Close()
	buf.Write("stdout", "after") // should be ignored

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := buf.Subscribe(ctx)

	line := <-ch
	if line.Data != "before" {
		t.Fatalf("expected 'before', got %s", line.Data)
	}

	// Channel should close (only one line in buffer).
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed, write after close should be ignored")
	}
}

func TestLogBuffer_MultipleSubscribers(t *testing.T) {
	buf := NewLogBuffer()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch1 := buf.Subscribe(ctx)
	ch2 := buf.Subscribe(ctx)

	buf.Write("stdout", "hello")

	line1 := <-ch1
	line2 := <-ch2

	if line1.Data != "hello" || line2.Data != "hello" {
		t.Fatalf("both subscribers should receive 'hello', got %s and %s", line1.Data, line2.Data)
	}

	buf.Close()
}

func TestLogBuffer_TimestampSet(t *testing.T) {
	buf := NewLogBuffer()
	before := time.Now()
	buf.Write("stdout", "test")
	after := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	buf.Close()
	ch := buf.Subscribe(ctx)
	line := <-ch

	if line.Timestamp.Before(before) || line.Timestamp.After(after) {
		t.Fatalf("timestamp %v should be between %v and %v", line.Timestamp, before, after)
	}
}
