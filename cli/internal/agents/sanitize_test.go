package agents

import (
	"strings"
	"testing"
)

func TestSanitizeForLog_RedactsKnownEnvSecrets(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deadbeefcafebabe123456")
	got := SanitizeForLog("config error: token=sk-deadbeefcafebabe123456 expired")
	if strings.Contains(got, "sk-deadbeefcafebabe123456") {
		t.Errorf("secret leaked: %q", got)
	}
	if !strings.Contains(got, "<redacted:DEEPSEEK_API_KEY>") {
		t.Errorf("redaction marker missing: %q", got)
	}
}

func TestSanitizeForLog_RedactsTokenSuffix(t *testing.T) {
	t.Setenv("SEMGREP_TOKEN", "abcdef0123456789xyz")
	got := SanitizeForLog("auth=abcdef0123456789xyz, more=foo")
	if strings.Contains(got, "abcdef0123456789xyz") {
		t.Errorf("token leaked: %q", got)
	}
}

func TestSanitizeForLog_SkipsShortValues(t *testing.T) {
	// Short values cause false positives — sanitize must not redact
	// them. PASSWORD=foo would otherwise replace every "foo" in logs.
	t.Setenv("FAKE_PASSWORD", "foo")
	in := "user said foo bar"
	got := SanitizeForLog(in)
	if got != in {
		t.Errorf("short-value redaction triggered false positive: %q -> %q", in, got)
	}
}

func TestSanitizeForLog_LeavesNonSensitiveAlone(t *testing.T) {
	t.Setenv("MY_NONSENSITIVE_VAR", "longvaluestring12345")
	in := "some text with longvaluestring12345 in it"
	got := SanitizeForLog(in)
	if got != in {
		t.Errorf("non-sensitive var redacted: %q -> %q", in, got)
	}
}

func TestSanitizeForLog_EmptyInput(t *testing.T) {
	if got := SanitizeForLog(""); got != "" {
		t.Errorf("empty input mutated to %q", got)
	}
}
