package swarm

import "fmt"

// Preset is a named bundle of per-agent prompts + a synthesizer
// prompt. Phase 1 ships one — pr-review. Phase 2 generalizes.
type Preset struct {
	Name string
	// PerAgent maps an agent to the prompt template that agent should
	// receive in Pass 1. Templates expect %s where the diff is
	// substituted in.
	PerAgent map[AgentName]string
	// Synthesizer prompt template. Consumes the marshalled per-agent
	// findings JSON via %s.
	Synthesizer string
	// Debate prompt template (Pass 2 critique). Used in --full mode.
	Debate string
}

// PresetFor returns the named preset or an error if unknown. Stage 1
// hardcodes pr-review; Stage 5 makes prompts loadable from the
// installed skill directory so authors can override without
// recompiling jutsu.
func PresetFor(name string) (*Preset, error) {
	switch name {
	case "pr-review":
		return &prReviewPreset, nil
	}
	return nil, fmt.Errorf("unknown preset %q (Phase 1 supports: pr-review)", name)
}

// prReviewPreset is the built-in default. The same skill that
// invokes jutsu swarm can override these by shipping its own
// prompts/<agent>.md (Stage 5).
var prReviewPreset = Preset{
	Name: "pr-review",
	PerAgent: map[AgentName]string{
		AgentClaude: prReviewSharedHeader + `

You are doing a high-level architecture + correctness review. Focus on:
- Logic errors and silent failure modes
- Cross-cutting consistency with the rest of the codebase
- API contracts and invariants that the diff weakens or strengthens
- Security boundaries (auth, input validation, secret handling)

Avoid restating what the diff already obviously does. If something is
fine, do not flag it. Only emit findings worth a human reviewer's time.

%s
`,
		AgentCodex: prReviewSharedHeader + `

You are doing a brutal edge-case + corner-condition review. Focus on:
- Off-by-one errors, nil/null/empty handling, integer overflow
- Race conditions, ordering assumptions, retry semantics
- Inputs that look fine but break under load or weird locale/encoding
- Error paths that swallow failures or leak resources

Be skeptical. Assume nothing the code doesn't enforce. Only emit
findings you'd stake your reputation on.

%s
`,
		AgentGemini: prReviewSharedHeader + `

You are doing a cross-file pattern + consistency review. Focus on:
- Diff introducing a pattern that conflicts with existing patterns
  elsewhere in the codebase
- Naming, idiom, and style drift
- Missing test coverage for code paths the diff exercises
- Documentation/comments that contradict the new behavior

If the diff is a one-off tactical fix, that's OK to say so and emit
no findings.

%s
`,
	},
	Synthesizer: `You are synthesizing a multi-agent code review for a single PR.

Below are findings JSON arrays from N independent reviewers. Each
finding has: severity (blocker/issue/minor/info), file, line_range,
summary, reasoning, confidence.

Your job:
1. Cluster findings that point at the same root cause across reviewers.
   Merge them into one entry; record which reviewers flagged it.
2. Tag each clustered finding with consensus level: 3/3, 2/3, or 1/3.
3. Sort by severity (blocker > issue > minor > info), break ties by
   consensus level (higher first).
4. Output a markdown report with these sections:
   - A "Findings" section with reasoning for each clustered finding.
   - A "Disagreements" section calling out 1/N findings — these are
     the conversation-starters worth surfacing prominently.

Be terse. No filler. No restating the prompt. No prefatory paragraph.
The disagreement table and top-line summary are rendered separately
and prepended to your output; do NOT duplicate them.

REVIEWERS' FINDINGS:
%s
`,
	Debate: `You previously reviewed a PR. Here are the findings from your peer
reviewers, plus your own. Your job: critique the others.

For each peer finding:
- If you agree, add it to your kept-findings list with confidence boosted.
- If you disagree (the finding is wrong, overcounted, or overblown),
  add it to your rejected list with a brief reason.
- Add any NEW findings the peers' reviews surfaced that you missed.

Return ONLY a JSON array matching the original findings schema, where
the reasoning field includes "[agreed with peer X]" or "[disputes peer
X: <reason>]" markers when applicable.

YOUR ORIGINAL FINDINGS:
%s

PEERS' FINDINGS:
%s
`,
}

const prReviewSharedHeader = `You are reviewing a code change. Return ONLY a JSON array of findings.
Schema for each finding:
{
  "severity":   "blocker" | "issue" | "minor" | "info",
  "file":       "path/relative/to/repo.go",
  "line_range": "42" | "42-58",
  "summary":    "one-line description",
  "reasoning":  "1-3 sentence explanation",
  "confidence": 0.0-1.0
}

If you find nothing, return [].
No prose, no code fences, no commentary outside the JSON.

DIFF:`
