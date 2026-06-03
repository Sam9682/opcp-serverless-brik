package executor

import "strings"

// RedactSecrets replaces all occurrences of any secret value in output with the
// specified placeholder. It processes secrets from longest to shortest to ensure
// that no secret value remains as a substring after redaction (a shorter secret
// that is a substring of a longer one won't interfere with the longer replacement).
func RedactSecrets(output string, secrets []Secret, placeholder string) string {
	if len(secrets) == 0 || output == "" {
		return output
	}

	// Sort secrets by value length descending so longer values are replaced first.
	// This prevents partial matches when one secret value is a substring of another.
	sorted := sortSecretsByValueLength(secrets)

	result := output
	for _, s := range sorted {
		if s.Value == "" {
			continue
		}
		result = strings.ReplaceAll(result, s.Value, placeholder)
	}

	return result
}

// sortSecretsByValueLength returns a copy of secrets sorted by value length
// in descending order (longest first).
func sortSecretsByValueLength(secrets []Secret) []Secret {
	sorted := make([]Secret, len(secrets))
	copy(sorted, secrets)

	// Simple insertion sort — secret lists are bounded at 64 entries.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].Value) > len(sorted[j-1].Value); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	return sorted
}
