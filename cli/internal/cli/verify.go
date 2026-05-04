package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/momentmaker/kaijutsu/cli/internal/fetch"
	"github.com/momentmaker/kaijutsu/cli/internal/sign"
	"github.com/momentmaker/kaijutsu/cli/internal/source"
)

// verifySignature enforces the sigstore signature on a loaded skill when
// the skill declares trust.expected-signer. Hard-fails on mismatch,
// missing bundle, or missing cosign. The --no-verify install flag short-
// circuits this check (caller skips calling verifySignature entirely).
//
// Identity translation: skill.yaml encodes shorthand like
//
//	expected-signer: kaijutsu-core@github
//
// which the CLI maps to the GitHub Actions OIDC identity regex pattern
// matching the sign-core.yml workflow on the same repo. Currently only
// kaijutsu-core@github is supported; v0.4 will accept arbitrary
// <org-or-user>@github mappings for community signers.
func verifySignature(ctx context.Context, stderr io.Writer, fetcher *fetch.Fetcher, l *loaded) error {
	if l.skill.Trust == nil || l.skill.Trust.ExpectedSigner == "" {
		return nil
	}
	if l.source == "local" {
		// Local registry mode: sig bundles only exist for remote tagged
		// releases.
		return nil
	}
	if l.tag == "" {
		// No tag context — either default-branch HEAD fallback (no
		// matching semver tag) or a lockfile sync (tag isn't stored
		// in the v1 lockfile schema; v0.4 will add a tag field).
		// Surface that verify was skipped despite expected-signer.
		if stderr != nil {
			fmt.Fprintf(stderr, "warning: skill %s declares expected-signer %q but no tag context is available (likely lockfile sync); signature verification skipped. v0.4 will store the tag in the lockfile to close this gap.\n",
				l.skill.Name, l.skill.Trust.ExpectedSigner)
		}
		return nil
	}
	if !sign.Available() {
		return fmt.Errorf("skill %s requires signature verification (expected-signer: %s) but cosign is not on PATH; install cosign (`brew install cosign` / `go install github.com/sigstore/cosign/v2/cmd/cosign@latest`) or pass --no-verify to bypass at your own risk",
			l.skill.Name, l.skill.Trust.ExpectedSigner)
	}

	identityRegex, oidcIssuer, err := identityForSigner(l.skill.Trust.ExpectedSigner)
	if err != nil {
		return fmt.Errorf("skill %s: %w", l.skill.Name, err)
	}

	// sign-core.yml uploads two assets per skill: <name>-<tag>.tar.gz
	// (the per-skill tarball that was signed) and <name>-<tag>.tar.gz.sig
	// (the Sigstore bundle). cosign verify-blob compares the signed
	// tarball against its bundle — so we fetch BOTH and verify them as
	// a pair. Note: this is a different tarball from the codeload
	// archive used for actual install. Verifying the asset proves the
	// kaijutsu-core workflow signed *a* tarball of skills/core/<name>/
	// at this tag; install proceeds with the codeload mirror at the
	// same commit SHA, which carries GitHub's authoritative integrity.
	src, err := source.Parse(l.source)
	if err != nil {
		return fmt.Errorf("skill %s: parse source: %w", l.skill.Name, err)
	}
	tarballName := fmt.Sprintf("%s-%s.tar.gz", l.skill.Name, l.tag)
	bundleName := tarballName + ".sig"

	tarballBytes, err := fetcher.GetReleaseAsset(ctx, src, l.tag, tarballName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("skill %s declares expected-signer %q but no signed tarball found at release %s asset %s; refusing to install. Pass --no-verify to bypass at your own risk",
				l.skill.Name, l.skill.Trust.ExpectedSigner, l.tag, tarballName)
		}
		return fmt.Errorf("skill %s: fetch signed tarball: %w", l.skill.Name, err)
	}
	bundleBytes, err := fetcher.GetReleaseAsset(ctx, src, l.tag, bundleName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("skill %s declares expected-signer %q but no signature bundle found at release %s asset %s; refusing to install. Pass --no-verify to bypass at your own risk",
				l.skill.Name, l.skill.Trust.ExpectedSigner, l.tag, bundleName)
		}
		return fmt.Errorf("skill %s: fetch sig bundle: %w", l.skill.Name, err)
	}

	tmpDir, err := os.MkdirTemp("", "kaijutsu-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tarballPath := filepath.Join(tmpDir, tarballName)
	bundlePath := filepath.Join(tmpDir, bundleName)
	if err := os.WriteFile(tarballPath, tarballBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(bundlePath, bundleBytes, 0644); err != nil {
		return err
	}

	if err := sign.VerifyBlob(ctx, tarballPath, bundlePath, identityRegex, oidcIssuer); err != nil {
		return fmt.Errorf("skill %s: signature verification FAILED: %w", l.skill.Name, err)
	}

	if stderr != nil {
		fmt.Fprintf(stderr, "ok    sigstore verified: %s@%s signed by %s\n", l.skill.Name, l.tag, l.skill.Trust.ExpectedSigner)
	}
	return nil
}

// identityForSigner maps the skill.yaml shorthand for the expected
// signer to the GitHub Actions OIDC identity regex + issuer that
// cosign verify-blob compares against. Currently supports only
// kaijutsu-core; v0.4 generalizes to community signers.
func identityForSigner(signer string) (identityRegex, oidcIssuer string, err error) {
	switch signer {
	case "kaijutsu-core@github":
		// sign-core.yml at momentmaker/kaijutsu signs every release.
		// Accept either tag-context or main-context identities — sign-core
		// is triggered via workflow_run which runs from main even though
		// the underlying push is a tag, so the issued OIDC identity carries
		// `@refs/heads/main` instead of `@refs/tags/<tag>`.
		return `^https://github\.com/momentmaker/kaijutsu/\.github/workflows/sign-core\.yml@refs/(heads/main|tags/.*)$`,
			"https://token.actions.githubusercontent.com", nil
	}
	if strings.HasSuffix(signer, "@github") {
		return "", "", fmt.Errorf("unknown expected-signer %q (only kaijutsu-core@github recognized in v0.3; community signer mapping arrives in v0.4)", signer)
	}
	return "", "", fmt.Errorf("malformed expected-signer %q (expected '<identifier>@github')", signer)
}
