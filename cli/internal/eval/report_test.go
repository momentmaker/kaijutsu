package eval

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestReport_RendersFromLocalJSON covers the end-to-end render
// path. Synthetic input → HTML output containing pass/fail rows +
// drill-down + embedded JSON blobs.
func TestReport_RendersFromLocalJSON(t *testing.T) {
	meta := Meta{
		StartedAt:        time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
		FinishedAt:       time.Date(2026, 5, 8, 12, 1, 0, 0, time.UTC),
		Workspace:        "/tmp/eval",
		Iteration:        1,
		SkillName:        "demo",
		Target:           "claude",
		Judge:            "claude",
		JudgeTemplateVer: JudgeTemplateVersion,
		Sides:            2,
	}
	bench := Benchmark{
		SkillName:  "demo",
		Iteration:  1,
		TotalEvals: 1,
		Results: map[string]EvalResult{
			"smoke": {
				ID:   "smoke",
				Name: "Smoke",
				Sides: map[string]SideResult{
					SideWithSkill: {
						Pass:       true,
						DurationMS: 100,
						CostUSD:    0.01,
						Output:     "with-skill output",
						Assertions: []AssertionResult{{
							Assertion: "assertion text",
							Pass:      true,
							Reason:    "matches expected",
							JudgeStage: 1,
						}},
					},
					SideWithoutSkill: {
						Pass:       false,
						DurationMS: 90,
						CostUSD:    0.01,
						Output:     "without-skill output",
						Assertions: []AssertionResult{{
							Assertion: "assertion text",
							Pass:      false,
							Reason:    "missing key detail",
							JudgeStage: 1,
						}},
					},
				},
			},
		},
	}
	html, err := RenderReport(meta, bench, SideWithSkill, SideWithoutSkill, 0.05)
	if err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	for _, must := range []string{
		"kaijutsu eval — demo",
		"iteration 1",
		"target: claude",
		"with_skill",
		"without_skill",
		"Smoke",
		"assertion text",
		"matches expected",
		"missing key detail",
		"with-skill output",
		"without-skill output",
		`class="pass"`,
		`class="fail"`,
		`<script type="application/json" id="meta">`,
		`<script type="application/json" id="benchmark">`,
		"v0.10-default", // judge template version
	} {
		if !strings.Contains(html, must) {
			t.Errorf("rendered HTML missing %q", must)
		}
	}
}

// TestReport_TruncatesLongOutputInDrillDown covers the embedded-
// snippet cap: outputs > 5000 bytes get a [truncated] marker.
func TestReport_TruncatesLongOutputInDrillDown(t *testing.T) {
	long := strings.Repeat("a", 6000)
	bench := Benchmark{
		SkillName: "x", Iteration: 1, TotalEvals: 1,
		Results: map[string]EvalResult{
			"e": {ID: "e", Name: "E", Sides: map[string]SideResult{
				SideWithSkill: {Pass: true, Output: long, Assertions: []AssertionResult{{Assertion: "a", Pass: true, JudgeStage: 1}}},
			}},
		},
	}
	meta := Meta{SkillName: "x", Iteration: 1, JudgeTemplateVer: JudgeTemplateVersion}
	html, err := RenderReport(meta, bench, SideWithSkill, SideWithoutSkill, 0)
	if err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if !strings.Contains(html, "[truncated") {
		t.Errorf("expected truncation marker for 6000-byte output")
	}
}

// TestReport_HTMLEscapesUserContent guards against XSS in skill
// names / eval names / assertion text. The report is local but
// could be hosted; html-escaping is the right default.
func TestReport_HTMLEscapesUserContent(t *testing.T) {
	bench := Benchmark{
		SkillName: "<script>alert('x')</script>",
		Iteration: 1,
		TotalEvals: 1,
		Results: map[string]EvalResult{
			"e": {ID: "e", Name: "<b>name</b>", Sides: map[string]SideResult{
				SideWithSkill: {Pass: true, Assertions: []AssertionResult{
					{Assertion: "<i>assertion</i>", Pass: true, Reason: "<u>reason</u>", JudgeStage: 1},
				}},
			}},
		},
	}
	meta := Meta{SkillName: "x", JudgeTemplateVer: JudgeTemplateVersion}
	html, err := RenderReport(meta, bench, SideWithSkill, SideWithoutSkill, 0)
	if err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	for _, banned := range []string{
		"<script>alert('x')</script>",
		"<b>name</b>",
		"<i>assertion</i>",
		"<u>reason</u>",
	} {
		if strings.Contains(html, banned) {
			t.Errorf("unescaped HTML in report: %q", banned)
		}
	}
	for _, must := range []string{
		"&lt;script&gt;",
		"&lt;b&gt;",
		"&lt;i&gt;",
	} {
		if !strings.Contains(html, must) {
			t.Errorf("expected escaped form %q", must)
		}
	}
}

// TestTruncateOutput_PreservesUTF8Boundaries covers the v0.10
// final-pr-review fix: byte-indexed slicing can split a multi-byte
// rune and produce invalid UTF-8 in the rendered HTML. Cut must
// rewind to the previous rune boundary.
func TestTruncateOutput_PreservesUTF8Boundaries(t *testing.T) {
	// "🚀" is 4 bytes (F0 9F 9A 80). Build a string where the
	// truncation cap lands inside that rune.
	s := strings.Repeat("a", 10) + "🚀" + strings.Repeat("b", 10)
	// Cap at 12 bytes — this falls in the middle of the rocket
	// rune (bytes 11-14 are the rocket; cap=12 = mid-rune).
	out := truncateOutput(s, 12)
	if !utf8.ValidString(out) {
		t.Errorf("truncated output contains invalid UTF-8: %q", out)
	}
	if !strings.HasPrefix(out, strings.Repeat("a", 10)) {
		t.Errorf("expected leading aaaaa... preserved; got %q", out[:20])
	}
	if strings.Contains(out, "\xf0\x9f\x9a") && !strings.Contains(out, "🚀") {
		t.Error("kept partial rune bytes — should have rewound past it")
	}
}
