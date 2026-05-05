package swarm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Per-input-kind byte caps. Each preset's input must fit under its
// kind's cap before reaching the agents — over-cap aborts with a
// clear error so the user can narrow scope rather than silently
// truncating + getting bad findings.
const (
	maxBytesDiff   = 200 * 1024
	maxBytesFiles  = 200 * 1024
	maxBytesPrompt = 8 * 1024
)

// InputContext bundles the resolved input + cache key + source meta
// for one swarm run. Stages 3–7 build agent prompts from .Body and
// route .CacheKey to CacheRun/LoadCachedResults.
type InputContext struct {
	Preset    *Preset
	InputKind InputKind
	// Body is the text fed into agent prompt templates via fmt.Sprintf.
	Body string
	// CacheKey identifies this input for cache + replay. 7-64 char
	// lowercase hex (validated by ValidateSHA before paths are formed).
	CacheKey string
	// PR + SHA are populated for InputDiff runs; empty otherwise.
	// post-comment uses PR; the run marker uses SHA.
	PR  int
	SHA string
	// Files lists the paths reviewed for InputFiles runs (empty
	// otherwise). Used in stderr summaries.
	Files []string
}

// InputOptions carries the per-invocation flags from the cobra
// subcommand into ResolveInput. Each preset's subcommand fills the
// fields relevant to its InputKind; ResolveInput dispatches.
type InputOptions struct {
	// InputDiff:
	PR             int
	DiffFromBranch string
	// InputFiles:
	Files []string
	// Goal is an optional InputFiles companion. When non-empty it
	// gets prepended to the body as a `--- GOAL ---\n<goal>\n` block
	// AND hashed into the cache key. Used by refactor-plan; other
	// InputFiles presets (doc-review) leave it empty.
	Goal string
	// InputPrompt:
	Prompt string
}

// ResolveInput dispatches per preset.InputKind. Returns a populated
// InputContext or a user-facing error.
//
// pr-review (InputDiff) preserves the Phase-1 flow: gh pr diff or
// git diff vs base ref. Other kinds will be exercised as Stages 3,
// 5, 6, 7 add their respective subcommands.
func ResolveInput(ctx context.Context, preset *Preset, opts InputOptions) (*InputContext, error) {
	switch preset.InputKind {
	case InputDiff:
		return resolveDiffInput(ctx, preset, opts)
	case InputFiles:
		return resolveFilesInput(preset, opts)
	case InputPrompt:
		return resolvePromptInput(preset, opts)
	}
	return nil, fmt.Errorf("preset %q has unknown InputKind %d", preset.Name, preset.InputKind)
}

func resolveDiffInput(ctx context.Context, preset *Preset, opts InputOptions) (*InputContext, error) {
	var pctx *PRContext
	var err error
	if opts.DiffFromBranch != "" {
		pctx, err = FetchBranchDiff(ctx, opts.DiffFromBranch)
	} else {
		pctx, err = FetchPRContext(ctx, opts.PR)
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pctx.Diff) == "" {
		return nil, errors.New("diff is empty; nothing to review")
	}
	if len(pctx.Diff) > maxBytesDiff {
		return nil, fmt.Errorf("diff is %d bytes — exceeds %d-byte cap. Narrow scope (smaller PR) or split into multiple reviews", len(pctx.Diff), maxBytesDiff)
	}
	return &InputContext{
		Preset:    preset,
		InputKind: InputDiff,
		Body:      pctx.Diff,
		CacheKey:  pctx.SHA,
		PR:        pctx.PR,
		SHA:       pctx.SHA,
	}, nil
}

func resolveFilesInput(preset *Preset, opts InputOptions) (*InputContext, error) {
	if len(opts.Files) == 0 {
		return nil, fmt.Errorf("preset %q requires at least one file path argument", preset.Name)
	}
	entries := make([]FileEntry, 0, len(opts.Files))
	totalBytes := len(opts.Goal) // goal counts against the 200KB cap too
	for _, p := range opts.Files {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		totalBytes += len(data)
		if totalBytes > maxBytesFiles {
			return nil, fmt.Errorf("input files exceed %d-byte cap (current: %d). Split into multiple runs or narrow scope", maxBytesFiles, totalBytes)
		}
		entries = append(entries, FileEntry{Path: p, Content: string(data)})
	}
	body := AssembleFilesBody(entries)
	if opts.Goal != "" {
		body = "--- GOAL ---\n" + strings.TrimSpace(opts.Goal) + "\n\n" + body
	}
	return &InputContext{
		Preset:    preset,
		InputKind: InputFiles,
		Body:      body,
		CacheKey:  CacheKeyForFilesWithGoal(preset, entries, opts.Goal),
		Files:     append([]string(nil), opts.Files...),
	}, nil
}

func resolvePromptInput(preset *Preset, opts InputOptions) (*InputContext, error) {
	// Canonicalize first, THEN cap-check on the canonical form so the
	// cap matches what gets stored in Body and what gets hashed into
	// CacheKey. Pre-canonicalize cap-check would let a prompt with
	// lots of internal whitespace pass at 8 KB raw but produce a much
	// shorter canonicalized body — inconsistent and confusing for the
	// "shorten the prompt" hint.
	canonical := CanonicalizePrompt(opts.Prompt)
	if canonical == "" {
		return nil, fmt.Errorf("preset %q requires a non-empty prompt argument", preset.Name)
	}
	if len(canonical) > maxBytesPrompt {
		return nil, fmt.Errorf("prompt is %d bytes (after whitespace normalization) — exceeds %d-byte cap. Shorten the prompt", len(canonical), maxBytesPrompt)
	}
	return &InputContext{
		Preset:    preset,
		InputKind: InputPrompt,
		Body:      canonical,
		CacheKey:  CacheKeyForPrompt(preset, opts.Prompt),
	}, nil
}
