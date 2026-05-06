package agents

import (
	"fmt"
	"os"
	"strings"
)

// SanitizeForLog redacts api-key values from a string before it appears
// in stderr / debug output / cache files. Used in error-formatting
// paths to prevent the swarm log from inadvertently exposing a
// credential read out of os.Environ.
//
// Strategy: scan the input for any value that exactly matches the
// current process's environment for known sensitive var-name suffixes
// (`*_API_KEY`, `*_TOKEN`, etc.) and replace each occurrence with
// `<redacted:VAR_NAME>`. Empty-string env values are skipped (nothing
// to redact and false-positive risk on common short strings).
func SanitizeForLog(s string) string {
	if s == "" {
		return s
	}
	for _, e := range os.Environ() {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			continue
		}
		name := e[:eq]
		val := e[eq+1:]
		if val == "" {
			continue
		}
		if !looksSensitive(name) {
			continue
		}
		// Only redact substantial values — short values cause false
		// positives ("KEY=foo" matches every "foo" in the log).
		if len(val) < 8 {
			continue
		}
		s = strings.ReplaceAll(s, val, fmt.Sprintf("<redacted:%s>", name))
	}
	return s
}

// looksSensitive returns true for env-var names that conventionally
// hold secrets. Conservative match: only the well-known suffixes.
func looksSensitive(name string) bool {
	upper := strings.ToUpper(name)
	for _, suffix := range []string{
		"_API_KEY",
		"_TOKEN",
		"_SECRET",
		"_PASSWORD",
		"_PRIVATE_KEY",
	} {
		if strings.HasSuffix(upper, suffix) {
			return true
		}
	}
	// Common bare names.
	switch upper {
	case "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
		"DEEPSEEK_API_KEY", "GLM_API_KEY", "KIMI_API_KEY":
		return true
	}
	return false
}
