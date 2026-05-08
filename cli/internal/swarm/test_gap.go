// test_gap.go — v0.12 Tier A preset. Surfaces failure scenarios
// MISSING from existing tests by running the multi-agent swarm
// against (code-under-test, existing-tests) pairs.
//
// Multi-agent shape is native to this task: imagining missing
// failure modes is exactly what cross-model disagreement amplifies.
// One reviewer thinks of a race; another thinks of malformed input;
// another thinks of resource exhaustion. The synthesizer clusters
// hypotheses and ranks by severity + cross-reviewer corroboration.
//
// Cobra wiring: cli/internal/cli/swarm_test_gap.go binds --code +
// --tests flags, reads file content, bakes the test content into
// the prompt at command time via BuildTestGapPrompt, and lets
// ResolveInput substitute the code-under-test into the trailing %s
// slot at dispatch time.
package swarm

import "strings"

// testGapPreset is the v0.12 missing-test-scenarios preset. The
// cobra layer mutates DefaultPrompt at command time to bake in the
// existing-tests content; this default template is a placeholder
// for library callers that go directly through the registry.
var testGapPreset = Preset{
	Name:          "test-gap",
	Description:   "Surface failure scenarios MISSING from existing tests. Multi-agent imagines edge cases / race conditions / resource exhaustion / malformed input that current tests don't cover.",
	InputKind:     InputFiles,
	SeverityVocab: []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo},
	DefaultPrompt: testGapDefaultPrompt,
	Synthesizer:   testGapSynthesizer,
	Debate:        genericDebateTemplate, // artifact-agnostic — input is tests, not a PR
}

// testGapDefaultPrompt is the placeholder template loaded from the
// registry. The cobra layer overrides this at command-time via
// BuildTestGapPrompt(testsContent) so the agent sees the existing
// tests inline. Library callers that invoke without setup get
// dispatched-only-on-code mode (no existing-tests context, less
// useful but doesn't hard-fail).
const testGapDefaultPrompt = `You are auditing CODE-UNDER-TEST for failure scenarios MISSING
from existing tests, but no existing-tests content was provided.
Treat the input as code-only auditing: surface plausible failure
scenarios the code's structure suggests should be tested, then
return.

Each finding is a JSON object with:
  severity:    blocker | issue | minor | info
  file:        relative path
  line_range:  "42" or "42-58" (or "0" when n/a)
  summary:     "[<CATEGORY>] one-line scenario description"
  reasoning:   1-3 sentences explaining the failure mode + why it
               matters under realistic load
  confidence:  0.0-1.0

CATEGORY tags: edge-case | race | resource | input | integration |
state.

If you find no plausible missing scenarios, return [].
No prose, no code fences, no commentary outside the JSON.

CODE:
%s
`

// BuildTestGapPrompt assembles the per-agent test-gap prompt with
// the supplied existing-tests content baked in. The result has
// exactly ONE %s slot remaining — for the code-under-test, which
// ResolveInput substitutes at dispatch time via fmt.Sprintf.
//
// Two safety steps protect downstream fmt.Sprintf:
//  1. {{TESTS_CONTENT}} placeholder + strings.Replace for the tests
//     content (NOT fmt.Sprintf with two %s) so fmt isn't asked to
//     leave a %s unfilled.
//  2. Literal `%` chars in tests content escaped to `%%` BEFORE
//     replacement — a test file containing `expected: 10%` would
//     otherwise leak into the prompt as `10%` and the code-slot
//     fmt.Sprintf would interpret `% c` (or whatever follows) as a
//     format directive (the v0.8 dreamLensWild pattern).
func BuildTestGapPrompt(testsContent string) string {
	if strings.TrimSpace(testsContent) == "" {
		return testGapDefaultPrompt
	}
	escaped := strings.ReplaceAll(testsContent, "%", "%%")
	return strings.Replace(testGapPromptTemplate, "{{TESTS_CONTENT}}", escaped, 1)
}

// testGapPromptTemplate uses {{TESTS_CONTENT}} as the tests-content
// placeholder (substituted via strings.Replace in BuildTestGapPrompt)
// and ONE %s slot for the code-under-test (substituted via
// fmt.Sprintf at dispatch time by ResolveInput).
const testGapPromptTemplate = `You are auditing for FAILURE SCENARIOS MISSING from existing tests.
Read the EXISTING-TESTS to understand what's already covered. Then
read the CODE-UNDER-TEST and imagine plausible failure modes that
the existing tests do NOT exercise.

Categorize every missing-scenario finding into ONE of:

- edge-case: boundary values, empty inputs, off-by-one, max-size
- race: timing, concurrency, ordering, interleaving
- resource: exhaustion (memory, file descriptors, connections, disk)
- input: malformed, hostile, injection, encoding, null/nil
- integration: cross-component contracts, error propagation, retries
- state: stale, uninitialized, leaked across calls, persistence

For each finding emit a JSON object:
  severity:    blocker | issue | minor | info
  file:        relative path (code-under-test)
  line_range:  "42" or "42-58" (or "0" when n/a — code generally
               implicates a function not a line range)
  summary:     "[<CATEGORY>] one-line scenario description"
  reasoning:   1-3 sentences naming the code structure that suggests
               this scenario is plausible + why the existing tests
               miss it + why it matters under realistic load
  confidence:  0.0-1.0

DO NOT flag:
- Scenarios already covered by existing tests (read the tests
  carefully; partial coverage still counts)
- Purely-theoretical scenarios that can't surface in production
  (heat-death-of-the-universe edge cases)
- Style nits, formatting, or code-quality issues (other presets)

DO flag:
- Concurrency under realistic load (the "happy path" test passes;
  what about 100 concurrent calls?)
- Error propagation gaps (a function returns an error; do callers
  handle the new variant?)
- Resource leaks (defer/cleanup paths not exercised by current tests)
- Boundary values around any explicit limit (max-size, max-depth,
  timeout-just-after, timeout-just-before)
- State that survives across calls when current tests don't reset

If you find no plausible missing scenarios, return [].
No prose, no code fences, no commentary outside the JSON.

EXISTING-TESTS:
{{TESTS_CONTENT}}

CODE-UNDER-TEST:
%s
`

// testGapSynthesizer aggregates per-agent missing-scenario findings
// into a markdown report keyed by category. Output shape mirrors
// pr-review (cluster + table + sections) so existing assemblers
// downstream don't need a new code path. Emphasizes RANKING +
// CLUSTERING (different reviewers may name the same scenario
// differently) over deduplication.
const testGapSynthesizer = `Three reviewers each proposed missing test scenarios for the same
code. Cluster their findings by category (edge-case / race /
resource / input / integration / state). Within a cluster,
different reviewers may have named the same underlying scenario
with different words; treat those as one cluster row with multiple
reviewers credited.

For each cluster output:
- summary: synthesized one-line scenario (use the clearest reviewer
  phrasing, not a verbatim quote)
- category: edge-case / race / resource / input / integration / state
- severity: highest across reviewers in the cluster
- confidence: mean across reviewers (round to 2 decimals)
- reviewers: list of reviewer names that proposed this scenario
- evidence: 1-2 sentences citing the strongest reviewer's reasoning

Rank clusters by:
  1. severity (blocker > issue > minor > info)
  2. reviewer count (3/3 > 2/3 > 1/3 — corroboration bumps trust)
  3. mean confidence

Output format:

## Missing-scenario clusters (top N ranked)

| # | severity | category | reviewers | confidence | summary |
|---|---|---|---|---|---|
| 1 | issue | race | 3/3 | 0.85 | concurrent X may interleave Y |
| 2 | issue | edge-case | 2/3 | 0.72 | empty Z handling untested |
| ... |

### 1. <summary>
- evidence: <1-2 sentences>
- reviewers: <list>

### 2. ...

For runners-up with confidence < 0.30, list as a "speculative" tail
section without full bodies. Do not fabricate findings to pad the list.

If no missing scenarios surfaced, output:

## Missing-scenario clusters

(no plausible missing scenarios surfaced — existing tests appear
comprehensive for the audited code under realistic load)

PER-AGENT FINDINGS:
%s
`
