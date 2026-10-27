package logging

import (
	"regexp"
	"strings"
)

// Diagnostic text is less structured than a slog attribute: it can be a
// configuration file, header dump, environment or multiline credential. Reuse
// the log value scrubber, but fail closed for credential-bearing formats it
// cannot safely delimit. In particular do not guess where YAML block scalars,
// folded values, escaped JSON, cookies or quoted shell values end. Losing a
// stream is preferable to publishing an inherited credential.
var diagnosticSensitiveField = regexp.MustCompile(`(?i)(?:^|[\s"'\\{,;-])(?:[a-z0-9_-]*[_-])?(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|session[_-]?key|authorization|proxy[_-]?authorization|cookie|set-cookie|credentials?|private[_-]?key)\s*["'\\]*\s*[:=]`)

// RedactDiagnosticText is for already byte-bounded complete values, NEVER a
// prefix/tail cut at the storage boundary. RedactText is the shared log helper;
// the additional policy omits a whole value if any sensitive field survives it.
func RedactDiagnosticText(s string) string {
	// A multiline assignment could carry continuation lines that ordinary log
	// patterns cannot associate with the key. Omit the complete stream, even
	// when the first line would have been masked successfully.
	if strings.ContainsAny(s, "\r\n") && diagnosticSensitiveField.MatchString(s) {
		return "[REDACTED: credential-bearing diagnostic omitted]"
	}
	scrubbed := RedactText(s)
	// Existing scrubber output for simple assignments/JSON is safe. Remove its
	// markers only for this detection pass so labels it already handled do not
	// unnecessarily suppress otherwise useful diagnostics.
	safe := regexpSafeMarkers.ReplaceAllString(scrubbed, "")
	if diagnosticSensitiveField.MatchString(safe) || strings.Contains(safe, "-----BEGIN ") {
		return "[REDACTED: credential-bearing diagnostic omitted]"
	}
	return scrubbed
}

var regexpSafeMarkers = regexp.MustCompile(`(?i)(?:"[^"\n]+"\s*:\s*"\[REDACTED\]"|\b[a-z0-9_-]+=\[REDACTED\])`)
