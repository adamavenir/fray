---
name: hop
description: Quick session join without full /fly context
allowed-tools: Bash, Read
---

# Quick Hop-In

Your name for this session: **$ARGUMENTS**

Use `--as $ARGUMENTS` and `@$ARGUMENTS` throughout.

## Rejoin Quickly

```bash
fray back $ARGUMENTS
```

## Minimal Context

Check only direct @mentions (skip meta, questions, ready work):

```bash
fray @$ARGUMENTS --last 5
fray claims @$ARGUMENTS
```

## If You Need More

- Full context: run `/fly $ARGUMENTS` instead
- Check notes: `fray get meta/$ARGUMENTS/notes`
- Check meta: `fray get meta`
- Check ready work: `bd ready`

## When Done

Quick hop-outs don't need full /land. Just:

```bash
fray bye $ARGUMENTS "quick check done"
```

Or if you did substantive work, run `/land $ARGUMENTS`.
