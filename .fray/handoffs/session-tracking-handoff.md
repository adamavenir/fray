# Handoff: Session Tracking & Fork Spawning (fray-9ul2)

## Context
This work was started but lost due to a git stash mishap. The beads have been recreated as fray-9ul2 (P0 epic).

## Epic: fray-9ul2 - Session tracking and fork spawning

### Goals
1. Every message knows which session posted it (`session_id` field)
2. Session boundaries tracked for auditing (`LastMsgID` in SessionEnd)
3. Fork spawning via `@agent#sessid` syntax (`fork_sessions` field)
4. Graceful recovery when session resume fails

### Task Dependency Order
```
fray-9ul2.1 (schema) ─┬─> fray-9ul2.2 (capture session_id) ─┐
                      │                                      ├─> fray-9ul2.4 (post.go) ─> fray-9ul2.5 (daemon fork)
                      ├─> fray-9ul2.3 (parse @agent#sessid) ─┘
                      ├─> fray-9ul2.6 (fray get --session)
                      └─> fray-9ul2.7 (chat UI footer)

fray-9ul2.8 (LastMsgID boundary tracking) - independent
fray-9ul2.9 (graceful recovery) - independent
fray-9ul2.10 (fix daemon test) - independent
```

## Current State

### PARTIALLY COMPLETE: fray-9ul2.1 (schema)

Files modified:
- `internal/db/schema.go` - Added `session_id` and `fork_sessions` columns ✅
- `internal/types/types.go` - Added `SessionID` and `ForkSessions` to Message ✅
- `internal/db/queries_messages.go` - Updated columns, CreateMessage, messageRow, toMessage, scanMessage ✅
- `internal/db/jsonl.go` - Added fields to MessageJSONLRecord ✅
- `internal/db/jsonl_rebuild.go` - Updated INSERT statement ✅

Still need for fray-9ul2.1:
- `internal/db/jsonl_append.go` - Update AppendMessage to include ForkSessions

### NOT STARTED
- fray-9ul2.2: Capture CLAUDE_SESSION_ID in post.go
- fray-9ul2.3: ExtractMentionsWithSession in mentions.go
- fray-9ul2.4: Update post.go to use new mention extraction
- fray-9ul2.5: Daemon fork spawn handling in buildWakePrompt
- fray-9ul2.6: `fray get --session` flag
- fray-9ul2.7: Chat UI session ID in footer
- fray-9ul2.8: LastMsgID in SessionEnd
- fray-9ul2.9: Graceful recovery logic
- fray-9ul2.10: Fix daemon test (expects "@mentioned" but prompt says "You are @agent")

## Database Migration Required
After completing fray-9ul2.1, run:
```bash
fray rebuild
```
This recreates SQLite from JSONL with new schema.

## Key Design Decisions

### @agent#sessid Syntax
- `@opus#abc123` means "spawn opus with context from session abc123"
- Stored in `fork_sessions` map: `{"opus": "abc123"}`
- Daemon detects this and builds fork-specific wake prompt
- Fork = NEW session with visibility into prior session (not resume)

### Session Boundaries
- `triggered_by` = first message that triggered spawn (from)
- `last_msg_id` = last message agent could see (to)
- Graceful exit via `fray bye` records exact last_msg_id
- Ungraceful exit uses watermark as approximation

### Graceful Recovery
- Quick failure (exit=1, <30s, has LastSessionID) = likely resume failure
- Set presence to idle (not error) so next spawn is fresh
- Clear LastSessionID to prevent retry loop

## Test Issue
`TestSpawnFlow_DirectMention` in daemon_test.go expects prompt to contain "@mentioned" but actual prompt format is "You are @agent". This was a pre-existing test bug (failing before this work).

## Files to Watch
- `internal/daemon/daemon.go` - buildWakePrompt, handleProcessExit
- `internal/command/post.go` - mention extraction and message creation
- `internal/core/mentions.go` - ExtractMentionsWithSession
- `internal/chat/messages.go` - formatMessage footer
- `internal/command/get.go` - --session flag
- `internal/command/bye.go` - session_end recording

## Commands to Verify
```bash
go build ./...           # Must pass
go test ./...            # daemon tests may fail (pre-existing)
fray rebuild             # After schema changes
fray chat                # Verify no SQL errors
```
