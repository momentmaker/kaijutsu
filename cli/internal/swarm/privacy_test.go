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
