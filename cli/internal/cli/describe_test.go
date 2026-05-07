package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestDescribe_OutputsValidJSON covers the basic contract: `jutsu
// describe` returns parseable JSON with the schema_version + tool
// + commands fields agents need at session-start.
func TestDescribe_OutputsValidJSON(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"describe"})
	if err := root.Execute(); err != nil {
		t.Fatalf("describe execute: %v", err)
	}
	var c catalog
	if err := json.Unmarshal(stdout.Bytes(), &c); err != nil {
		t.Fatalf("parse JSON output: %v\noutput:\n%s", err, stdout.String())
	}
	if c.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", c.SchemaVersion)
	}
	if c.Tool != "jutsu" {
		t.Errorf("tool = %q, want jutsu", c.Tool)
	}
	if len(c.Commands) == 0 {
		t.Error("commands list is empty — cobra tree wiring broken?")
	}
}

// TestDescribe_IncludesKnownCommands smoke-checks that core commands
// (init, install, swarm, finding) appear in the catalog. Guards
// against accidental tree disconnection between root.AddCommand and
// the describe walker.
func TestDescribe_IncludesKnownCommands(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"describe"})
	_ = root.Execute()

	var c catalog
	_ = json.Unmarshal(stdout.Bytes(), &c)

	want := map[string]bool{
		"jutsu init":     false,
		"jutsu install":  false,
		"jutsu swarm":    false,
		"jutsu finding":  false,
		"jutsu agent":    false,
	}
	for _, cmd := range c.Commands {
		if _, ok := want[cmd.Name]; ok {
			want[cmd.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("describe catalog missing %q", name)
		}
	}
}

// TestDescribe_SkipsHelpAndCompletion verifies cobra's auto-generated
// `help` and `completion` subcommands don't pollute the catalog —
// they're noise for fresh agents.
func TestDescribe_SkipsHelpAndCompletion(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"describe"})
	_ = root.Execute()

	if strings.Contains(stdout.String(), `"name": "jutsu help"`) {
		t.Error("describe should skip cobra's auto-generated `help` subcommand")
	}
	if strings.Contains(stdout.String(), `"name": "jutsu completion"`) {
		t.Error("describe should skip cobra's auto-generated `completion` subcommand")
	}
}

// TestDescribe_NestedSubcommandsResolved verifies grandchild commands
// (e.g. `jutsu agent add`, `jutsu finding accept`) appear under their
// parent's subcommands array — not flattened to root.
func TestDescribe_NestedSubcommandsResolved(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"describe"})
	_ = root.Execute()

	var c catalog
	_ = json.Unmarshal(stdout.Bytes(), &c)

	var agentCmd *commandSpec
	for _, cmd := range c.Commands {
		if cmd.Name == "jutsu agent" {
			agentCmd = cmd
			break
		}
	}
	if agentCmd == nil {
		t.Fatal("jutsu agent missing from catalog")
	}
	if len(agentCmd.Subcommands) == 0 {
		t.Error("jutsu agent should have nested subcommands (add, list, doctor, etc.)")
	}
	// At least one expected subcommand
	found := false
	for _, sub := range agentCmd.Subcommands {
		if sub.Name == "jutsu agent add" || sub.Name == "jutsu agent list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("jutsu agent subcommands missing expected entries (add / list)")
	}
}

// TestDescribe_FlagsHaveTypeInfo guards against a regression in the
// flag-walking path. Each flag spec must carry name + type at minimum.
func TestDescribe_FlagsHaveTypeInfo(t *testing.T) {
	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"describe"})
	_ = root.Execute()

	var c catalog
	_ = json.Unmarshal(stdout.Bytes(), &c)

	checked := 0
	walk := func(cmds []*commandSpec) {
		for _, cmd := range cmds {
			for _, f := range cmd.Flags {
				if f.Name == "" {
					t.Errorf("%s has flag with empty name", cmd.Name)
				}
				if f.Type == "" {
					t.Errorf("%s flag %q has empty type", cmd.Name, f.Name)
				}
				checked++
			}
		}
	}
	walk(c.Commands)
	if checked == 0 {
		t.Error("no flags found across all commands — describe walker likely broken")
	}
}
