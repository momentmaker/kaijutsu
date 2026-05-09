package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cleanSwarmYaml = `
- name: my-tight-review
  description: shorter pr-review
  inputKind: diff
  defaultPrompt: "review %s"
  synthesizer: "synthesize %s"
  severityVocab: [blocker, issue, minor, info]
  mode: quick
  personas: [paranoid-security-claude]
  confidenceFloor: 0.55
`

const collisionSwarmYaml = `
- name: pr-review
  description: shadows built-in
  inputKind: diff
  defaultPrompt: "%s"
  synthesizer: "%s"
  severityVocab: [issue]
`

const brokenEntrySwarmYaml = `
- name: bad-prompt
  description: missing slot
  inputKind: prompt
  defaultPrompt: "no slot here"
  synthesizer: "%s"
  severityVocab: [issue]
`

func writeYamlFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSwarmValidate_CleanYamlExitsZero(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "swarm.yaml")
	writeYamlFile(t, path, cleanSwarmYaml)

	cmd := newSwarmValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean yaml should exit zero; got err=%v\nstderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout should contain 'OK'; got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "my-tight-review") {
		t.Errorf("stdout should list 'my-tight-review'; got %q", stdout.String())
	}
}

func TestSwarmValidate_NameCollisionExitsNonZero(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "swarm.yaml")
	writeYamlFile(t, path, collisionSwarmYaml)

	cmd := newSwarmValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("yaml with built-in name collision should error")
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "pr-review") {
		t.Errorf("output should name the colliding preset; got %q", combined)
	}
}

func TestSwarmValidate_MissingFileErrorsHelpfully(t *testing.T) {
	cmd := newSwarmValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"/this/path/does/not/exist.yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found'; got %v", err)
	}
}

func TestSwarmValidate_FieldCitationOnBrokenEntry(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "swarm.yaml")
	writeYamlFile(t, path, brokenEntrySwarmYaml)

	cmd := newSwarmValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("yaml with missing slot should error")
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "bad-prompt") {
		t.Errorf("output should name the bad entry; got %q", combined)
	}
	if !strings.Contains(combined, "%s slot") {
		t.Errorf("output should cite the slot-count rule; got %q", combined)
	}
}

func TestSwarmValidate_DefaultPathIsKaijutsuSwarmYaml(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	writeYamlFile(t, filepath.Join(tmp, ".kaijutsu", "swarm.yaml"), cleanSwarmYaml)

	cmd := newSwarmValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{}) // no path arg
	if err := cmd.Execute(); err != nil {
		t.Fatalf("default path should resolve to .kaijutsu/swarm.yaml; got err=%v\nstderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "my-tight-review") {
		t.Errorf("stdout should show validated preset; got %q", stdout.String())
	}
}
