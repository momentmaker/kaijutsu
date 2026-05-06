# MCP-as-peer reference integrations

The `mcp` driver lets deterministic analyzers participate in `jutsu
swarm` alongside LLM peers. Per spec D7 the server exposes a single
agreed-upon tool — `analyze` (overridable via `tool_name`) — that
takes the preset name + severity vocabulary + input body, and returns
a JSON array of findings.

This document is illustrative — none of the configs below are run as
part of the v0.6 test suite. The `mcp` driver itself is fully tested
against the in-tree stub at `testdata/stub_mcp_server/`.

## semgrep-mcp (security analyzer)

[semgrep](https://semgrep.dev) is a static analysis engine with ~2000
community-maintained rules covering OWASP basics, framework-specific
patterns, and common bug classes. A `semgrep-mcp` wrapper exposing
the `analyze` contract is straightforward to build (or fork from
existing community efforts).

```yaml
# ~/.kaijutsu/agents.yaml
providers:
  semgrep-mcp:
    driver: mcp
    transport: stdio
    command: npx
    args: ["semgrep-mcp"]
    tool_name: analyze
    timeout_seconds: 120
```

```yaml
# <repo>/.kaijutsu/agents.yaml
enabled: [claude, codex, gemini, semgrep-mcp]
```

```bash
$ jutsu swarm security-audit --personas \
    paranoid-security-claude,pragmatic-codex,default-gemini,default-semgrep-mcp
```

LLM peers + semgrep run in parallel. Findings tagged `[deterministic]`
in the synthesizer's disagreement table. When semgrep agrees with
two of three LLMs at issue+ severity, that's near-certain real signal.

**Free-tier note:** semgrep community rules cover most OWASP Top-10
patterns for free, no account required. Pro rules (interprocedural
taint, secrets, supply-chain CVE matching) require a paid AppSec
Platform account but overlap heavily with what the LLM peers catch —
non-auth is the right default for jutsu's use case. See README.

## eslint-mcp (JavaScript linter)

```yaml
providers:
  eslint-mcp:
    driver: mcp
    transport: stdio
    command: npx
    args: ["eslint-mcp", "--config", ".eslintrc.json"]
    tool_name: analyze
```

ESLint's rule corpus is JavaScript-specific. Useful for projects with
a strict `.eslintrc` — eslint findings rarely overlap with what
generalist LLMs catch, so the disagreement table benefits from the
deterministic peer.

## Custom in-house analyzer

Any binary that speaks JSON-RPC 2.0 over stdio and implements the
`analyze` tool can be plugged in. Minimal protocol:

1. Receive `initialize` request, respond with capabilities.
2. Receive `notifications/initialized` (no response).
3. Receive `tools/call` with `name: "analyze"` and `arguments:
   {preset, severity_vocab, input}`. Respond with:
   ```json
   {
     "jsonrpc": "2.0", "id": <req.id>,
     "result": {
       "content": [{
         "type": "text",
         "text": "[{\"severity\":\"issue\",\"summary\":\"...\",\"file\":\"x.go\",\"line_range\":\"42\"}]"
       }]
     }
   }
   ```

The findings JSON inside `text` must conform to the preset's
severity vocabulary (passed in the request). Unknown severities are
discarded with `Result.Err` set — design decision per spec D7
("predictable failure beats silent relabeling").

See `testdata/stub_mcp_server/main.go` for a 100-line reference
implementation.

## Driver semantics

- **Cost:** `Result.CostUSD = 0` always (deterministic local execution).
- **CacheStatus:** `CacheUnsupported` (no provider-side prompt cache).
- **Tag in synthesizer output:** findings labeled `[deterministic]`
  to distinguish from LLM findings. Cross-driver consensus
  (LLM + MCP agreeing) is the highest-signal indicator the
  synthesizer surfaces.
- **Severity floor:** info-tier MCP findings are dropped before
  synthesis (deterministic-tool noise floor; spec MCP-as-peer
  acceptance criteria). minor+ findings appear in the disagreement
  table.
- **Out-of-vocab severity:** the MCP driver does NOT relabel; it
  surfaces `Result.Err: "MCP server '<name>' returned unknown
  severity '<sev>'"` and discards the entire response. Misconfigured
  servers fail loudly instead of polluting findings silently.

## Trust model

Project-layer `<repo>/.kaijutsu/agents.yaml` is checked into the repo.
Running `jutsu swarm` against a cloned repo executes the configured
MCP `command` as a child process. **This is the same trust model as
running `make`, `npm install`, or any tool that reads project-scoped
config.** Treat untrusted repos accordingly:

- Audit `agents.yaml` before first `jutsu swarm` against an unfamiliar
  repo. The existing per-repo consent gate (`.kaijutsu/<preset>.yaml`
  `allow-multi-model: true`) is your prompt to do this.
- Prefer `--global` MCP providers (in `~/.kaijutsu/agents.yaml`) for
  trusted analyzers; project-layer entries are easier to inject into.
- Inside CI: pass `JUTSU_REPO_SCAN_ROOTS=""` and avoid setting `--force`
  on `agent remove` so cross-repo reference checks fail loud rather
  than swallow misconfiguration.

## Limitations (v0.6 / Stage 6)

- **stdio transport only.** http transport (with optional secret
  headers via `headers:` env-var indirection) deferred to v0.6.x.
- **Severity vocabulary inferred from prompt content.** v0.6 uses a
  best-effort heuristic in `defaultSeverityVocab` that matches preset
  prompts to one of three vocabs. Stage 6.x will plumb the preset
  metadata explicitly so the server receives the correct vocab
  regardless of prompt phrasing.
- **No retries.** Failed MCP calls surface as `Result.Err` and are
  visible in the swarm disagreement table; the orchestrator does not
  retry with backoff. v0.7+ retry policy applies to all driver kinds.
