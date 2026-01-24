# Multi-Machine Sync (Git) — How To

This guide walks through a basic multi‑machine setup using **git** for sync.

## 1) Initialize on the first machine

```bash
fray init
```

If you already have a legacy channel, migrate instead:

```bash
fray migrate --multi-machine
```

At this point you have:
- `.fray/shared/` (synced data)
- `.fray/local/` (local runtime + DB)

## 2) Ignore local data in git

Add these to your repo `.gitignore` (or `.fray/.gitignore`):

```
.fray/local/
.fray/*.db
.fray/*.db-wal
.fray/*.db-shm
```

## 3) Commit the shared state

```bash
git add .fray/shared .fray/fray-config.json
git commit -m "fray: initialize shared state"
git push
```

## 4) Join from another machine

```bash
git clone <your-repo>
cd <your-repo>
fray init
```

`fray init` detects an existing shared channel and guides you through:
- selecting a unique machine ID
- choosing which agents to run locally

If you want to run an agent that already exists in shared history:

```bash
fray agent add <agent-id>
```

## 5) Sync loop

Each machine writes only to its own folder:

```
.fray/shared/machines/<machine-id>/
```

So git merges are typically clean (append‑only JSONL per machine).

On each machine:

```bash
git pull
fray rebuild
```

Or keep the daemon running and auto‑rebuild on changes:

```bash
fray daemon --watch
```

## Tips

- Use `fray machines` to see what’s present in shared data.
- Use `fray machine rename <old> <new>` when you need to rename a machine.
- If a machine ID collides during init, just pick a different ID.

## Troubleshooting

**Legacy write blocked**  
If fray refuses to write because of legacy files, run:

```bash
fray migrate --multi-machine
```

**Nothing showing up after pull**  
Run `fray rebuild`, or keep `fray daemon --watch` running.
