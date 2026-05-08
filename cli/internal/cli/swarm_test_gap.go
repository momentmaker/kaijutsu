// swarm_test_gap.go — `jutsu swarm test-gap` cobra wiring. v0.12.
// Surfaces failure scenarios MISSING from existing tests by running
// the multi-agent swarm against (code, tests) pairs.
//
// Cobra layer reads --tests content + bakes into the prompt at
// command time via swarm.BuildTestGapPrompt (mirrors reverse's
// --spec pattern). --code goes through the standard InputFiles
// pipeline as the body that gets substituted into the trailing %s
// slot at dispatch time by ResolveInput.
package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// MaxTestsContentBytes caps the --tests content baked into the
// prompt. 200 KB matches the spec's input cap (Decision #8).
// Test directories can balloon — a deep test/ tree can easily hit
// 1 MB+. Cap forces narrowing via --include or smaller path.
const MaxTestsContentBytes = 200 * 1024

func newSwarmTestGapCmd() *cobra.Command {
	var (
		flags    commonSwarmFlags
		codePath string
		testPath string
	)
	cmd := &cobra.Command{
		Use:   "test-gap",
		Short: "Surface failure scenarios MISSING from existing tests (multi-agent imagines edge cases / races / resource exhaustion / malformed input).",
		Long: `Audits a code-under-test path against an existing test path
(file or directory). Multi-agent swarm imagines plausible failure
scenarios the existing tests do NOT exercise + the synthesizer
clusters by category (edge-case / race / resource / input /
integration / state) + ranks by severity + corroboration count.

Differs from pr-review: pr-review hunts BUGS in code; test-gap
hunts MISSING SCENARIOS in tests. Pair with /polish — polish
ensures tests pass, test-gap ensures they cover.

Example:
  jutsu swarm test-gap --code src/auth/login.go --tests src/auth/login_test.go
  jutsu swarm test-gap --code cli/internal/swarm --tests cli/internal/swarm/

Severity vocab matches pr-review (blocker | issue | minor | info).
Default confidence threshold = 0.30 (matches reverse) — hypothesis-
generation surfaces speculative findings; lower threshold admits
them; the synthesizer's ranking + your filter via the confidence
column do the gatekeeping. Tune via --confidence-threshold.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			projectRoot, _ := os.Getwd()

			preset, err := swarm.LoadPresetWithSkillOverrides(projectRoot, "test-gap")
			if err != nil {
				return err
			}
			if flags.grantConsent {
				return runGrantConsent(cmd, projectRoot, preset)
			}
			if flags.replayKey != "" {
				return runReplay(ctx, cmd, projectRoot, "test-gap", flags.replayKey, flags.synthesizer, flags.perAgentBudget, flags.timeout, false)
			}

			if codePath == "" {
				return errors.New("--code <path> is required")
			}
			if testPath == "" {
				return errors.New("--tests <path> is required (file or directory)")
			}

			testsBody, err := readTestsContent(testPath)
			if err != nil {
				return fmt.Errorf("--tests %s: %w", testPath, err)
			}
			if len(testsBody) > MaxTestsContentBytes {
				return fmt.Errorf("--tests content is %d bytes — exceeds %d-byte cap. Narrow with a smaller path (e.g. a single test file or a tighter subdir)", len(testsBody), MaxTestsContentBytes)
			}

			// Bake tests content into DefaultPrompt at command-time —
			// mirrors reverse's BuildReversePrompt(specContent) pattern.
			// Resulting prompt has ONE %s slot for the code-under-test,
			// substituted by ResolveInput at dispatch time.
			presetCopy := *preset
			presetCopy.DefaultPrompt = swarm.BuildTestGapPrompt(string(testsBody))
			preset = &presetCopy

			ictx, err := swarm.ResolveInput(ctx, preset, swarm.InputOptions{
				Files: []string{codePath},
			})
			if err != nil {
				return err
			}
			return runSwarmPipeline(ctx, cmd, projectRoot, preset, ictx, flags)
		},
	}
	cmd.Flags().StringVar(&codePath, "code", "", "path to the code-under-test (file or directory) — required")
	cmd.Flags().StringVar(&testPath, "tests", "", "path to existing tests (file or directory) — required")
	bindCommonFlags(cmd, &flags, false) // no --post-comment for test-gap
	return cmd
}

// readTestsContent reads --tests content. For a single file, returns
// the file body. For a directory, walks recursively + concatenates
// each file's content with a `// === <relative-path> ===` header so
// the agent can attribute scenarios to specific test files. Skips
// hidden dirs (.git, .kaijutsu, etc.) and non-regular files.
func readTestsContent(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// Always emit the header so single-file + directory paths
		// produce uniformly-shaped prompt content. Mirrors
		// readBugReproFiles convention; gives the agent a stable
		// place to anchor scenario citations regardless of input
		// shape.
		var b []byte
		b = append(b, []byte("// === "+filepath.Base(path)+" ===\n")...)
		b = append(b, body...)
		b = append(b, '\n')
		return b, nil
	}
	var b strings.Builder
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Skip hidden dirs.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		rel, _ := filepath.Rel(path, p)
		fmt.Fprintf(&b, "// === %s ===\n", rel)
		b.Write(body)
		b.WriteString("\n")
		// Early-exit if we've blown past the cap — saves walking
		// remaining files.
		if b.Len() > MaxTestsContentBytes {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}
