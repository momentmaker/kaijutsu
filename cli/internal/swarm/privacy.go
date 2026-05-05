package swarm

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SecretsHit pairs a sensitive path/pattern with a short reason for
// the user-facing block message.
type SecretsHit struct {
	Match  string // file path or matched substring
	Reason string // why it was flagged
}

// SecretsScan walks a unified diff and returns hits for files that
// look like secrets containers (.env*, *.pem, id_rsa*, etc.) plus
// content patterns that look like credentials. Returns no hits when
// the diff is clean. Callers MUST hard-block the run when len(hits)>0
// unless the user passes --allow-secrets.
func SecretsScan(diff string) []SecretsHit {
	var hits []SecretsHit
	hits = append(hits, scanFilenames(diff)...)
	hits = append(hits, scanContent(diff)...)
	return hits
}

var secretFilenameMatchers = []struct {
	match  func(base string) bool
	reason string
}{
	{func(b string) bool { return strings.HasPrefix(b, ".env") }, "dotenv file"},
	{func(b string) bool { return filepath.Ext(b) == ".pem" }, "PEM key/cert"},
	{func(b string) bool { return strings.HasPrefix(b, "id_rsa") || strings.HasPrefix(b, "id_ed25519") || strings.HasPrefix(b, "id_dsa") }, "SSH private key"},
	{func(b string) bool { return b == "credentials" || strings.HasPrefix(b, "credentials.") }, "credentials file"},
	{func(b string) bool { return b == ".npmrc" || b == ".pypirc" || b == ".netrc" }, "package-manager auth file"},
	{func(b string) bool { return strings.HasSuffix(b, ".kubeconfig") || b == "kubeconfig" }, "kubeconfig file"},
	{func(b string) bool { return strings.HasSuffix(b, ".pfx") || strings.HasSuffix(b, ".p12") }, "PKCS#12 keystore"},
}

// scanFilenames walks `diff --git a/X b/X` headers in a unified diff
// and reports any file whose basename matches a secret filename
// pattern.
func scanFilenames(diff string) []SecretsHit {
	var hits []SecretsHit
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		// Format: "diff --git a/path b/path"
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		path := strings.TrimPrefix(fields[3], "b/")
		base := filepath.Base(path)
		for _, m := range secretFilenameMatchers {
			if m.match(base) {
				hits = append(hits, SecretsHit{Match: path, Reason: m.reason})
				break
			}
		}
	}
	return hits
}

var contentPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	// AWS access key id
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "AWS access key id"},
	// GitHub PAT (classic + fine-grained)
	{regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`), "GitHub personal access token"},
	{regexp.MustCompile(`github_pat_[A-Za-z0-9_]{60,}`), "GitHub fine-grained token"},
	// Google API key (covers Gemini API keys — same prefix shape)
	{regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`), "Google API key"},
	// Slack token
	{regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`), "Slack token"},
	// Stripe live secret
	{regexp.MustCompile(`sk_live_[A-Za-z0-9]{24,}`), "Stripe live secret"},
	// Anthropic API key. Real shape is sk-ant-api03-<long>. Match
	// any sk-ant- prefix conservatively to also catch test/staging
	// variants. Important for kaijutsu specifically — accidental
	// leak goes back through claude itself.
	{regexp.MustCompile(`sk-ant-api\d+-[A-Za-z0-9_\-]{20,}`), "Anthropic API key"},
	// OpenAI: project keys (sk-proj-...), service-account keys
	// (sk-svcacct-...), classic user keys (sk-...). Classic shape
	// is sk- followed by ~48 alphanumerics; require 40+ so short
	// tokens like "sk-1" in test fixtures don't false-positive.
	// Anchor classic pattern with a non-word leading char so
	// `task-skXXXX` and similar don't trigger.
	{regexp.MustCompile(`sk-proj-[A-Za-z0-9_\-]{40,}`), "OpenAI project API key"},
	{regexp.MustCompile(`sk-svcacct-[A-Za-z0-9_\-]{40,}`), "OpenAI service-account API key"},
	{regexp.MustCompile(`(?:^|[ \t"'=:;,])sk-[A-Za-z0-9]{40,}`), "OpenAI API key (classic)"},
	// Generic PEM block
	{regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`), "private key block"},
}

// scanContent walks added lines in the diff (lines starting with "+"
// but not "+++") for high-confidence credential patterns. We
// deliberately do not flag every "password=" — too many false
// positives in test fixtures.
func scanContent(diff string) []SecretsHit {
	var hits []SecretsHit
	seen := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		body := line[1:]
		for _, p := range contentPatterns {
			loc := p.re.FindString(body)
			if loc == "" {
				continue
			}
			key := p.reason + "|" + loc
			if seen[key] {
				continue
			}
			seen[key] = true
			hits = append(hits, SecretsHit{Match: loc, Reason: p.reason})
		}
	}
	return hits
}
