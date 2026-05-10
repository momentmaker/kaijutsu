// export.go — v0.15.0 export helpers.
//
// ExportJSONL writes findings rows one per line, no envelope, no
// schema_version. Sibling to the v0.7 single-object json export
// (which lives inline in cli/finding.go). The jsonl shape is the
// natural pipeable form: `jutsu finding export --format jsonl |
// rg foo` works as expected.
//
// Round-trip contract: a slice of Row values written via ExportJSONL,
// parsed back via json.Unmarshal one line at a time, deep-equals the
// original slice. Byte-equality is NOT promised (key ordering +
// numeric repr would require a canonical JSON encoder we don't ship).
package findings

import (
	"encoding/json"
	"fmt"
	"io"
)

// ExportJSONL writes one JSON object per line to w. Each line
// represents a Row. No envelope, no schema_version line, no trailing
// newline beyond the per-line ones.
//
// Empty rows = zero bytes written, no error.
func ExportJSONL(w io.Writer, rows []Row) error {
	if w == nil {
		return fmt.Errorf("findings: ExportJSONL: nil writer")
	}
	enc := json.NewEncoder(w)
	// Findings summaries / reasoning routinely contain code with `<`, `>`,
	// `&`. Default Go json encoder HTML-escapes these to < etc., which
	// muddies grep / jq / human-eyeball workflows. JSONL is meant for
	// pipelines, not browsers; turn the escaping off.
	enc.SetEscapeHTML(false)
	for i := range rows {
		if err := enc.Encode(rows[i]); err != nil {
			return fmt.Errorf("findings: ExportJSONL row %d: %w", i, err)
		}
	}
	return nil
}
