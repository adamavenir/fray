# Implementation Plan: Job System for Parallel Agents

## Overview

Add a job system to fray that enables mlld scripts to spawn parallel agent workers for swarming tasks. Jobs are externally orchestrated (mlld controls spawn/termination), fray provides CLI primitives, storage, and activity panel display.

**Key design decisions (from parallel-agents thread):**
- Naming: `@agent[abc-n]` where `abc` is job ID suffix, `n` is worker index
- External orchestration: daemon does NOT manage job workers - mlld handles coordination
- Auto-created `job-<id>` thread for worker coordination
- Activity panel clusters job workers as `@agent × 5 [abc]` (collapsed by default)
- Bare `@agent` mentions blocked when job workers exist (must use `@agent[abc-n]`)

## ID Mapping (Canonical Reference)

| Entity | Format | Example | Notes |
|--------|--------|---------|-------|
| Job GUID | `job-<8char>` | `job-abc12345` | Stored in `fray_jobs.guid`, full identifier |
| Worker ID | `<agent>[<suffix>-<idx>]` | `dev[abc1-0]` | `suffix` = first 4 chars of job GUID suffix, `idx` = 0-based |
| Thread name | `job-<full-guid>` | `job-abc12345` | Uses complete job GUID for uniqueness |
| Display suffix | `[<suffix>-<idx>]` | `[abc1-0]` | Shortened for UI compactness |

**Worker ID construction:**
```
job-abc12345 + agent "dev" + idx 0 → dev[abc1-0]
job-xyz98765 + agent "pm.frontend" + idx 3 → pm.frontend[xyz9-3]
```

## Current State

No job system exists. Parallel agent coordination happens externally via mlld scripts without fray visibility or tracking.

## Goals

1. Enable mlld to register parallel workers with fray for coordination and visibility
2. Provide activity panel clustering so humans can see swarm status at a glance
3. Prevent mention routing ambiguity when multiple workers share a base agent name
4. Create audit trail via JSONL for job lifecycle events

## Storage

### New Table: `fray_jobs`

```sql
CREATE TABLE IF NOT EXISTS fray_jobs (
  guid TEXT PRIMARY KEY,           -- e.g., "job-abc123"
  name TEXT NOT NULL,              -- human-readable job name
  context TEXT,                    -- JSON blob with issues, threads, messages, etc.
  owner_agent TEXT,                -- agent who created the job
  status TEXT DEFAULT 'running',   -- running, completed, failed, cancelled
  thread_guid TEXT,                -- auto-created job-<id> thread
  created_at INTEGER NOT NULL,
  completed_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_fray_jobs_status ON fray_jobs(status);
```

### Extended `fray_agents` Table

Add columns for job workers:

```sql
-- Migration adds:
job_id TEXT,           -- FK to fray_jobs.guid (null for regular agents)
job_idx INTEGER,       -- worker index within job (0-based)
is_ephemeral INTEGER NOT NULL DEFAULT 0  -- 1 for job workers, 0 for persistent agents
```

### JSONL Records

New record types in `agents.jsonl`:
- `job_create` - job creation event
- `job_update` - status changes (completed, failed, cancelled)
- `job_worker_join` - worker joins job
- `job_worker_leave` - worker leaves job

## Commands

### `fray job create`

Create a new job with optional context.

```bash
fray job create "polish run" --context ./context.json --as coordinator
fray job create "qa sweep" --context '{"issues":["bd-abc"]}'
```

**Output:** Returns job ID (e.g., `job-abc123`) to stdout for mlld capture.

**Behavior:**
1. Generate job GUID with "job" prefix
2. Create `fray_jobs` record
3. Auto-create thread named `job-<id>` for coordination
4. Append `job_create` to `agents.jsonl`
5. Return job ID

### `fray job join`

Register a worker for a job (called by mlld in parallel loop).

```bash
fray job join job-abc123 --as dev --idx 0
```

**Output:** Returns full worker ID (e.g., `dev[abc-0]`) to stdout.

**Behavior:**
1. Validate job exists and is running
2. Check if worker exists - if offline, revive (upsert semantics)
3. Create/update ephemeral agent entry: `dev[abc1-0]`
4. Set `job_id`, `job_idx`, `is_ephemeral=1`
5. Mark presence as `active`
6. If `CLAUDE_ENV_FILE` set, write env vars:
   - `FRAY_AGENT_ID=dev[abc1-0]`
   - `FRAY_JOB_ID=job-abc12345`
   - `FRAY_JOB_IDX=0`
7. Append `job_worker_join` to `agents.jsonl`
8. Return worker ID

### `fray job leave`

Worker leaves job (called when task complete).

```bash
fray job leave --as dev[abc-0]
# Or uses FRAY_AGENT_ID env var
fray job leave
```

**Behavior:**
1. Mark worker presence as `offline`
2. Append `job_worker_leave` to `agents.jsonl`
3. Don't delete agent entry (keeps history)

### `fray job complete`

Mark job as completed or failed (called by mlld after parallel loop).

```bash
fray job complete job-abc123
fray job complete job-abc123 --status failed
```

**Flags:**
- `--status <status>` - Set status to `completed` (default) or `failed`

**Behavior:**
1. Update job status to specified value
2. Set `completed_at` timestamp
3. Append `job_update` to `agents.jsonl`

### `fray job status`

Show job status. JSON by default (for mlld consumption).

```bash
fray job status job-abc123
fray job status job-abc123 --text  # human-readable format
```

**Flags:**
- `--text` - Human-readable output instead of JSON

**Output (JSON, default):**
```json
{
  "guid": "job-abc12345",
  "name": "polish run",
  "status": "running",
  "workers": [
    {"id": "dev[abc1-0]", "presence": "active"},
    {"id": "dev[abc1-1]", "presence": "offline"}
  ],
  "active_count": 1,
  "total_count": 2
}
```

### `fray job list`

List active jobs.

```bash
fray job list
fray job list --all  # include completed
```

### `fray job cancel`

Cancel a running job.

```bash
fray job cancel job-abc123
```

## Mention Routing

### Disambiguation Required

When job workers exist for base agent `dev`, bare `@dev` is ambiguous:

```
@dev       → ERROR: "Multiple @dev workers active. Use @dev[abc-n] to specify."
@dev[abc-0] → Routes to specific worker
@dev[abc-*] → Future: broadcast to all workers in job (not MVP)
```

**Implementation:**
- Modify `extractMentions()` to detect ambiguous mentions
- Modify daemon wake logic to skip ambiguous mentions
- Add validation in `fray post` for ambiguous targets

### Parser Changes

Agent name regex needs to accept bracket suffix:

```go
// Current: alice, pm.3.sub.1
// New: alice, pm.3.sub.1, dev[abc-0], designer.frontend[xyz-3]
var agentIDPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]*(\[[a-z0-9]+-\d+\])?$`)
```

## Activity Panel

### Clustering Display

Job workers cluster in activity panel:

```
▶ opus                    # regular agent
▶ dev × 3 [abc1]          # 3 active workers for job-abc12345
  ▶ dev[abc1-0]           # (expanded view)
  ▶ dev[abc1-1]
  ▷ dev[abc1-2]           # idle
▽ designer                # regular agent offline
```

**Implementation in `panels.go:renderActivitySection()`:**

1. Group agents by `job_id` column from DB (not by parsing agent ID)
2. For each job group with workers, render collapsed cluster row
3. Track expansion state in `Model.expandedJobClusters map[string]bool`
4. Click on cluster toggles expansion
5. Aggregate presence: show dominant state icon, count per state

### Display Formatting

Use `job_id` and `job_idx` columns for grouping. Parsing only for display:

```go
func formatJobWorkerDisplay(agentID string, jobID string, jobIdx int) string {
    // Extracts display suffix from stored job_id for UI
    // job-abc12345 → "abc1" (first 4 chars of suffix)
    suffix := jobID[4:8] // "abc1" from "job-abc12345"
    return fmt.Sprintf("%s[%s-%d]", baseAgentName(agentID), suffix, jobIdx)
}
```

## Implementation Phases

### Phase 1: Storage & Core Types (≈4 hours)

1. **Schema migration** - `internal/db/schema.go`
   - Add `fray_jobs` table
   - Add columns to `fray_agents`: `job_id`, `job_idx`, `is_ephemeral`
   - Migration function for existing DBs

2. **Types** - `internal/types/types.go`
   - `Job` struct
   - `JobStatus` enum (running, completed, failed, cancelled)
   - `JobContext` struct for context JSON

3. **DB queries** - `internal/db/queries.go`
   - `CreateJob()`, `GetJob()`, `UpdateJobStatus()`
   - `GetJobWorkers()`, `GetActiveJobs()`
   - `IsAmbiguousMention()` - check if base agent has active job workers

4. **JSONL records** - `internal/db/jsonl.go`
   - `JobJSONLRecord`, `JobUpdateJSONLRecord`
   - `JobWorkerJoinJSONLRecord`, `JobWorkerLeaveJSONLRecord`
   - `AppendJobCreate()`, `AppendJobUpdate()`, etc.

**Exit criteria:**
- [ ] `go build ./...` passes
- [ ] `go test ./internal/db/...` passes
- [ ] Manual verification: can create job in DB

### Phase 2: Job Commands (≈4 hours)

1. **Job command group** - `internal/command/job.go`
   - `NewJobCmd()` parent command
   - `NewJobCreateCmd()`, `NewJobJoinCmd()`, `NewJobLeaveCmd()`
   - `NewJobCompleteCmd()`, `NewJobStatusCmd()`, `NewJobListCmd()`
   - `NewJobCancelCmd()`

2. **Auto thread creation** - reuse existing thread creation logic
   - Create `job-<id>` thread on job create
   - Store thread GUID in job record

3. **CLAUDE_ENV_FILE integration** - mirror `fray new` behavior
   - Write `FRAY_AGENT_ID` on `job join`

**Exit criteria:**
- [ ] `fray job create/join/leave/complete` work end-to-end
- [ ] Job thread auto-created
- [ ] `fray job status` returns correct JSON

### Phase 3: Mention Routing (≈3 hours)

1. **Agent ID parser** - `internal/core/mentions.go`
   - Update regex to accept `[id-n]` suffix
   - `ParseJobWorkerName()` function

2. **Ambiguity detection** - `internal/command/post.go`
   - Check for active job workers before posting
   - Error message with disambiguation hint

3. **Daemon skip** - `internal/daemon/daemon.go`
   - Skip ambiguous mentions in wake logic
   - Skip bare `@agent` when job workers active

**Exit criteria:**
- [ ] `fray post "@dev hello"` errors when `dev[abc-0]` exists
- [ ] `fray post "@dev[abc-0] hello"` works
- [ ] Daemon doesn't spawn on ambiguous mentions

### Phase 4: Activity Panel (≈4 hours)

1. **Clustering logic** - `internal/chat/panels.go`
   - `groupJobWorkers()` function
   - Modify `renderActivitySection()` for clusters
   - Add `expandedJobClusters` to Model

2. **Click handling** - `internal/chat/model.go`
   - Toggle cluster expansion on click

3. **Agent row rendering** - adapt existing `renderAgentRow()` for clusters

**Exit criteria:**
- [ ] Job workers display as collapsed cluster
- [ ] Click expands/collapses
- [ ] Individual worker rows show correct presence

### Phase 5: Testing & Documentation (≈2 hours)

1. **Integration tests** - `internal/command/job_test.go`
   - Full lifecycle: create → join → post → leave → complete
   - Concurrent workers
   - Mention disambiguation

2. **CLAUDE.md update** - add job commands to quick reference

3. **Changelog entry**

**Exit criteria:**
- [ ] `go test ./...` passes
- [ ] CLAUDE.md updated with job commands
- [ ] CHANGELOG.md entry added

## Total Estimated Effort: ≈17 hours

## Dependencies

- No external dependencies
- Existing code patterns for commands, JSONL, DB migrations
- mlld enhancement needed: env injection for `@claude()` (tracked separately)

## Risks & Mitigations

1. **Agent name collision** - Job workers use bracket syntax which is currently invalid for regular agents. Parsing must be strict.

2. **Stale job workers** - If mlld crashes, workers may remain "active" forever. Consider: `fray job gc` command to clean up stale workers.

3. **Activity panel performance** - With many job workers, clustering logic could be slow. Mitigate: cache grouping results, only recompute on agent change.

## Not in Scope (Future Work)

- Daemon-managed job orchestration
- `@agent[abc-*]` broadcast to all workers
- Job worker retry logic
- Cross-job worker migration
- Job templates

## Overall Exit Criteria

**Tests:**
- [ ] `go test ./...` passes (full suite)
- [ ] New tests cover job lifecycle, worker join/leave, mention disambiguation

**Behavior:**
- [ ] `fray job create/join/leave/complete/status/list/cancel` all work
- [ ] Activity panel clusters job workers correctly
- [ ] Bare `@agent` mentions blocked when job workers exist
- [ ] Daemon skips ambiguous mentions
- [ ] mlld can capture job/worker IDs from stdout
- [ ] `FRAY_AGENT_ID`, `FRAY_JOB_ID`, `FRAY_JOB_IDX` written on join

**Documentation:**
- [ ] CLAUDE.md updated with job commands
- [ ] CHANGELOG.md entry added

---

**Thread reference:** `parallel-agents` (thrd-kzt310ip)
**Technical review:** msg-t7f89n1q (@clank)
**Design synthesis:** msg-1380rnr0, msg-cykfpt6r, msg-abu32519, msg-ma9pjlul (@designer)
**Review feedback:** msg-znuu3a0z (@gpt52)
