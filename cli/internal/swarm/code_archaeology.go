// code_archaeology.go — v0.12 Tier A preset. Explain WHY legacy
// code looks the way it does. Each agent generates competing
// historical-context theories ranked by evidence; the synthesizer
// clusters theories + marks corroborated vs contested.
//
// Multi-agent shape is native: theory generation across distinct
// historical hypotheses (workaround / era pattern / abandoned
// migration / perf opt / security mitigation / accidental
// complexity) benefits from cross-model diversity. Multiple
// reviewers may converge on the same theory with independent
// evidence, which is the strongest possible signal.
//
// Cobra wiring: cli/internal/cli/swarm_code_archaeology.go binds
// --code <path> + optional --git-log <since>. Git-log fetch is
// best-effort per spec Decision #9 — failures degrade to
// empty-history mode, NOT hard error. Git-log content is baked
// into the prompt at command time via BuildCodeArchaeologyPrompt;
// code body goes through standard InputFiles pipeline.
package swarm

import "strings"

// codeArchaeologyPreset is the v0.12 legacy-code-history preset.
// The cobra layer mutates DefaultPrompt at command-time to bake
// in --git-log content; this default template handles the
// no-history fallback for library callers + Decision #9's
// graceful-degradation path.
var codeArchaeologyPreset = Preset{
	Name:          "code-archaeology",
	Description:   "Explain WHY legacy code looks the way it does. Multi-agent generates competing historical-context theories (workaround / era pattern / abandoned migration / perf opt / security mitigation / accidental complexity) ranked by evidence + corroboration.",
	InputKind:     InputFiles,
	SeverityVocab: []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo},
	DefaultPrompt: codeArchaeologyDefaultPrompt,
	Synthesizer:   codeArchaeologySynthesizer,
	Debate:        genericDebateTemplate, // artifact-agnostic — input is legacy code + git history, not a PR
}

// codeArchaeologyDefaultPrompt is the no-history fallback. Used
// when the cobra layer didn't bake --git-log content (or when
// git-log fetch failed gracefully per Decision #9). Theories
// degrade — less evidence to cite, but the code itself is still
// analyzable.
const codeArchaeologyDefaultPrompt = `You are auditing legacy CODE to understand WHY it looks the way
it does. No git history was available — work from code structure
alone.

Generate 3-5 distinct historical-context theories. Cite evidence
from the code itself: comments, function/variable names that hint
at era, structural patterns, neighboring code, edge-case handling
that implies past bugs.

Each finding is a JSON object with:
  severity:    blocker | issue | minor | info
  file:        relative path
  line_range:  "42" or "42-58" (or "0" when n/a)
  summary:     "[<CATEGORY>] one-line theory"
  reasoning:   1-3 sentences explaining the theory + citing the
               specific code evidence that supports it
  confidence:  0.0-1.0

CATEGORY tags: workaround | era-pattern | abandoned-migration |
perf-opt | security-mitigation | accidental-complexity.

Severity meaning here is "trust this theory":
- blocker: load-bearing theory the user MUST verify before refactoring
- issue:   strong evidence; likely accurate
- minor:   plausible; corroborate with another source
- info:    speculative; only useful if other reviewers agree

If the code is too uniform / new to surface theories, return [].
No prose, no code fences, no commentary outside the JSON.

CODE:
%s
`

// BuildCodeArchaeologyPrompt assembles the per-agent prompt with
// the supplied --git-log content baked in. Result has exactly ONE
// %s slot remaining — for the code body, substituted by
// ResolveInput at dispatch time.
//
// Same two-slot safety pattern as test-gap + bug-repro:
//  1. {{HISTORY_CONTENT}} placeholder via strings.Replace.
//  2. Literal `%` chars escaped to `%%` to protect downstream
//     fmt.Sprintf from format-directive interpretation (the v0.8
//     dreamLensWild safety pattern).
func BuildCodeArchaeologyPrompt(historyContent string) string {
	if strings.TrimSpace(historyContent) == "" {
		return codeArchaeologyDefaultPrompt
	}
	escaped := strings.ReplaceAll(historyContent, "%", "%%")
	return strings.Replace(codeArchaeologyPromptTemplate, "{{HISTORY_CONTENT}}", escaped, 1)
}

// codeArchaeologyPromptTemplate uses {{HISTORY_CONTENT}} for the
// git-log baked-in content + ONE %s slot for the code body.
const codeArchaeologyPromptTemplate = `You are auditing legacy CODE to understand WHY it looks the way
it does. Git HISTORY is provided — commit messages + dates are the
strongest available evidence for theories.

Generate 3-5 distinct historical-context theories. For each
theory, cite EVIDENCE: a specific commit message, a date pattern
("this was added during era X"), a file naming convention, a
comment, neighboring code, or an edge-case handler that implies a
past bug.

Each finding is a JSON object with:
  severity:    blocker | issue | minor | info
  file:        relative path
  line_range:  "42" or "42-58" (or "0" when n/a)
  summary:     "[<CATEGORY>] one-line theory"
  reasoning:   1-3 sentences explaining the theory; MUST quote
               specific evidence (commit hash + message snippet,
               or code citation) — do not assert without evidence
  confidence:  0.0-1.0

CATEGORY tags:
- workaround: response to a past bug or quirk in another system
- era-pattern: idiomatic style of a past era of this codebase
  (pre-modern-language-feature, pre-framework-X, etc.)
- abandoned-migration: in-progress migration that stalled; current
  code is half-old half-new
- perf-opt: optimization for past hardware / scale / concurrency
  pattern (may or may not still be load-bearing)
- security-mitigation: response to a past vulnerability; check if
  the threat is still relevant before removing
- accidental-complexity: no clear historical reason; the code is
  just the way it is because no one cleaned it up

Severity meaning is "trust this theory":
- blocker: load-bearing — user MUST verify before refactoring
- issue:   strong evidence; likely accurate
- minor:   plausible; should be corroborated
- info:    speculative; only useful if other reviewers agree

DO cite specific commits when the history mentions them.
DO NOT pad with weak theories — quality over quantity.

If the code is too uniform / new for theories to apply, return [].
No prose, no code fences, no commentary outside the JSON.

GIT-HISTORY:
{{HISTORY_CONTENT}}

CODE:
%s
`

// codeArchaeologySynthesizer aggregates per-agent theories into a
// markdown report. Emphasizes CORROBORATION: theories proposed by
// 2+ reviewers with independent evidence are the strongest signal.
const codeArchaeologySynthesizer = `Three reviewers each generated historical-context theories about
the same code. Cluster by category (workaround / era-pattern /
abandoned-migration / perf-opt / security-mitigation /
accidental-complexity). Within a cluster, different reviewers may
have proposed different framings of the same underlying theory;
treat those as one cluster row.

For each cluster output:
- summary: synthesized one-line theory
- category: workaround / era-pattern / abandoned-migration /
  perf-opt / security-mitigation / accidental-complexity
- severity: highest across reviewers
- confidence: mean across reviewers (round to 2 decimals)
- reviewers: list of reviewer names that proposed this theory
- evidence: PRESERVE the strongest reviewer's specific citation
  (commit hash, code line, comment quote) — DO NOT summarize away
  the citation
- provenance: "corroborated" if 2+ reviewers cite INDEPENDENT
  evidence; "single-source" if only one reviewer; "contested" if
  reviewers cite EVIDENCE THAT POINTS DIFFERENT DIRECTIONS

Rank clusters by:
  1. severity (blocker > issue > minor > info)
  2. provenance (corroborated > single-source > contested)
  3. mean confidence

Output format:

## Theories (top N ranked)

### 1. [<category>] <summary> — <provenance>, confidence <c>, reviewers <n/3>
- evidence: <verbatim citation from strongest reviewer>
- reviewers: <list>

### 2. ...

## Contested theories (if any)

For "contested" provenance entries, list the conflicting evidence
side-by-side so the user can investigate which reviewer is right:

| reviewer | category | evidence |
|---|---|---|
| claude | workaround | "commit abc123 says fix-for-mobile-bug" |
| antigravity | perf-opt | "the loop unroll pattern matches era-X SIMD" |

If reviewers found no theories, output:

## Theories

(no plausible historical-context theories surfaced — code may be
recent / uniform / undocumented; if you suspect there IS history,
re-run with a wider --git-log <since>)

PER-AGENT FINDINGS:
%s
`
