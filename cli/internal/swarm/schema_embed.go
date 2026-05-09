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
// Consumed by `jutsu swarm validate` to schema-validate user
// swarm.yaml files before structural validation runs.
func SchemaJSON() []byte {
	return swarmSchemaJSON
}
