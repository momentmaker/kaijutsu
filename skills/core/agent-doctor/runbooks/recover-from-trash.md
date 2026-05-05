# Runbook: Recover from Trash

User said "I think I needed that file you cleaned up." Walk through this.

Each agent dir has its own `.trash/`. Identify which one first.

## 0. Identify the agent

Trash lives at `<agent-dir>/.trash/`. If you don't know which agent's dir held the file, check all four:

```bash
for d in ~/.claude ~/.codex ~/.gemini ~/.agents; do
  [[ -d "$d/.trash" ]] && echo "$d/.trash"
done
```

The rest of this runbook uses `$AGENT_DIR` — set it to the right one before running:

```bash
export AGENT_DIR=~/.claude   # or ~/.codex, etc.
```

## 1. List trash batches

```bash
ls -la "$AGENT_DIR/.trash/"
```

Each top-level dir is one cleanup batch, named `YYYY-MM-DD-HHMMSS`. Inside, original relative paths are preserved.

Example layout:

```
~/.claude/.trash/2026-05-04-141530/
  paste-cache/abc123.txt
  telemetry/1p_failed_events.xxx.json
  projects/-Users-rubberduck-GitHub-momentmaker-old-repo/
    abc-session.jsonl
    xyz-session.jsonl
```

## 2. Find the file

If the user remembers part of the name:

```bash
find "$AGENT_DIR/.trash" -name '*partial-name*'
```

If they want the most recent batch:

```bash
ls -1t "$AGENT_DIR/.trash" | head -1
```

## 3. Restore

Path is preserved relative to `$AGENT_DIR`, so you can move it back into place. **Important:** if cleanup also removed the parent directory (e.g. an entire `paste-cache/` purge), recreate the parent first or `mv` will fail.

```bash
# Restore one file (mkdir -p in case the parent dir was also trashed)
mkdir -p "$AGENT_DIR/paste-cache"
mv "$AGENT_DIR/.trash/2026-05-04-141530/paste-cache/abc123.txt" \
   "$AGENT_DIR/paste-cache/"

# Restore an entire batch — rsync handles parent-dir creation automatically:
rsync -a "$AGENT_DIR/.trash/2026-05-04-141530/" "$AGENT_DIR/"
```

## 4. Confirm + clean batch

After confirming the restore worked, optionally remove the empty/no-longer-needed batch:

```bash
# If empty:
rmdir "$AGENT_DIR/.trash/2026-05-04-141530/"

# Or if the user just wants it gone:
rm -rf "$AGENT_DIR/.trash/2026-05-04-141530/"
```

## When trash has been purged

If the file was in a batch older than 30 days **and** `--tier aggressive --apply` has run since, the batch was hard-deleted. **It's gone.** Tell the user honestly. Don't pretend.

For session jsonls specifically: check whether the user has another machine with the same agent dir (Dropbox / iCloud sync). If not, the conversation is unrecoverable.

## Prevention

If the same kind of file keeps getting recovered, **add it to the protected list**. Edit `<this-skill>/scripts/lib.sh` and append the path to the `compute_protected()` function. The cleanup logic respects it immediately on next run.
