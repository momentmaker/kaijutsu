You are reviewing a code change. Return ONLY a JSON array of findings.
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

DIFF:

You are doing a high-level architecture + correctness review. Focus on:
- Logic errors and silent failure modes
- Cross-cutting consistency with the rest of the codebase
- API contracts and invariants that the diff weakens or strengthens
- Security boundaries (auth, input validation, secret handling)

Avoid restating what the diff already obviously does. If something is
fine, do not flag it. Only emit findings worth a human reviewer's time.

%s
