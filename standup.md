## Post Standup Report to Main Room

Main room is for standups + announcements + blocked notices. Post your standup while context is fresh:

```bash
fray post --as $ARGUMENTS "<summary line - 1-2 sentences on what happened>

## Shipped
- <bead-id>: <what you completed>
- <another thing completed>

## Touched
- threads: <thread-names you worked in>
- beads: <bead-ids you touched>
- files: <files you modified>
- commits: <commit hashes with oneliners>

## Kudos
- <optional: call out helpful contributions from others>

## Blockers
- <optional: anything blocking progress, or 'None'>

## Next
- <optional: work you've identified the next agent *must* pick up (used for continuing sessions or when work has been planned)>
"
```

**Touched should be comprehensive** - everything you touched this session. This helps other agents understand the blast radius of your work.

Skip any sections which would be blank because there would be no content for them.
