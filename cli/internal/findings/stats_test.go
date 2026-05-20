package findings

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/momentmaker/kaijutsu/cli/internal/swarm"
)

// stats_test.go — covers the v0.14.0 Stats query + ParseSinceDuration
// shorthand parser. Mirrors the test plan in IMPLEMENTATION_PLAN.md
// Stage 2.

// openStatsStore opens an in-memory store backed by a tmp file (the
// store's loader hard-fails on `:memory:`-style DSNs since its WAL +
// busy_timeout pragmas need a real file).
func openStatsStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := Open(filepath.Join(tmp, "findings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustRecordRun(t *testing.T, s *Store, runID, fp, preset string, results []swarm.AgentResult) {
	t.Helper()
	if _, _, err := RecordRun(s, RunMeta{
		RunID:      runID,
		CodebaseFP: fp,
		Preset:     preset,
	}, results); err != nil {
		t.Fatalf("RecordRun(%s): %v", runID, err)
	}
}

func agentResult(agent string, n int) []swarm.AgentResult {
	out := []swarm.AgentResult{{Agent: agent}}
	for i := 0; i < n; i++ {
		out[0].Findings = append(out[0].Findings, swarm.Finding{
			Severity: "issue", File: "x.go", LineRange: "1", Summary: "f",
		})
	}
	return out
}

func TestStats_CountsByPreset(t *testing.T) {
	s := openStatsStore(t)
	mustRecordRun(t, s, "r1", "fp1", "pr-review", agentResult("default-claude", 3))
	mustRecordRun(t, s, "r2", "fp1", "polish", agentResult("default-antigravity", 2))
	mustRecordRun(t, s, "r3", "fp1", "pr-review", agentResult("default-claude", 1))

	rows, err := Stats(s, StatsOpts{By: StatsByPreset, AllCodebases: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2; got %+v", len(rows), rows)
	}
	// Ordered by count desc.
	if rows[0].Group != "pr-review" || rows[0].Count != 4 {
		t.Errorf("rows[0] = %+v; want {pr-review 4}", rows[0])
	}
	if rows[1].Group != "polish" || rows[1].Count != 2 {
		t.Errorf("rows[1] = %+v; want {polish 2}", rows[1])
	}
}

func TestStats_HonorsSinceWindow(t *testing.T) {
	s := openStatsStore(t)
	// Insert 1 row + manually backdate created_at; insert 1 fresh row.
	mustRecordRun(t, s, "old", "fp1", "pr-review", agentResult("default-claude", 1))
	if _, err := s.DB().Exec(`UPDATE findings SET created_at = ? WHERE run_id = 'old'`,
		time.Now().UTC().Add(-100*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	mustRecordRun(t, s, "fresh", "fp1", "pr-review", agentResult("default-claude", 1))

	rows, err := Stats(s, StatsOpts{By: StatsByPreset, Since: 7 * 24 * time.Hour, AllCodebases: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Count != 1 {
		t.Errorf("expected 1 row count=1 (fresh only); got %+v", rows)
	}
}

func TestStats_GroupByPersona(t *testing.T) {
	s := openStatsStore(t)
	mustRecordRun(t, s, "r1", "fp1", "pr-review", agentResult("default-claude", 2))
	mustRecordRun(t, s, "r2", "fp1", "pr-review", agentResult("default-antigravity", 5))

	rows, err := Stats(s, StatsOpts{By: StatsByPersona, AllCodebases: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d; want 2", len(rows))
	}
	if rows[0].Group != "default-antigravity" || rows[0].Count != 5 {
		t.Errorf("rows[0] = %+v; want {default-antigravity 5}", rows[0])
	}
}

func TestStats_GroupByProvider(t *testing.T) {
	s := openStatsStore(t)
	mustRecordRun(t, s, "r1", "fp1", "pr-review", agentResult("default-claude", 3))

	rows, err := Stats(s, StatsOpts{By: StatsByProvider, AllCodebases: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Group != "default-claude" {
		t.Errorf("rows = %+v; want one row group=default-claude (recorder denormalizes provider from agent name in legacy mode)", rows)
	}
}

func TestStats_CodebaseFilter(t *testing.T) {
	s := openStatsStore(t)
	mustRecordRun(t, s, "r1", "fp1", "pr-review", agentResult("default-claude", 3))
	mustRecordRun(t, s, "r2", "fp2", "pr-review", agentResult("default-claude", 5))

	rows, err := Stats(s, StatsOpts{By: StatsByPreset, CodebaseFP: "fp1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Count != 3 {
		t.Errorf("rows = %+v; want one row count=3 (fp1 only)", rows)
	}
}

func TestStats_EmptyDB_NoError(t *testing.T) {
	s := openStatsStore(t)
	rows, err := Stats(s, StatsOpts{By: StatsByPreset, AllCodebases: true})
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil {
		t.Error("Stats should return empty slice, not nil")
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v; want empty", rows)
	}
}

func TestStats_UnknownAxisErrors(t *testing.T) {
	s := openStatsStore(t)
	_, err := Stats(s, StatsOpts{By: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown axis")
	}
}

func TestStats_NilStoreErrors(t *testing.T) {
	if _, err := Stats(nil, StatsOpts{}); err == nil {
		t.Error("expected error for nil store")
	}
}

func TestParseSinceDuration_DaysWeeksMonthsYears(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"7d", 7 * 24 * time.Hour},
		{"4w", 4 * 7 * 24 * time.Hour},
		{"3mo", 3 * 30 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{" 90d ", 90 * 24 * time.Hour},
	}
	for _, c := range cases {
		got, err := ParseSinceDuration(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSinceDuration_FallsThroughToStdParser(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
	}
	for _, c := range cases {
		got, err := ParseSinceDuration(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSinceDuration_EmptyIsZero(t *testing.T) {
	d, err := ParseSinceDuration("")
	if err != nil {
		t.Fatal(err)
	}
	if d != 0 {
		t.Errorf("empty got %v; want 0", d)
	}
}

func TestParseSinceDuration_RejectsNegativeAndZero(t *testing.T) {
	for _, in := range []string{"0d", "0s", "-7d", "-30s"} {
		if _, err := ParseSinceDuration(in); err == nil {
			t.Errorf("%q should error", in)
		}
	}
}

func TestParseSinceDuration_BadInput(t *testing.T) {
	for _, in := range []string{"d", "abc", "7x", "mo"} {
		if _, err := ParseSinceDuration(in); err == nil {
			t.Errorf("%q should error", in)
		}
	}
}
