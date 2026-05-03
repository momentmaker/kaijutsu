// Package sign wraps the cosign CLI so jutsu can verify Sigstore-signed
// skills. v0 implementation: detect cosign presence and emit advisory
// notices. Real verify-blob enforcement arrives once Stage 5's
// sign-core.yml workflow publishes signature bundles for skills/core.
package sign

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var (
	// ErrCosignNotFound is returned when cosign isn't on PATH.
	ErrCosignNotFound = errors.New("cosign not found in PATH")
	// ErrNotImplemented is returned by VerifyBlob until Stage 5's signing
	// workflow lands and signatures become fetchable as release assets.
	ErrNotImplemented = errors.New("signature verification stub: signed releases not yet published")
)

// Available reports whether the cosign binary is installed.
func Available() bool {
	_, err := exec.LookPath("cosign")
	return err == nil
}

// VerifyBlob shells out to cosign verify-blob with keyless OIDC parameters.
// blobPath must already be on disk; bundlePath is a Sigstore bundle (.sig).
// expectedIdentity should be a regexp covering the GitHub-issued workflow
// identity (e.g. "https://github.com/momentmaker/kaijutsu/.github/.*").
//
// v0 placeholder: returns ErrNotImplemented until publish/sign workflow
// is producing real bundles for the user to verify against.
func VerifyBlob(ctx context.Context, blobPath, bundlePath, expectedIdentity, oidcIssuer string) error {
	if !Available() {
		return ErrCosignNotFound
	}
	if _, err := os.Stat(bundlePath); err != nil {
		return ErrNotImplemented
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
