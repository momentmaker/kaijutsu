package swarm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInput_FilesHappyPath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	if err := os.WriteFile(a, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("beta"), 0644); err != nil {
		t.Fatal(err)
	}
	preset := &Preset{Name: "refactor-plan", InputKind: InputFiles}
	ictx, err := ResolveInput(context.Background(), preset, InputOptions{Files: []string{b, a}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Sorted regardless of input order.
	if !strings.HasPrefix(ictx.Body, "--- "+a+"\n") {
		t.Errorf("expected a.go section first (sorted), got body:\n%s", ictx.Body)
	}
	if len(ictx.CacheKey) != 12 {
		t.Errorf("cache key length wrong: %q", ictx.CacheKey)
	}
	if len(ictx.Files) != 2 {
		t.Errorf("expected 2 files in meta, got %v", ictx.Files)
	}
}

func TestResolveInput_FilesEmpty(t *testing.T) {
	preset := &Preset{Name: "refactor-plan", InputKind: InputFiles}
	_, err := ResolveInput(context.Background(), preset, InputOptions{})
	if err == nil {
		t.Fatal("expected error on empty file list")
	}
}

func TestResolveInput_FilesOverCap(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "huge.bin")
	// One byte over the cap.
	if err := os.WriteFile(big, make([]byte, maxBytesFiles+1), 0644); err != nil {
		t.Fatal(err)
	}
	preset := &Preset{Name: "refactor-plan", InputKind: InputFiles}
	_, err := ResolveInput(context.Background(), preset, InputOptions{Files: []string{big}})
	if err == nil {
		t.Fatal("expected error when files exceed cap")
	}
	if !strings.Contains(err.Error(), "exceed") {
		t.Errorf("expected 'exceed' in error, got %v", err)
	}
}

func TestResolveInput_FilesWithGoalPrependedAndCounted(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	if err := os.WriteFile(a, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	preset := &Preset{Name: "refactor-plan", InputKind: InputFiles}
	ictx, err := ResolveInput(context.Background(), preset, InputOptions{
		Files: []string{a},
		Goal:  "extract handler into service",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(ictx.Body, "--- GOAL ---\n") {
		t.Errorf("expected GOAL prefix, got body:\n%s", ictx.Body)
	}
	if !strings.Contains(ictx.Body, "extract handler into service") {
		t.Errorf("goal text missing from body: %s", ictx.Body)
	}
	// Cache key should differ from a no-goal call
	noGoalCtx, _ := ResolveInput(context.Background(), preset, InputOptions{Files: []string{a}})
	if ictx.CacheKey == noGoalCtx.CacheKey {
		t.Errorf("goal should affect cache key; both = %q", ictx.CacheKey)
	}
}

func TestResolveInput_FilesWithGoalCountsAgainstCap(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	// File at exactly cap; goal pushes it over.
	if err := os.WriteFile(a, make([]byte, maxBytesFiles), 0644); err != nil {
		t.Fatal(err)
	}
	preset := &Preset{Name: "refactor-plan", InputKind: InputFiles}
	_, err := ResolveInput(context.Background(), preset, InputOptions{
		Files: []string{a},
		Goal:  "this is the goal text that pushes over",
	})
	if err == nil {
		t.Fatal("expected error: file at cap + non-empty goal should bust the cap")
	}
	if !strings.Contains(err.Error(), "exceed") {
		t.Errorf("expected 'exceed' in error, got %v", err)
	}
}

func TestResolveInput_PromptHappyPath(t *testing.T) {
	preset := &Preset{Name: "brainstorm", InputKind: InputPrompt}
	ictx, err := ResolveInput(context.Background(), preset, InputOptions{Prompt: "  how  do  I rate-limit  "})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ictx.Body != "how do I rate-limit" {
		t.Errorf("body not canonicalized: %q", ictx.Body)
	}
	if len(ictx.CacheKey) != 12 {
		t.Errorf("cache key length wrong: %q", ictx.CacheKey)
	}
}

func TestResolveInput_PromptEmpty(t *testing.T) {
	preset := &Preset{Name: "brainstorm", InputKind: InputPrompt}
	_, err := ResolveInput(context.Background(), preset, InputOptions{Prompt: "   "})
	if err == nil {
		t.Fatal("expected error on whitespace-only prompt")
	}
}

func TestResolveInput_PromptOverCap(t *testing.T) {
	preset := &Preset{Name: "brainstorm", InputKind: InputPrompt}
	huge := strings.Repeat("a", maxBytesPrompt+1)
	_, err := ResolveInput(context.Background(), preset, InputOptions{Prompt: huge})
	if err == nil {
		t.Fatal("expected error on over-cap prompt")
	}
}

func TestResolveInput_UnknownKind(t *testing.T) {
	preset := &Preset{Name: "weird", InputKind: 99}
	_, err := ResolveInput(context.Background(), preset, InputOptions{})
	if err == nil {
		t.Fatal("expected error on unknown InputKind")
	}
}
