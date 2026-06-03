package executor

import (
	"bytes"
	"testing"
)

func TestCaptureOutput_BelowLimit(t *testing.T) {
	stdout := []byte("hello stdout")
	stderr := []byte("hello stderr")
	maxBytes := int64(1048576) // 1 MB

	outStr, errStr, outTrunc, errTrunc := CaptureOutput(stdout, stderr, maxBytes)

	if outStr != "hello stdout" {
		t.Errorf("expected stdout %q, got %q", "hello stdout", outStr)
	}
	if errStr != "hello stderr" {
		t.Errorf("expected stderr %q, got %q", "hello stderr", errStr)
	}
	if outTrunc {
		t.Error("expected outTruncated=false, got true")
	}
	if errTrunc {
		t.Error("expected errTruncated=false, got true")
	}
}

func TestCaptureOutput_ExactlyAtLimit(t *testing.T) {
	maxBytes := int64(10)
	stdout := bytes.Repeat([]byte("a"), 10)
	stderr := bytes.Repeat([]byte("b"), 10)

	outStr, errStr, outTrunc, errTrunc := CaptureOutput(stdout, stderr, maxBytes)

	if outStr != string(stdout) {
		t.Errorf("expected full stdout, got %q", outStr)
	}
	if errStr != string(stderr) {
		t.Errorf("expected full stderr, got %q", errStr)
	}
	if outTrunc {
		t.Error("expected outTruncated=false at exact limit")
	}
	if errTrunc {
		t.Error("expected errTruncated=false at exact limit")
	}
}

func TestCaptureOutput_AboveLimit_ReturnsTail(t *testing.T) {
	maxBytes := int64(5)
	stdout := []byte("0123456789") // 10 bytes, expect tail "56789"
	stderr := []byte("abcdefghij") // 10 bytes, expect tail "fghij"

	outStr, errStr, outTrunc, errTrunc := CaptureOutput(stdout, stderr, maxBytes)

	if outStr != "56789" {
		t.Errorf("expected stdout tail %q, got %q", "56789", outStr)
	}
	if errStr != "fghij" {
		t.Errorf("expected stderr tail %q, got %q", "fghij", errStr)
	}
	if !outTrunc {
		t.Error("expected outTruncated=true, got false")
	}
	if !errTrunc {
		t.Error("expected errTruncated=true, got false")
	}
}

func TestCaptureOutput_EmptyInputs(t *testing.T) {
	outStr, errStr, outTrunc, errTrunc := CaptureOutput(nil, nil, 1048576)

	if outStr != "" {
		t.Errorf("expected empty stdout, got %q", outStr)
	}
	if errStr != "" {
		t.Errorf("expected empty stderr, got %q", errStr)
	}
	if outTrunc {
		t.Error("expected outTruncated=false for nil input")
	}
	if errTrunc {
		t.Error("expected errTruncated=false for nil input")
	}
}

func TestCaptureOutput_OnlyStdoutTruncated(t *testing.T) {
	maxBytes := int64(5)
	stdout := []byte("0123456789") // 10 bytes, exceeds limit
	stderr := []byte("hi")         // 2 bytes, within limit

	outStr, errStr, outTrunc, errTrunc := CaptureOutput(stdout, stderr, maxBytes)

	if outStr != "56789" {
		t.Errorf("expected stdout tail %q, got %q", "56789", outStr)
	}
	if errStr != "hi" {
		t.Errorf("expected full stderr %q, got %q", "hi", errStr)
	}
	if !outTrunc {
		t.Error("expected outTruncated=true")
	}
	if errTrunc {
		t.Error("expected errTruncated=false")
	}
}

func TestCaptureOutput_OnlyStderrTruncated(t *testing.T) {
	maxBytes := int64(5)
	stdout := []byte("ok")         // 2 bytes, within limit
	stderr := []byte("abcdefghij") // 10 bytes, exceeds limit

	outStr, errStr, outTrunc, errTrunc := CaptureOutput(stdout, stderr, maxBytes)

	if outStr != "ok" {
		t.Errorf("expected full stdout %q, got %q", "ok", outStr)
	}
	if errStr != "fghij" {
		t.Errorf("expected stderr tail %q, got %q", "fghij", errStr)
	}
	if outTrunc {
		t.Error("expected outTruncated=false")
	}
	if !errTrunc {
		t.Error("expected errTruncated=true")
	}
}
