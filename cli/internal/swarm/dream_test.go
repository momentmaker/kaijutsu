package swarm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDreamPreset_Registered verifies the dream preset is in the
// default registry — `jutsu swarm dream` resolution depends on this.
func TestDreamPreset_Registered(t *testing.T) {
	p, err := PresetFor("dream")
	if err != nil {
		t.Fatalf("dream preset not registered: %v", err)
	}
	if p.Name != "dream" {
		t.Fatalf("preset.Name = %q, want dream", p.Name)
	}
	if p.InputKind != InputPrompt {
		t.Errorf("dream InputKind = %v, want InputPrompt", p.InputKind)
	}
}

// TestDreamLensesBase covers the canonical 4-lens default + LEAD
// position. The first element is the LEAD lens for v0.8.0's
// rotation rule (Stage 3); test pins the order.
func TestDreamLensesBase(t *testing.T) {
	want := []string{"honest", "fit", "gaps", "wild"}
	got := DreamLensesBase()
	if len(got) != len(want) {
		t.Fatalf("DreamLensesBase len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DreamLensesBase[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDreamLensesAll verifies the 8-lens superset and ordering: base
// 4 first, extras 4 after. Ordering matters for the LEAD-rotation
// rule + for stable prompt assembly.
func TestDreamLensesAll(t *testing.T) {
	want := []string{"honest", "fit", "gaps", "wild", "adversary", "inverse", "status-quo", "time"}
	got := DreamLensesAll()
	if len(got) != len(want) {
		t.Fatalf("DreamLensesAll len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DreamLensesAll[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestIsValidDreamLens covers the 8 known + a handful of typos.
// `null` is intentionally invalid (was renamed to `status-quo` in
// the doc-review squash to dodge JSON/YAML/Go reserved-word
// collisions).
func TestIsValidDreamLens(t *testing.T) {
	for _, name := range DreamLensesAll() {
		if !IsValidDreamLens(name) {
			t.Errorf("IsValidDreamLens(%q) = false, want true", name)
		}
	}
	for _, bad := range []string{"null", "honesty", "Wild", "", "premortem"} {
		if IsValidDreamLens(bad) {
			t.Errorf("IsValidDreamLens(%q) = true, want false (typo / reserved-word)", bad)
		}
	}
}

// TestBuildDreamPrompt_BaseHasAntiSycophancy guards against accidental
// softening of the anti-sycophancy preamble. The exact phrases live
// in dreamSharedHeader; this test asserts they survive any future
// edit.
func TestBuildDreamPrompt_BaseHasAntiSycophancy(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesBase())
	for _, phrase := range []string{
		`"Great question!"`,
		`"You're absolutely right!"`,
		`"This is interesting"`,
		"hedging that means nothing",
	} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("dream prompt missing anti-sycophancy phrase %q — preamble was softened", phrase)
		}
	}
}

// TestBuildDreamPrompt_ContainsLensFooter verifies the trailing
// fmt.Sprintf placeholder is present so the swarm dispatch path can
// substitute the topic via fmt.Sprintf(template, topic).
func TestBuildDreamPrompt_ContainsLensFooter(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesBase())
	if !strings.Contains(prompt, "TOPIC:") {
		t.Errorf("dream prompt missing TOPIC: marker:\n%s", prompt[:min(len(prompt), 200)])
	}
	// Single %s — the topic is substituted exactly once.
	if c := strings.Count(prompt, "%s"); c != 1 {
		t.Errorf("dream prompt has %d %%s placeholders, want exactly 1", c)
	}
}

// TestBuildDreamPrompt_LensSelection verifies that --lenses honest,
// gaps produces a prompt with ONLY those two lenses' content blocks.
func TestBuildDreamPrompt_LensSelection(t *testing.T) {
	prompt := BuildDreamPrompt([]string{"honest", "gaps"})
	if !strings.Contains(prompt, "LENS: honest") {
		t.Error("expected LENS: honest in prompt")
	}
	if !strings.Contains(prompt, "LENS: gaps") {
		t.Error("expected LENS: gaps in prompt")
	}
	for _, missing := range []string{"LENS: fit", "LENS: wild", "LENS: adversary"} {
		if strings.Contains(prompt, missing) {
			t.Errorf("prompt should NOT contain %q for honest+gaps selection", missing)
		}
	}
}

// TestBuildDreamPrompt_AllLensesPresent verifies the all-8 path
// includes every lens content block + the matrix-ready header.
func TestBuildDreamPrompt_AllLensesPresent(t *testing.T) {
	prompt := BuildDreamPrompt(DreamLensesAll())
	for _, lens := range DreamLensesAll() {
		want := "LENS: " + lens
		if !strings.Contains(prompt, want) {
			t.Errorf("all-lenses prompt missing %q", want)
		}
	}
}

// TestDreamPreset_SeverityVocabIsKaijutsuStandard verifies dream uses
// the standard kaijutsu vocabulary (blocker / issue / minor / info)
// rather than inventing a dream-specific severity set. v0.7 findings
// DB schema validation depends on this.
func TestDreamPreset_SeverityVocabIsKaijutsuStandard(t *testing.T) {
	want := map[Severity]bool{
		SeverityBlocker: true,
		SeverityIssue:   true,
		SeverityMinor:   true,
		SeverityInfo:    true,
	}
	got := map[Severity]bool{}
	for _, s := range dreamPreset.SeverityVocab {
		got[s] = true
	}
	if len(want) != len(got) {
		t.Fatalf("dream SeverityVocab len = %d, want %d", len(got), len(want))
	}
	for s := range want {
		if !got[s] {
			t.Errorf("dream SeverityVocab missing %q", s)
		}
	}
}

// TestDreamSynthesizer_ContainsHardStop guards against accidental
// removal of the synthesizer's hard-stop rule that prevents the
// trailing-coda failure mode.
func TestDreamSynthesizer_ContainsHardStop(t *testing.T) {
	if !strings.Contains(dreamSynthesizer, "HARD STOP RULE") {
		t.Error("dream synthesizer missing 'HARD STOP RULE' — coda guardrail softened")
	}
	if !strings.Contains(dreamSynthesizer, "End the output at the last section") {
		t.Error("dream synthesizer missing the end-at-last-section directive")
	}
}

// TestBuildDreamPrompt_FmtSafe is a regression test for the
// %!s(MISSING) bug surfaced by the first real `swarm dream` session.
// Literal `%` characters inside lens prompts must be doubled (`%%`)
// so fmt.Sprintf doesn't interpret them as format directives. The
// test renders the assembled prompt with a benign topic and asserts
// no fmt error markers leak through.
func TestBuildDreamPrompt_FmtSafe(t *testing.T) {
	for _, set := range [][]string{DreamLensesBase(), DreamLensesAll()} {
		tmpl := BuildDreamPrompt(set)
		// Single %s placeholder is the topic substitution slot. Pass
		// a fixed topic; if any other %-directive remains in the body,
		// fmt.Sprintf returns "%!<verb>(MISSING)" or similar markers.
		out := fmt.Sprintf(tmpl, "test topic")
		for _, marker := range []string{"%!", "(MISSING)", "(BADINDEX)"} {
			if strings.Contains(out, marker) {
				t.Errorf("BuildDreamPrompt(%v) → fmt.Sprintf produced marker %q (unescaped %% in a lens body?). Excerpt:\n%s",
					set, marker, excerpt(out, marker))
			}
		}
	}
}

func excerpt(s, marker string) string {
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	start := i - 80
	if start < 0 {
		start = 0
	}
	end := i + 80
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

// TestStripDreamCoda_RemovesTrailingParagraph covers the v0.8.3
// programmatic backstop for the synthesizer HARD STOP rule. Models
// occasionally emit a closing "In summary" / "Overall" paragraph
// despite the prompt instruction; the strip catches them.
func TestStripDreamCoda_RemovesTrailingParagraph(t *testing.T) {
	draft := `### 4. Lens-blind-spots

- all-agents-agreed warning: honest — model-shared bias.

In summary, the dream surfaced strong signal across all four lenses.
This is a great topic to explore further.`

	got := StripDreamCoda(draft)
	if strings.Contains(got, "In summary") {
		t.Errorf("coda 'In summary' paragraph not stripped:\n%s", got)
	}
	if !strings.Contains(got, "model-shared bias") {
		t.Errorf("legitimate section content was stripped:\n%s", got)
	}
}

// TestStripDreamCoda_PreservesTablesAndCodeBlocks verifies the
// strip ignores markdown table rows and fenced code blocks even
// when they contain coda-like phrases.
func TestStripDreamCoda_PreservesTablesAndCodeBlocks(t *testing.T) {
	draft := `### 1. Load-bearing insights

| Lens | Insight | Conf |
|------|---------|------|
| honest | "In summary" was caught as coda by the test | 0.9 |
| gaps | But this row is INSIDE a table, not a coda paragraph | 0.8 |

` + "```" + `
overall, this code block also mentions overall but is fenced
` + "```" + `

### 2. Cross-lens consensus

- legitimate bullet content`

	got := StripDreamCoda(draft)
	if !strings.Contains(got, `"In summary"`) {
		t.Error("table cell containing 'In summary' was wrongly stripped")
	}
	if !strings.Contains(got, "overall, this code block") {
		t.Error("code block containing 'overall' was wrongly stripped")
	}
	if !strings.Contains(got, "legitimate bullet content") {
		t.Error("section after preserved coda-like words was lost")
	}
}

// TestStripDreamCoda_TakesLastOccurrence guards against the v0.8.3
// adversarial-review BLOCKER: an "Overall" used earlier as a
// transitional marker shouldn't truncate legitimate later content.
// Strip should target the LAST coda phrase, not the first.
func TestStripDreamCoda_TakesLastOccurrence(t *testing.T) {
	draft := `### 2. Cross-lens consensus

Overall ranking: gemini's lens-fit lens converged with claude's gaps lens.

### 3. Lens-unique findings

- legitimate finding 1
- legitimate finding 2

### 4. Lens-blind-spots

- model-shared bias on honest lens

In summary, here's the takeaway paragraph the model wasn't supposed to write.`

	got := StripDreamCoda(draft)
	if !strings.Contains(got, "legitimate finding 1") {
		t.Errorf("legitimate content lost (truncated at FIRST 'Overall' instead of LAST 'In summary'):\n%s", got)
	}
	if !strings.Contains(got, "model-shared bias") {
		t.Errorf("section 4 content lost:\n%s", got)
	}
	if strings.Contains(got, "In summary") {
		t.Errorf("trailing coda not stripped:\n%s", got)
	}
}

// TestStripDreamCoda_NoOpWhenNoCoda verifies clean input passes
// through unchanged. Most well-behaved dream outputs hit this path.
func TestStripDreamCoda_NoOpWhenNoCoda(t *testing.T) {
	draft := `### 4. Lens-blind-spots

- all-agents-agreed warning: honest — bias signal
- all-agents-agreed warning: fit — convergence on docs source
`
	got := StripDreamCoda(draft)
	if got != draft {
		t.Errorf("clean input was modified:\nbefore:\n%s\nafter:\n%s", draft, got)
	}
}

// TestStripDreamCoda_CatchesMultipleOpeners covers the various coda
// phrases models use. Each should trigger the strip.
func TestStripDreamCoda_CatchesMultipleOpeners(t *testing.T) {
	codaPhrases := []string{
		"In summary, here's what we found.",
		"Overall, this is a strong idea.",
		"In conclusion, the lenses converge.",
		"Let me know if you want to dig deeper.",
		"Hope this helps you decide.",
		"Feel free to ask follow-up questions.",
	}
	for _, coda := range codaPhrases {
		draft := "### 4. Lens-blind-spots\n\n- legit content\n\n" + coda
		got := StripDreamCoda(draft)
		if strings.Contains(got, coda) {
			t.Errorf("coda phrase %q was not stripped", coda)
		}
	}
}

// TestSkillPrompts_AntiSycophancyPerFile is the v0.8.3 followup to
// TestBuildDreamPrompt_BaseHasAntiSycophancy. The Go-side test only
// covers the assembled swarm-preset prompt; this one walks each
// markdown file in skills/core/dream/prompts/{base,extras} and
// asserts the anti-sycophancy forbid list is present in EACH file.
//
// Standalone /dream invocations load these markdown files directly,
// skipping the Go preset path. Without this guard, a future edit
// could soften (or accidentally drop) the anti-sycophancy preamble
// in any single lens prompt without breaking the Go-side test.
func TestSkillPrompts_AntiSycophancyPerFile(t *testing.T) {
	root := repoRootFromTest(t)
	dirs := []string{
		filepath.Join(root, "skills", "core", "dream", "prompts", "base"),
		filepath.Join(root, "skills", "core", "dream", "prompts", "extras"),
	}
	requiredPhrases := []string{
		`"Great question!"`,
		`"You're absolutely right!"`,
		`"This is interesting"`,
		`"Could be worth considering"`,
	}

	checked := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			content := string(body)
			for _, phrase := range requiredPhrases {
				if !strings.Contains(content, phrase) {
					t.Errorf("%s: missing required forbid phrase %q (anti-sycophancy preamble was softened)", path, phrase)
				}
			}
			checked++
		}
	}
	if checked < 8 {
		t.Errorf("expected ≥8 prompt files (4 base + 4 extras), got %d", checked)
	}
}

// repoRootFromTest derives the repo root path from the test file's
// own location. Walks up looking for the skills/ directory + cli/
// go.mod marker. Avoids relying on cwd, which differs between local
// `go test` (package dir) and CI (repo root).
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		hasSkills := stat(filepath.Join(dir, "skills", "core")) != nil
		hasCli := stat(filepath.Join(dir, "cli", "go.mod")) != nil
		if hasSkills && hasCli {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("walked to filesystem root without finding repo markers")
		}
		dir = parent
	}
	t.Fatal("repo root walk depth exceeded")
	return ""
}

func stat(p string) os.FileInfo {
	st, err := os.Stat(p)
	if err != nil {
		return nil
	}
	return st
}

// TestDreamPreset_DebatePlaceholderSafe verifies dreamPreset.Debate is
// a non-empty template that instructs agents to return an empty
// array. Library callers bypassing the cobra reject would otherwise
// dispatch agents with an empty prompt → wasted API spend on garbage
// output. The placeholder is the safety net.
func TestDreamPreset_DebatePlaceholderSafe(t *testing.T) {
	if dreamPreset.Debate == "" {
		t.Fatal("dreamPreset.Debate is empty — library callers would dispatch empty prompts to agents")
	}
	if !strings.Contains(dreamPreset.Debate, "[]") {
		t.Error("debate placeholder should instruct agents to return [] explicitly")
	}
	// Two %s placeholders match swarm.Debate's fmt.Sprintf signature
	// (own findings JSON + peers' findings JSON).
	if c := strings.Count(dreamPreset.Debate, "%s"); c != 2 {
		t.Errorf("debate template has %d %%s placeholders, want 2 (matches swarm.Debate fmt signature)", c)
	}
}
