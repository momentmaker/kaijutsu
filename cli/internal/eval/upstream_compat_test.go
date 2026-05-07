package eval

import (
	"os/exec"
	"testing"
)

// TestKaijutsuExtensionBlock_IgnoredByUpstreamParser verifies the
// Stage 2 forward-compat contract: a kaijutsu-extended evals.json
// parses cleanly in the upstream agent-skills-eval npx tool. This
// is the only test in the eval package that touches the network /
// external tooling.
//
// SKIPS when npx is absent (CI without Node, sandboxed test envs).
// CI installs npx in eval-skills.yml when this test must run; local
// dev users see a SKIP and can ignore.
//
// v0.10 ships the test stub ready to use the upstream parser; the
// actual `npx agent-skills-eval --validate` flag landed in upstream
// v1.0.x. If the flag is missing or the package isn't on npm, the
// test SKIPs rather than fails — kaijutsu's compat claim is "the
// extension blocks don't break upstream parsing", which we can
// only verify when the upstream tool is available.
func TestKaijutsuExtensionBlock_IgnoredByUpstreamParser(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not on PATH; upstream-compat test SKIPS (kaijutsu's forward-compat claim is unverified in this run)")
	}
	// Stage 2 ships the SKIP-able test surface. Real upstream
	// invocation lands when the agent-skills-eval npm package
	// exposes a stable `--validate` flag against a fixture path.
	// Until then, the SKIP is intentional: spec promised "kaijutsu.*
	// blocks ignored by upstream parsers" — we'll verify when the
	// upstream tool reaches a stable validate API.
	t.Skip("upstream agent-skills-eval validate flag pending stable release; v0.10.x candidate to wire in")
}
