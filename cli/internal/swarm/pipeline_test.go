package swarm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestRunPipeline_EstimateRequiresCallback covers the fail-fast
// guard: --estimate without EstimateFn callback hard-errors rather
// than silently no-op'ing.
func TestRunPipeline_EstimateRequiresCallback(t *testing.T) {
	opts := PipelineOpts{
		Estimate: true,
		Preset:   &Preset{Name: "noop"},
		Input:    &InputContext{},
		// EstimateFn intentionally nil
	}
	_, err := RunPipeline(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when --estimate set but EstimateFn is nil")
	}
	if !strings.Contains(err.Error(), "EstimateFn") {
		t.Errorf("error should mention EstimateFn; got: %v", err)
	}
}

// TestRunPipeline_EstimateInvokesCallbackThenReturns covers the
// happy estimate path: EstimateFn fires, no other pipeline work
// happens, Result is empty (not a real run).
func TestRunPipeline_EstimateInvokesCallbackThenReturns(t *testing.T) {
	called := false
	opts := PipelineOpts{
		Estimate: true,
		Preset:   &Preset{Name: "noop"},
		Input:    &InputContext{},
		EstimateFn: func(po PipelineOpts) error {
			called = true
			return nil
		},
	}
	res, err := RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatalf("estimate path errored: %v", err)
	}
	if !called {
		t.Error("EstimateFn was not invoked")
	}
	if res == nil {
		t.Fatal("RunPipeline returned nil Result on estimate path")
	}
	if res.Markdown != "" {
		t.Errorf("estimate path should produce empty Markdown; got %q", res.Markdown)
	}
}

// TestRunPipeline_EstimateSurfacesCallbackError covers the error
// propagation path: EstimateFn returning an error must propagate
// through RunPipeline (the cli wrapper's renderer can fail to
// resolve agents.yaml, build estimates, etc.).
func TestRunPipeline_EstimateSurfacesCallbackError(t *testing.T) {
	wantErr := errors.New("estimator failed")
	opts := PipelineOpts{
		Estimate: true,
		Preset:   &Preset{Name: "noop"},
		Input:    &InputContext{},
		EstimateFn: func(po PipelineOpts) error {
			return wantErr
		},
	}
	_, err := RunPipeline(context.Background(), opts)
	if !errors.Is(err, wantErr) {
		t.Errorf("RunPipeline error = %v, want %v", err, wantErr)
	}
}

// TestRunPipeline_NilWritersFallbackToDiscard covers the I/O
// safety net: nil Stdout/Stderr/Stdin must not cause a nil-pointer
// dereference. RunPipeline replaces nil writers with io.Discard.
//
// We exercise this by hitting the EstimateFn path which doesn't
// touch stdout/stderr but still goes through the writer-init code.
func TestRunPipeline_NilWritersFallbackToDiscard(t *testing.T) {
	opts := PipelineOpts{
		Estimate:   true,
		Preset:     &Preset{Name: "noop"},
		Input:      &InputContext{},
		EstimateFn: func(po PipelineOpts) error { return nil },
		// Stdout, Stderr, Stdin all nil
	}
	if _, err := RunPipeline(context.Background(), opts); err != nil {
		t.Errorf("nil writers should not cause error; got %v", err)
	}
}

// TestRunPipeline_ResultStructPopulated covers the structure of
// what RunPipeline returns. Estimate path produces an empty Result
// (not nil) so callers can dereference safely.
func TestRunPipeline_ResultStructPopulated(t *testing.T) {
	var stderr bytes.Buffer
	opts := PipelineOpts{
		Estimate:   true,
		Preset:     &Preset{Name: "noop"},
		Input:      &InputContext{},
		Stderr:     &stderr,
		EstimateFn: func(po PipelineOpts) error { return nil },
	}
	res, err := RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("Result must not be nil even on estimate path")
	}
}
