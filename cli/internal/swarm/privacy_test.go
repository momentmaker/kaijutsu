package swarm

import (
	"strings"
	"testing"
)

func TestSecretsScan_DotEnvFilename(t *testing.T) {
	diff := `diff --git a/.env.local b/.env.local
new file mode 100644
+++ b/.env.local
@@ -0,0 +1 @@
+FOO=bar`
	hits := SecretsScan(diff)
	if len(hits) == 0 {
		t.Fatal("expected hit on .env.local filename")
	}
}

func TestSecretsScan_PrivateKeyContent(t *testing.T) {
	diff := `diff --git a/notes.md b/notes.md
+++ b/notes.md
+-----BEGIN RSA PRIVATE KEY-----
+MIIEvQIBADANBgkqhkiG9w0BAQEFAASC...
+-----END RSA PRIVATE KEY-----`
	hits := SecretsScan(diff)
	if len(hits) == 0 {
		t.Fatal("expected hit on PEM block in content")
	}
}

func TestSecretsScan_AnthropicKey(t *testing.T) {
	diff := `diff --git a/cfg.toml b/cfg.toml
+++ b/cfg.toml
+ANTHROPIC_API_KEY=sk-ant-api03-abc123_def-XYZ789-abc123def456ghi789jkl0`
	hits := SecretsScan(diff)
	found := false
	for _, h := range hits {
		if strings.Contains(h.Reason, "Anthropic") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Anthropic key hit, got %+v", hits)
	}
}

func TestSecretsScan_OpenAIProjectKey(t *testing.T) {
	diff := `diff --git a/cfg.toml b/cfg.toml
+++ b/cfg.toml
+OPENAI_KEY=sk-proj-abc123def456_ghi789-jkl012mno345pqr678stu901vwx234yz5`
	hits := SecretsScan(diff)
	found := false
	for _, h := range hits {
		if strings.Contains(h.Reason, "OpenAI project") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected OpenAI project key hit, got %+v", hits)
	}
}

func TestSecretsScan_OpenAIClassicKey(t *testing.T) {
	diff := `diff --git a/cfg.toml b/cfg.toml
+++ b/cfg.toml
+key="sk-Abcd1234EfghIjkl5678MnopQrst9012UvwxYzab3456"`
	hits := SecretsScan(diff)
	found := false
	for _, h := range hits {
		if strings.Contains(h.Reason, "classic") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected OpenAI classic key hit, got %+v", hits)
	}
}

func TestSecretsScan_OpenAIClassicNoFalsePositiveOnEmbedded(t *testing.T) {
	// `sk-` preceded by `-` (a non-delimiter char in the leading
	// anchor `[ \t"'=:;,]`) should NOT match. Real-world shapes
	// this guards against include `task-sk-...`, `repo-sk-...`,
	// kebab-case identifiers that end in -sk- midword.
	diff := `diff --git a/foo.go b/foo.go
+++ b/foo.go
+var taskID = "task-sk-Abcd1234EfghIjkl5678MnopQrst9012UvwxYzab3456"`
	hits := SecretsScan(diff)
	for _, h := range hits {
		if strings.Contains(h.Reason, "classic") {
			t.Errorf("false positive on hyphen-prefixed sk- substring: %+v", h)
		}
	}
}

func TestSecretsScan_AwsAccessKey(t *testing.T) {
	diff := `diff --git a/cfg.toml b/cfg.toml
+++ b/cfg.toml
+aws_key = "AKIAIOSFODNN7EXAMPLE"`
	hits := SecretsScan(diff)
	found := false
	for _, h := range hits {
		if strings.Contains(h.Reason, "AWS") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected AWS access key hit, got %+v", hits)
	}
}

func TestSecretsScan_CleanDiff(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
+++ b/main.go
+package main
+
+func main() { println("hi") }`
	hits := SecretsScan(diff)
	if len(hits) != 0 {
		t.Fatalf("expected no hits on clean diff, got %+v", hits)
	}
}

func TestSecretsScan_IgnoresRemovedLines(t *testing.T) {
	// Only added lines (lines starting with +, but not +++) should
	// be scanned. A removed credential should NOT block.
	diff := `diff --git a/cfg.toml b/cfg.toml
--- a/cfg.toml
+++ b/cfg.toml
-aws_key = "AKIAIOSFODNN7EXAMPLE"
+aws_key = ""`
	hits := scanContent(diff)
	for _, h := range hits {
		if strings.Contains(h.Reason, "AWS") {
			t.Errorf("removal of secret should not flag, got %+v", h)
		}
	}
}

func TestSecretsScan_PEMFile(t *testing.T) {
	diff := `diff --git a/keys/server.pem b/keys/server.pem
new file mode 100644`
	hits := SecretsScan(diff)
	if len(hits) == 0 {
		t.Fatal("expected hit on .pem filename")
	}
}
