// schema_embed.go — embeds the swarm.yaml JSON Schema into the
// binary at build time. Used by `jutsu swarm validate` so the
// command has runtime access to the schema without depending on
// install location.
//
// The embedded copy at cli/internal/swarm/swarm.schema.json is
// vendored from schemas/swarm.schema.json at the repo root. Keep
// in sync; lint-skills.yml validates BOTH copies on PR.
package swarm

import _ "embed"

//go:embed swarm.schema.json
var swarmSchemaJSON []byte

// SchemaJSON returns the embedded JSON Schema for swarm.yaml.
//
// v0.13.0 status: exposed but not yet consumed by user-facing
// commands. `jutsu swarm validate` currently performs struct-level
// validation (name pattern, %s slot count, severity enum) — the
// same checks the schema would enforce, expressed in Go directly
// to avoid pulling a JSON-Schema-validator dependency. The
// embedded schema stays available so a future strict-mode flag
// (`jutsu swarm validate --json-schema-strict`) can run the
// canonical schema against arbitrary yaml without depending on
// the install location of schemas/swarm.schema.json.
//
// Also consumed by hypothetical external tooling that wants to
// validate swarm.yaml against the canonical schema without
// shipping a separate copy of the file.
func SchemaJSON() []byte {
	return swarmSchemaJSON
}
