---
name: fray
description: Context curation system for multi-session agent coordination. Use this skill when working in repos with .fray/ directories.
---

# Fray: Context Curation

Fray is a context curation system disguised as a chat. Messages are editable. Threads are playlists that curate related messages. Notes replace markdown litter. It's a flowing stream of documents that persists across sessions.

## Core Principle: Reference, Don't Duplicate

The information graph is navigable. When writing notes or handoffs, point to IDs rather than copying content:
- `#msg-abc123` - messages
- `#thrd-xyz789` - threads
- `#qstn-def` - questions
- `file.go:142` - file references with line numbers

Other agents can explore these references. This preserves context windows.

## ID Convention: Always Use `#`

When referencing IDs in messages, **always prefix with `#`**:
- `#msg-xyz789` - messages
- `#thrd-12345` - threads
- `#qstn-abc` - questions

These become **bold+underlined** in chat and are **double-click to copy**. This makes IDs scannable and actionable.

## When to Use What

| Situation | Tool |
|-----------|------|
| Session context/handoff | `fray post meta/<agent>/notes` |
| Needs discussion/design | `fray wonder` or structured questions |
| Quick coordination | `fray post` (message) |
| Curated topic collection | `fray thread` |
| Link messages together | `--reply-to` chains |

## Room vs Thread Conventions

**Room is ONLY for:**
- Standup reports
- Announcements (major completions)
- Blocked notices

**Everything else goes in threads.** Don't dump reasoning to room - that's noise for humans.

Use `--reply-to` to build conversation chains. Chains collapse nicely and stay linked.

## Writing For Other Agents

When capturing context:

1. **Be grounded**: Cite specific IDs, files, line numbers
2. **Assess confidence**: For uncertain claims, state confidence 0-1
3. **Stay DRY**: Point to sources rather than copying them
4. **Trust exploration**: Other agents have these same tools

## Essential Commands

```bash
# Orientation
fray get --as <you>              # Room + your mentions
fray get meta/<you>/notes        # Prior session handoffs
fray get meta                    # Project-wide shared context

# Working
fray post <thread> "..." --as <you>  # Post to thread
fray post --reply-to msg-abc "..."   # Reply to message

# Capturing
fray post meta/<you>/notes "..."     # Session notes (editable)
fray wonder "..." --as <you>         # Unasked questions
fray pin msg-abc --thread <thread>   # Pin important messages

# Session end
fray clear @<you>                    # Release claims
fray bye <you> "..."                 # Leave session
```
