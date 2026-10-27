package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactDiagnosticSecretFields(t *testing.T) {
	for _, text := range []string{
		`{"client_secret":"topsecret","message":"safe"}`,
		`{"api_key":"prefix\"topsecret","message":"safe"}`,
		"CLIENT_SECRET=topsecret safe",
		"-----BEGIN PRIVATE KEY-----\ntopsecret\n-----END PRIVATE KEY----- safe",
	} {
		got := RedactText(text)
		require.NotContains(t, got, "topsecret")
		require.Contains(t, got, "[REDACTED]")
		require.Contains(t, got, "safe")
	}
}
