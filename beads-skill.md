---
name: beads
description: Git-backed issue tracker with dependency graph for AI coding agents. Use this skill when working in repos with .beads/ directories.
---

# Beads: Issue Tracking with Dependencies

Beads (`bd`) tracks work items with dependencies. Issues flow through states:
`blocked → ready → in_progress → closed`

## Core Principle: Work on Ready Issues

**Only work on `bd ready` issues** unless explicitly asked otherwise. This ensures you're not blocked by dependencies.

```bash
bd ready                          # Shows issues with all deps resolved
bd list --blocked                 # Shows blocked issues (for context)
```

## Dependency Graph

Issues can depend on other issues. The graph determines what's ready:

```bash
bd deps add fray-abc fray-xyz    # fray-abc depends on fray-xyz
bd deps rm fray-abc fray-xyz     # Remove dependency
bd show fray-abc                 # Shows issue with its dependencies
```

Work flows through the graph: as dependencies close, blocked issues become ready.

## When to Use What

| Situation | Tool |
|-----------|------|
| Multi-session work item | `bd create` |
| Blocking another issue | `bd deps add` |
| Starting work | `bd update <id> --status in_progress` |
| Completing work | `bd close <id> --reason "..."` |
| Discovered work | `bd create "..." --type task` |

## Essential Commands

```bash
# Orientation
bd ready                                 # Unblocked issues
bd list --status in_progress             # What's being worked on

# Working
bd update fray-abc --status in_progress  # Start work
bd close fray-abc --reason "..."         # Complete with reason

# Creating
bd create "fix auth bug" --type bug      # Create issue
bd create "..." --label discuss          # Needs design input
bd create "..." --label idea             # Floating idea

# Dependencies
bd deps add fray-abc fray-xyz            # fray-abc depends on fray-xyz
bd deps rm fray-abc fray-xyz             # Remove dependency
```

## Issue Types

- `bug` - something broken
- `task` - work to be done
- `feature` - new functionality
- `chore` - maintenance

## Labels

- `discuss` - needs design input before implementation
- `idea` - floating idea, not committed
- `blocked` - blocked by external factor (not a beads dependency)
