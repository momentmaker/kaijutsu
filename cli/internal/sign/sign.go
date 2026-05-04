// Package sign wraps the cosign CLI so jutsu can verify Sigstore-signed
// skills. As of v0.3, install enforces signatures when a skill declares
// trust.expected-signer; cli/internal/cli/verify.go drives the workflow.
package sign

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrCosignNotFound is returned when cosign isn't on PATH.
var ErrCosignNotFound = errors.New("cosign not found in PATH")

// Available reports whether the cosign binary is installed.
func Available() bool {
	_, err := exec.LookPath("cosign")
	return err == nil
}

// VerifyBlob shells out to cosign verify-blob with keyless OIDC parameters.
// blobPath must already be on disk; bundlePath is a Sigstore bundle (.sig).
// expectedIdentity is a regexp covering the GitHub Actions workflow identity
// (e.g. "^https://github\.com/momentmaker/kaijutsu/\.github/workflows/sign-core\.yml@refs/tags/.*$").
// oidcIssuer is the OIDC issuer URL (e.g. "https://token.actions.githubusercontent.com").
//
// Returns nil on a verified signature; returns an error wrapping cosign's
// stderr otherwise so the caller can surface the real failure reason.
func VerifyBlob(ctx context.Context, blobPath, bundlePath, expectedIdentity, oidcIssuer string) error {
	if !Available() {
		return ErrCosignNotFound
	}
	if _, err := os.Stat(blobPath); err != nil {
		return fmt.Errorf("blob %s missing: %w", blobPath, err)
	}
	if _, err := os.Stat(bundlePath); err != nil {
		return fmt.Errorf("bundle %s missing: %w", bundlePath, err)
	}
	cmd := exec.CommandContext(ctx, "cosign", "verify-blob",
		"--bundle", bundlePath,
		"--certificate-identity-regexp", expectedIdentity,
		"--certificate-oidc-issuer", oidcIssuer,
		blobPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign verify-blob: %w: %s", err, string(output))
	}
	return nil
}
