package swarm

import (
	"strings"
	"testing"
)

func TestParseFindings_Direct(t *testing.T) {
	in := `[{"severity":"blocker","file":"a.go","line_range":"42","summary":"x","reasoning":"y","confidence":0.9}]`
	got, err := ParseFindings(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Severity != SeverityBlocker {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseFindings_FencedJSON(t *testing.T) {
	in := "Here you go:\n```json\n[{\"severity\":\"issue\",\"file\":\"x\",\"line_range\":\"1\",\"summary\":\"s\",\"reasoning\":\"r\",\"confidence\":0.5}]\n```\nHope that helps."
	got, err := ParseFindings(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Severity != SeverityIssue {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseFindings_Wrapped(t *testing.T) {
	in := `{"findings":[{"severity":"minor","file":"y","line_range":"2","summary":"s","reasoning":"r","confidence":0.3}]}`
	got, err := ParseFindings(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Severity != SeverityMinor {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseFindings_FirstArrayInProse(t *testing.T) {
	in := "Sure! [{\"severity\":\"info\",\"file\":\"z\",\"line_range\":\"3\",\"summary\":\"s\",\"reasoning\":\"r\",\"confidence\":1.0}] done."
	got, err := ParseFindings(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseFindings_Malformed(t *testing.T) {
	in := "I cannot return JSON for this request."
	_, err := ParseFindings(in)
	if err != ErrMalformedJSON {
		t.Fatalf("expected ErrMalformedJSON, got %v", err)
	}
}

func TestParseFindings_Empty(t *testing.T) {
	in := "[]"
	got, err := ParseFindings(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestNormalize_SeverityCoercion(t *testing.T) {
	in := []Finding{
		{Severity: "CRITICAL"},
		{Severity: "warn"},
		{Severity: "nit"},
		{Severity: "weird"},
	}
	out := normalize(in)
	want := []Severity{SeverityBlocker, SeverityIssue, SeverityMinor, SeverityInfo}
	for i, w := range want {
		if out[i].Severity != w {
			t.Errorf("[%d] got %s want %s", i, out[i].Severity, w)
		}
	}
}

func TestNormalize_ConfidenceClamp(t *testing.T) {
	in := []Finding{{Confidence: -0.5}, {Confidence: 2.0}, {Confidence: 0.5}}
	out := normalize(in)
	if out[0].Confidence != 0 || out[1].Confidence != 1 || out[2].Confidence != 0.5 {
		t.Fatalf("bad clamp: %+v", out)
	}
}

func TestFirstJSONArray_NestedAndStrings(t *testing.T) {
	in := `prefix [{"a":"]"},{"b":"["}] suffix`
	got := firstJSONArray(in)
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("bad extract: %q", got)
	}
}
