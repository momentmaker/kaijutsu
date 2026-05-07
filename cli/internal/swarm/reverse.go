// reverse.go — v0.9 spec-vs-impl drift detector preset.
//
// Reverse takes a SPEC (markdown) + DIFF (PR or branch range) and
// asks "what's in the diff that wasn't authorized? what's in the
// spec that the diff ignored?" Output is a drift table with four
// categories:
//   ADDED      — in diff, not mentioned in spec
//   OMITTED    — in spec, not in diff
//   CHANGED    — spec said one thing; diff did another
//   AMBIGUOUS  — spec was vague; diff made a choice
//
// Reverse runs as a sibling to pr-review (different framing, same
// dispatch + synthesizer pipeline shape). The cobra layer wires
// `--spec <path>` reading + bakes the spec content into the
// per-agent prompt at command time; ResolveInput then substitutes
// the diff into the prompt's trailing %s slot.
//
// Per-agent prompts: all three converge on the same drift task
// since drift detection is preset-shaped (no per-agent specialism
// like pr-review's blunder-hunt vs paranoid-security splits). The
// preset ships ONE shared prompt as DefaultPrompt; per-agent
// variants are an opt-in v1.0+ refinement.
package swarm

import (
	"fmt"
	"sort"
	"strings"
)

// MaxGitHubCommentBytes is the byte cap for posted PR comment
// bodies. GitHub's hard limit is 65,536; we cap at 60,000 with a
// 4 KB safety margin for headers + footers + the kaijutsu marker
// line. Production callers pass this; tests pass smaller caps to
// trigger truncation deterministically without giant fixtures.
const MaxGitHubCommentBytes = 60_000

// ReverseTruncationFooter is the trailing line appended when the
// rendered drift report exceeds MaxGitHubCommentBytes. Stable text
// so callers (sync-pr, future replay) can detect truncation.
const ReverseTruncationFooter = "\n\n_… more findings truncated. Run `jutsu swarm reverse` locally for the full report._\n"

// ReverseConfidenceThreshold is the v0.9 default confidence floor
// for the reverse preset's synthesizer. Lower than pr-review's 0.55
// because drift detection is harder than code review (requires
// reasoning across two artifacts) and false-positive drifts are
// easier to dismiss than false-negative misses.
const ReverseConfidenceThreshold = 0.30

// reversePreset is the v0.9 spec-vs-impl drift detector. The cobra
// layer mutates DefaultPrompt at command-time to bake in the user-
// supplied spec content; this default template is a placeholder for
// library callers that go directly through the registry.
var reversePreset = Preset{
	Name:          "reverse",
	Description:   "Spec-vs-impl drift detector. Compares a spec (--spec) against a diff (--diff). Categories: ADDED, OMITTED, CHANGED, AMBIGUOUS.",
	InputKind:     InputDiff,
	SeverityVocab: []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo},
	DefaultPrompt: reverseDefaultPrompt,
	Synthesizer:   reverseSynthesizer,
	Debate:        prReviewPreset.Debate, // reuse pr-review debate shape; reverse Pass-2 isn't dream-shaped
}

// reverseDefaultPrompt is the placeholder template loaded from the
// registry. The cobra layer overrides this at command-time via
// BuildReversePrompt(specContent) so the agent sees the spec inline.
// If a library caller invokes the preset without setting up the spec
// override, this fallback at least produces a useful per-diff review
// rather than dispatching empty content.
const reverseDefaultPrompt = `You are auditing a code diff for spec-vs-impl drift, but no spec
was provided. Treat the diff as scope-only auditing: surface scope
creep within the diff itself, then return.

Each finding is a JSON object with:
  severity: blocker | issue | minor | info
  file: relative path
  line_range: "42" or "42-58"
  summary: "[<CATEGORY>] one-line drift description"
  reasoning: 1-3 sentence explanation
  confidence: 0.0-1.0

CATEGORY tags: ADDED | OMITTED | CHANGED | AMBIGUOUS. Without a spec,
only ADDED is meaningful. If the diff has no scope-creep concerns,
return [].

DIFF:
%s
`

// BuildReversePrompt assembles the per-agent reverse prompt with the
// supplied spec content baked in. The result has exactly ONE %s slot
// remaining — for the diff, which ResolveInput substitutes at
// dispatch time via fmt.Sprintf.
//
// Two safety steps protect downstream fmt.Sprintf:
//  1. We use {{SPEC_CONTENT}} placeholder + strings.Replace for the
//     spec (NOT fmt.Sprintf with two %s) so fmt isn't asked to leave
//     a %s unfilled.
//  2. We escape literal `%` chars in spec content to `%%` BEFORE
//     replacement. Without this, a spec line like "10% improvement"
//     leaks into the prompt as `10%`, then the diff substitution's
//     fmt.Sprintf interprets `% i` (or whatever follows) as a format
//     directive — surfaces as %!d(MISSING) at runtime. The v0.8
//     dreamLensWild incident in another guise.
//
// Spec content can include kaijutsu's spec-doc convention sections
// (## In scope, ## Out of scope, ## Open questions) — we pass it
// through verbatim and let the model anchor to whatever section
// structure exists. No required structure.
func BuildReversePrompt(specContent string) string {
	if strings.TrimSpace(specContent) == "" {
		return reverseDefaultPrompt
	}
	// Escape literal `%` so the diff-slot fmt.Sprintf downstream
	// doesn't interpret spec content as format directives.
	escaped := strings.ReplaceAll(specContent, "%", "%%")
	return strings.Replace(reversePromptTemplate, "{{SPEC_CONTENT}}", escaped, 1)
}

// reversePromptTemplate uses a literal {{SPEC_CONTENT}} placeholder
// for the spec (substituted via strings.Replace in BuildReversePrompt)
// and ONE %s slot for the diff (substituted via fmt.Sprintf at
// dispatch time by ResolveInput). The two-slot pattern avoids
// fmt.Sprintf's unfilled-arg warning that surfaces as %!s(MISSING)
// in the rendered prompt — the v0.8 dreamLensWild incident.
const reversePromptTemplate = `You are auditing for spec-vs-impl drift. The spec authorized a set
of changes; the diff is what was actually shipped. Your job is to
find the gap between intent and execution.

Categorize every drift finding into ONE of:

- ADDED: in the diff, NOT mentioned in the spec
- OMITTED: in the spec, NOT in the diff
- CHANGED: spec said one thing, diff did a different thing
- AMBIGUOUS: spec was vague, diff made a choice — flag the choice

For each finding emit a JSON object:
  severity:    blocker | issue | minor | info
  file:        relative path (or "spec" for OMITTED findings without a code anchor)
  line_range:  "42" or "42-58" (or "0" when n/a)
  summary:     "[<CATEGORY>] one-line drift description"
  reasoning:   1-3 sentences naming the spec quote (or "n/a" for ADDED)
               + the diff evidence (or "n/a" for OMITTED) + why this
               is drift, not legitimate scope adjustment
  confidence:  0.0-1.0

DO NOT flag:
- Code-quality issues (other presets cover that)
- Style nits or formatting
- Drift that the spec EXPLICITLY blesses ("see also: X" / "out of scope" / "deferred to v0.X")

DO flag:
- Out-of-scope items shipped silently
- Non-goals violated
- Acceptance criteria not met
- Risk-mitigations promised but not implemented
- Implementation choices the spec deliberately left open (AMBIGUOUS
  category) — surface the choice so the human reviewer can ratify
  or push back

Spec-doc convention hint (kaijutsu specs often use these section
anchors; not required): "## In scope", "## Out of scope", "## Open
questions". If you find them, use them as anchors for OMITTED /
ADDED categorization.

If you find no drift, return [].
No prose, no code fences, no commentary outside the JSON.

SPEC:
{{SPEC_CONTENT}}

DIFF:
%s
`

// reverseSynthesizer aggregates per-agent drift findings into a
// markdown report keyed by drift category. Output shape mirrors
// pr-review (cluster + table + sections) so existing assemblers
// downstream don't need a new code path. The category lives in
// the summary field's "[<CATEGORY>]" prefix.
const reverseSynthesizer = `You are synthesizing N agents' drift findings on a spec-vs-impl
audit. Each agent ran the same drift task; their findings may
overlap, disagree, or surface unique drifts.

DO NOT use these phrases:
- "Great question!" / "Excellent point!" / "You're absolutely right!"
- "This is interesting" without specifying WHY
- Hedging: "could be worth considering", "might be a concern"

Required sections, in this order:

### 1. Drift findings by category

Render a markdown table with columns:
| Severity | Category | File:Lines | Summary | Reporters |
|---|---|---|---|---|

Sort: severity DESC (blocker > issue > minor > info), then category
(ADDED, OMITTED, CHANGED, AMBIGUOUS), then File:Lines lexicographic.
Reporters column lists which agents flagged the finding (cross-agent
agreement signal).

Cluster cross-agent matches: if 2+ agents flagged the same drift
(same file/line OR same spec quote in OMITTED), emit ONE row with
all reporter names. Single-reporter findings get their own row.

### 2. Cross-agent disagreements

If different agents categorized the SAME drift differently (one says
ADDED, another says CHANGED), surface the disagreement here with the
competing categories named. Empty section if all categorizations
agreed.

### 3. Single-reporter highlights

Drifts surfaced by exactly one agent — may be noise OR may be the one
perspective the others missed. List them with the reporter named
inline.

HARD STOP RULE. End the output at the last section. Do NOT write a
closing paragraph or summary.

REVIEWERS' FINDINGS:
%s
`

// MarkerReverse builds the closing-line HTML comment marker for a
// reverse-preset PR comment. Distinct from the pr-review marker so
// sync-pr (Stage 5) can filter by marker prefix and ignore reverse
// comments — reverse + pr-review on the same PR coexist with
// separate comment blocks.
func MarkerReverse(runID, sha string) string {
	return fmt.Sprintf("<!-- kaijutsu-reverse:run-id=%s sha=%s -->", runID, sha)
}

// ReverseTruncate enforces the GitHub comment-body byte cap by
// dropping lowest-priority findings until the rendered output fits
// under capBytes. Returns the truncated finding set and the count
// of findings that were dropped.
//
// Determinism rule: ordering is (severity, file_path, line_range_start)
// — severity primary (blocker > issue > minor > info), file_path
// lexicographic for ties, line_range_start numeric for further ties.
// Same input always produces the same truncation point so sync-pr +
// replay see stable output.
//
// Production callers pass MaxGitHubCommentBytes; tests pass smaller
// caps to trigger truncation deterministically without giant
// fixtures.
func ReverseTruncate(findings []Finding, capBytes int) ([]Finding, int) {
	if capBytes <= 0 || len(findings) == 0 {
		return findings, 0
	}
	sorted := make([]Finding, len(findings))
	copy(sorted, findings)
	// severityRank in synth.go ranks blocker=4, info=0 (highest first
	// in DESC sort). Comparator returns true when i should sort
	// BEFORE j → "i is higher severity" maps to "i.rank > j.rank".
	sort.SliceStable(sorted, func(i, j int) bool {
		if rank := severityRank(sorted[i].Severity) - severityRank(sorted[j].Severity); rank != 0 {
			return rank > 0
		}
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return lineRangeStart(sorted[i].LineRange) < lineRangeStart(sorted[j].LineRange)
	})

	// Walk the sorted list accumulating bytes. Stop at the index
	// whose inclusion would breach capBytes. The byte-size estimate
	// uses Finding's serialized JSON form as a proxy for rendered
	// output; the rendered table is bigger than the JSON, but the
	// proportional ordering is what matters — tighter cap on the
	// JSON side just means the truncation footer fires a little
	// earlier in rendered output.
	used := len(ReverseTruncationFooter)
	kept := 0
	for _, f := range sorted {
		approx := approxFindingBytes(f)
		if used+approx > capBytes {
			break
		}
		used += approx
		kept++
	}
	dropped := len(sorted) - kept
	return sorted[:kept], dropped
}

// lineRangeStart parses the lower bound of a "42" or "42-58" line
// range. Returns 0 for unparseable input — sort still produces
// deterministic ordering since 0 collisions break to file-path
// ordering above.
func lineRangeStart(lr string) int {
	if lr == "" {
		return 0
	}
	end := strings.IndexByte(lr, '-')
	if end < 0 {
		end = len(lr)
	}
	n := 0
	for i := 0; i < end; i++ {
		c := lr[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// approxFindingBytes estimates the rendered table-row size for a
// finding. Used as the byte-budget heuristic in ReverseTruncate.
// The actual rendered size is the synthesizer's choice; this is a
// proxy that orders correctly, which is what truncation needs.
func approxFindingBytes(f Finding) int {
	return len(string(f.Severity)) + len(f.File) + len(f.LineRange) +
		len(f.Summary) + len(f.Reasoning) + 32 // 32-byte overhead per row
}
