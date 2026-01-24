# Multi-Machine Sync Architecture

This document describes how fray works across multiple machines. It is an internal, implementation-focused overview.

## Goals

- **Zero‑conflict writes**: each machine only appends to its own files.
- **Eventual consistency**: all machines converge when shared files are synced.
- **Offline‑first**: no network dependency; sync happens externally (git/Syncthing/iCloud/Dropbox).
- **Deterministic rebuild**: all machines derive the same SQLite cache from the same shared data.
- **Local runtime**: sessions/presence/invoke config stay machine‑local.

## Storage Layout

```
.fray/
  fray-config.json                # syncs (storage_version >= 2)

  shared/                         # synced by your backend
    machines/
      <machine-id>/
        messages.jsonl            # message, updates, reactions, deletes
        threads.jsonl             # thread events + pins + moves
        questions.jsonl
        agent-state.jsonl         # ghost cursors, faves, roles, descriptors

  local/                          # never sync
    machine-id                    # {"id":"laptop","seq":123,"created_at":...}
    runtime.jsonl                 # agent registration, invoke config, sessions
    fray.db                       # local cache (rebuilt)
    fray.db-wal
    fray.db-shm
```

### Data Classification

**Synced (shared/machines/**`<id>`**):**
- Messages + updates + reactions + tombstones
- Threads + pins/mutes/subscriptions
- Questions
- Ghost cursors, faves, role assignments
- Agent descriptors

**Local (local/runtime.jsonl):**
- Agent registration/invoke config
- Sessions, heartbeats
- Presence + watermarks

## Event Model

Each machine appends to its own JSONL files. Events include:
- `origin`: machine id
- `seq`: per‑machine sequence (monotonic, with atomic increment)

**Merge ordering** is deterministic:
1. timestamp
2. machine id
3. sequence
4. file index (tie‑breaker)

**Sticky tombstones** (e.g., `message_delete`) prevent resurrection after offline merges.

## Read Path (Rebuild)

Rebuild reads **all** shared machine files plus local runtime data, then:
- merges messages with updates/deletes
- applies thread and question updates
- inserts agent descriptors (shared) and local agents (runtime)
- writes unified state into `fray.db`

Rebuild is triggered when JSONL mtimes are newer than the DB. The daemon can also watch for changes and auto‑rebuild.

## Write Path

Write routing depends on storage version:

- Messages, threads, questions, agent‑state → `shared/machines/<local>/…`
- Agent runtime, sessions, presence → `local/runtime.jsonl`

Writes are **append‑only** with:
- file‑level `flock`
- newline‑delimited JSON
- `fsync` for durability

Truncated trailing lines are discarded during read with a warning.

## Mention Encoding & Routing

Mentions are encoded with machine scope at write time:

| You type | Stored as | Who processes |
|---|---|---|
| `@opus` | `@opus@<this-machine>` | only local machine |
| `@opus@server` | `@opus@server` | server only |
| `@opus@all` | `@opus@all` | broadcast |

Machine aliases (from renames) are resolved during encoding and daemon processing.

## Display Rules

The UI shows `@agent@machine` **only when needed**:
- If an agent has posted from multiple origins, display includes the machine.
- Otherwise, display stays `@agent`.

This decision is based on **historical origins** (synced data), not live presence.

## Integrity & Safety

- **Atomic append** with `flock` + `fsync`.
- **Checksums**: `shared/checksums.json` stores per‑file SHA256 + line counts.
- **Truncated line recovery**: partial final line is ignored.
- **GUID collisions**: detected during rebuild, logged to `local/collisions.json` (no auto‑remediation).
- **Legacy write blocking**: v2 projects refuse to write to legacy files.

## Machine Identity

Each machine has a stable ID stored in `local/machine-id`:

```json
{"id":"laptop","seq":123,"created_at":1234567890}
```

Machine IDs must be unique across the shared `machines/` directory. Collision checks happen during init.

Renames:
- update `machine_aliases` in `fray-config.json`
- rename `shared/machines/<old>` → `<new>`
- update local machine‑id if renaming the current machine

## Sync Transport

Fray does **not** implement a transport. You sync `.fray/shared/` using git or another backend.  
When new data arrives, rebuild (or run the daemon with `--watch`).

## Current Limits / Deferred

- No cryptographic event signing yet (planned separately).
- Presence is local‑only by design.
- Transport health is not tracked; “remote” just means “not local.”
