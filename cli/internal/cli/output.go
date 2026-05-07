// output.go — auto-format selection per the agent-first / human-
// friendly design principle (AGENTS.md). Commands call AutoFormat()
// to decide between markdown / table / colored (TTY) and JSON
// (pipe / redirect / agent capture). Explicit `--format json` /
// `--format markdown` always wins.
//
// Pattern from `gh`, `jq -C`, `kubectl`. Zero coordination cost:
// agents shelling out get JSON automatically without flag knowledge;
// humans in terminals get pretty output without thinking; CI / scripts
// get JSON because they're non-TTY.
package cli

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// fdHolder is satisfied by *os.File (the writer behind os.Stdout).
// When cmd.OutOrStdout() returns something else (e.g. a *bytes.Buffer
// in tests, or a wrapping writer), the type assertion fails and we
// fall back to "non-TTY" → JSON. That's the desired behavior for
// programmatic captures.
type fdHolder interface {
	Fd() uintptr
}

// FormatMarkdown / FormatJSON / FormatTable are the canonical
// format names. Commands switch on the AutoFormat result. Other
// formats (e.g. yaml) can extend this enum without breaking callers.
const (
	FormatMarkdown = "markdown"
	FormatJSON     = "json"
	FormatTable    = "table"
)

// AutoFormat returns the output format the command should use. Looks
// up the --format flag (if registered on the command) for an explicit
// override; otherwise picks based on whether stdout is a TTY.
//
// Default fallback when stdout is a TTY: caller-provided ttyDefault.
// Most commands use FormatMarkdown; a few (e.g. `jutsu list`) prefer
// FormatTable. Caller picks; AutoFormat just routes.
//
// Pipes, redirects, and agent stdout-capture all return FormatJSON
// — non-TTY = machine consumer.
func AutoFormat(cmd *cobra.Command, ttyDefault string) string {
	if cmd.Flags().Lookup("format") != nil {
		if explicit, _ := cmd.Flags().GetString("format"); explicit != "" {
			return explicit
		}
	}
	if cmd.Flags().Lookup("json") != nil {
		if jsonFlag, _ := cmd.Flags().GetBool("json"); jsonFlag {
			return FormatJSON
		}
	}
	// Prefer cobra's OutOrStdout writer for TTY check — when tests
	// or wrapping callers redirect via cmd.SetOut(buf), AutoFormat
	// must see the redirection and return JSON (the default for
	// non-TTY consumers). Falls back to the process stdout when the
	// cobra writer doesn't carry a file descriptor (e.g. *bytes.Buffer
	// → fdHolder assertion fails → treat as non-TTY).
	if w, ok := cmd.OutOrStdout().(fdHolder); ok {
		if isatty.IsTerminal(w.Fd()) || isatty.IsCygwinTerminal(w.Fd()) {
			return ttyDefault
		}
		return FormatJSON
	}
	// Non-fd writer (test buffer, wrapping writer) → non-TTY by
	// definition → JSON.
	_ = os.Stdout
	return FormatJSON
}

// BindFormatFlags registers the standard --format + --json flags on
// a cobra command. Commands that gain auto-format support call this
// in their cobra setup.
//
// --format: explicit string (json / markdown / table — caller validates)
// --json:   shorthand boolean for --format json
//
// Both nil-safe: AutoFormat handles missing flags gracefully (returns
// ttyDefault on TTY, FormatJSON otherwise).
func BindFormatFlags(cmd *cobra.Command) {
	cmd.Flags().String("format", "", "output format (json | markdown | table); auto-detects when unset (TTY=human, pipe=json)")
	cmd.Flags().Bool("json", false, "shorthand for --format json")
}
