package executor

import (
	"strings"
	"testing"
)

func TestRedactSecrets_EmptyOutput(t *testing.T) {
	secrets := []Secret{{Name: "TOKEN", Value: "abc123"}}
	result := RedactSecrets("", secrets, "***")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRedactSecrets_NoSecrets(t *testing.T) {
	output := "hello world"
	result := RedactSecrets(output, nil, "***")
	if result != output {
		t.Errorf("expected %q, got %q", output, result)
	}
}

func TestRedactSecrets_SingleSecret(t *testing.T) {
	secrets := []Secret{{Name: "TOKEN", Value: "secret123"}}
	output := "The token is secret123 in the output"
	result := RedactSecrets(output, secrets, "***")

	if strings.Contains(result, "secret123") {
		t.Errorf("secret value still present in result: %q", result)
	}
	expected := "The token is *** in the output"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRedactSecrets_MultipleSecrets(t *testing.T) {
	secrets := []Secret{
		{Name: "TOKEN", Value: "abc"},
		{Name: "PASSWORD", Value: "xyz"},
	}
	output := "abc and xyz are secrets"
	result := RedactSecrets(output, secrets, "[REDACTED]")

	if strings.Contains(result, "abc") || strings.Contains(result, "xyz") {
		t.Errorf("secret values still present in result: %q", result)
	}
	expected := "[REDACTED] and [REDACTED] are secrets"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRedactSecrets_OverlappingSecrets(t *testing.T) {
	// "secret" is a substring of "secret123" — longer should be replaced first
	secrets := []Secret{
		{Name: "SHORT", Value: "secret"},
		{Name: "LONG", Value: "secret123"},
	}
	output := "value is secret123 here"
	result := RedactSecrets(output, secrets, "***")

	if strings.Contains(result, "secret") {
		t.Errorf("secret substring still present in result: %q", result)
	}
	expected := "value is *** here"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRedactSecrets_MultipleOccurrences(t *testing.T) {
	secrets := []Secret{{Name: "KEY", Value: "pass"}}
	output := "pass word pass again pass"
	result := RedactSecrets(output, secrets, "*")

	if strings.Contains(result, "pass") {
		t.Errorf("secret value still present in result: %q", result)
	}
	expected := "* word * again *"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRedactSecrets_EmptySecretValue(t *testing.T) {
	secrets := []Secret{{Name: "EMPTY", Value: ""}}
	output := "hello world"
	result := RedactSecrets(output, secrets, "***")
	if result != output {
		t.Errorf("expected %q, got %q", output, result)
	}
}

func TestRedactSecrets_SecretNotInOutput(t *testing.T) {
	secrets := []Secret{{Name: "TOKEN", Value: "nothere"}}
	output := "hello world"
	result := RedactSecrets(output, secrets, "***")
	if result != output {
		t.Errorf("expected %q, got %q", output, result)
	}
}
