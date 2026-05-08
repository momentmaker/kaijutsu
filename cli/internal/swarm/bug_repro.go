// bug_repro.go — v0.12 Tier A preset. Vague bug report → ranked
// repro hypotheses + minimal-repro steps for the top hypothesis.
//
// Multi-agent shape is native: hypothesis generation across distinct
// failure-mode categories (state / race / env / input / version)
// benefits from cross-model diversity. The synthesizer clusters
// hypotheses by category + ranks by confidence + corroboration.
//
// Cobra wiring: cli/internal/cli/swarm_bug_repro.go binds positional
// bug description + optional --files <paths>. Files content is
// baked into the prompt at command time via BuildBugReproPrompt;
// the bug description goes through the standard InputPrompt path
// as the body substituted into the trailing %s slot at dispatch.
package swarm

import "strings"

// bugReproPreset is the v0.12 vague-bug → ranked-hypotheses preset.
// The cobra layer mutates DefaultPrompt at command-time to bake in
// optional --files content; this default template is the
// no-files-context fallback for library callers that go directly
// through the registry.
var bugReproPreset = Preset{
	Name:          "bug-repro",
	Description:   "Vague bug report → ranked repro hypotheses across categories (state/race/env/input/version) + minimal-repro steps for the top hypothesis.",
	InputKind:     InputPrompt,
	SeverityVocab: []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo},
	DefaultPrompt: bugReproDefaultPrompt,
	Synthesizer:   bugReproSynthesizer,
	Debate:        prReviewPreset.Debate,
}

// bugReproDefaultPrompt is the no-files-context fallback. Used when
// the cobra layer didn't bake --files content via BuildBugReproPrompt
// (or when called directly from the registry). The bug description
// substitutes into the trailing %s slot at dispatch time.
const bugReproDefaultPrompt = `A user reports the following bug. No code context was provided —
work from the bug description alone.

Generate 3-5 distinct repro hypotheses spanning DIFFERENT
categories. Don't pad with weak guesses; pick categories you think
most likely.

Each finding is a JSON object with:
  severity:    blocker | issue | minor | info
  file:        relative path or "n/a"
  line_range:  "42" or "42-58" or "0" when n/a
  summary:     "[<CATEGORY>] one-line hypothesis description"
  reasoning:   1-3 sentences explaining why this could explain the
               reported bug
  confidence:  0.0-1.0
  repro_steps: array of 3-5 numbered repro steps that would VERIFY
               this hypothesis (only for high-confidence hypotheses;
               low-confidence may use [])

CATEGORY tags: state | race | env | input | version.

If the bug description is too vague to hypothesize from, return [].
No prose, no code fences, no commentary outside the JSON.

BUG:
%s
`

// BuildBugReproPrompt assembles the per-agent bug-repro prompt with
// the supplied --files content baked in. The result has exactly ONE
// %s slot remaining — for the bug description, which ResolveInput
// substitutes at dispatch time via fmt.Sprintf.
//
// Two safety steps protect downstream fmt.Sprintf:
//  1. {{FILES_CONTENT}} placeholder + strings.Replace (NOT
//     fmt.Sprintf with two %s) so fmt isn't asked to leave a %s
//     unfilled.
//  2. Literal `%` chars in files content escaped to `%%` BEFORE
//     replacement — prevents the v0.8 dreamLensWild incident where
//     a code comment containing `// 10% slack` would leak `% s` as
//     a format directive.
func BuildBugReproPrompt(filesContent string) string {
	if strings.TrimSpace(filesContent) == "" {
		return bugReproDefaultPrompt
	}
	escaped := strings.ReplaceAll(filesContent, "%", "%%")
	return strings.Replace(bugReproPromptTemplate, "{{FILES_CONTENT}}", escaped, 1)
}

// bugReproPromptTemplate uses {{FILES_CONTENT}} for the code
// context (substituted via strings.Replace in BuildBugReproPrompt)
// and ONE %s slot for the bug description (substituted via
// fmt.Sprintf at dispatch time by ResolveInput).
const bugReproPromptTemplate = `A user reports a bug. Code context for the affected area is
provided.

Generate 3-5 distinct repro hypotheses spanning DIFFERENT
categories from the list below. Use the code context to ground
your hypotheses (cite specific functions / lines when relevant);
don't pad with weak guesses.

Each finding is a JSON object with:
  severity:    blocker | issue | minor | info
  file:        relative path (from the provided code context if
               cited, or "n/a")
  line_range:  "42" or "42-58" (or "0" when n/a)
  summary:     "[<CATEGORY>] one-line hypothesis description"
  reasoning:   1-3 sentences explaining the chain of events that
               could cause this bug, citing code-context evidence
               when available
  confidence:  0.0-1.0
  repro_steps: array of 3-5 numbered repro steps that would VERIFY
               this hypothesis (REQUIRED for confidence >= 0.50;
               may be [] for lower-confidence speculative
               hypotheses)

CATEGORY tags:
- state: uninitialized / stale / wrong / leaked across calls
- race: timing / concurrency / ordering / interleaving
- env: config / version / OS / deps / network / clock
- input: malformed / hostile / edge / encoding / null / empty
- version: pre-existing bug / regression introduced in commit X /
  dependency upgrade

DO consider:
- Recent changes to the affected code (if visible in the context)
- Cross-component contracts that could be violated
- Time / size / locale / charset assumptions

DO NOT consider:
- Vague "user error" or "they should RTFM" hypotheses
- Solutions (this preset finds CAUSES, not fixes)

If the bug description is too vague even with code context to
hypothesize from, return [].
No prose, no code fences, no commentary outside the JSON.

CODE-CONTEXT:
{{FILES_CONTENT}}

BUG:
%s
`

// bugReproSynthesizer aggregates per-agent bug-repro hypotheses
// into a markdown report keyed by category. Output emphasizes
// CLUSTERING + RANKING; corroborated hypotheses (proposed by 2+
// reviewers) bubble up with higher trust.
const bugReproSynthesizer = `Three reviewers each generated bug-repro hypotheses for the same
bug. Cluster their hypotheses by category (state / race / env /
input / version). Within a cluster, different reviewers may have
proposed slightly-different framings of the same hypothesis;
treat those as one cluster row with multiple reviewers credited.

For each cluster output:
- summary: synthesized one-line hypothesis (use the clearest
  reviewer phrasing)
- category: state / race / env / input / version
- severity: highest across reviewers in the cluster
- confidence: mean across reviewers (round to 2 decimals)
- reviewers: list of reviewer names that proposed this hypothesis
- repro_steps: best version across reviewers — preserve verbatim
  from the highest-confidence reviewer in the cluster (do NOT
  synthesize new steps)
- evidence: 1-2 sentences citing the strongest reviewer's reasoning

Rank clusters by:
  1. severity (blocker > issue > minor > info)
  2. reviewer count (3/3 > 2/3 > 1/3 — corroboration bumps trust)
  3. mean confidence

Output format:

## Repro hypotheses (top 3 ranked)

### 1. [<category>] <summary> — confidence <c>, reviewers <n/3>
- evidence: <1-2 sentences>
- reviewers: <list>
- repro_steps:
  1. <step>
  2. <step>
  ...

### 2. ...
### 3. ...

## Lower-confidence runners-up

| # | category | reviewers | confidence | summary |
|---|---|---|---|---|
| 4 | env | 1/3 | 0.42 | clock skew between regions |
| 5 | ... |

(Runners-up listed WITHOUT repro_steps to avoid wasting user time
on low-confidence chases. To get steps for a runner-up, re-run
with the runner-up's category as the focus.)

If no plausible hypotheses surfaced, output:

## Repro hypotheses

(no plausible hypotheses surfaced — bug description may be too
vague; provide more context via --files or narrow the description)

PER-AGENT FINDINGS:
%s
`
