# status.mld Specification

User-customizable status display for the activity panel using mlld.

## Overview

The activity panel displays agent presence and status. While **presence** (spawning, active, idle, offline, error) is system-managed, **status** messages are agent-controlled and can be customized via a `.fray/llm/status.mld` script.

## Two-Layer Display System

### 1. Presence (System-Managed) → Icon Color

Presence states determine the icon's base color:

| Presence | Color | Description |
|----------|-------|-------------|
| `spawning` | White | Agent process starting |
| `prompting` | White | Sending context to API |
| `prompted` | White | Receiving API response |
| `active` | Green | Agent actively working |
| `idle` | Yellow | Agent waiting (no activity) |
| `offline` | Gray | Agent not running |
| `error` | Red | Agent in error state |

### 2. Status (Agent-Managed) → Icon Shape + Message

When presence is `active` or `idle`, the status message determines icon shape and additional styling. The `status.mld` script parses status text and returns a `StatusDisplay` object.

## StatusDisplay Object

```typescript
interface StatusDisplay {
  // Icon
  icon?: string;           // Custom icon character (default: based on presence)
  iconcolor?: string;      // Override icon color (default: presence-based)

  // Agent name
  usrcolor?: string;       // Override agent name color (default: agent's color)

  // Message
  message?: string;        // Transformed status message
  msgcolor?: string;       // Override message color (default: dim gray)

  // Token progress bar
  usedtokcolor?: string;   // Color for used tokens (default: agent color)
  unusedtokcolor?: string; // Color for unused portion (default: bg)
  bgcolor?: string;        // Background color for entire row
}
```

All fields are optional. Unset fields use default values.

## Color Formats

Lipgloss supports multiple color formats:

| Format | Example | Notes |
|--------|---------|-------|
| Hex | `"#FF5500"`, `"#3C3C3C"` | Recommended - user-friendly |
| ANSI256 | `"21"`, `"157"`, `"216"` | 256-color palette codes |
| ANSI16 | `"0"` through `"15"` | Basic terminal colors |

**Terminal Compatibility**: Lipgloss auto-degrades colors for terminals that don't support TrueColor. Hex colors will be approximated to the nearest ANSI256/ANSI16 value.

## Example status.mld

```mlld
/import { @status } from @payload

/exe @parse(text) = when first [
  text.startsWith("blocked") => {
    icon: "⊘",
    iconcolor: "#FF0000",
    message: text.replace("blocked: ", ""),
    msgcolor: "#FF6666",
    usrcolor: "#FF6666",
    bgcolor: "#300000"
  }
  text.startsWith("awaiting") => {
    icon: "⧖",
    message: text.replace("awaiting: ", ""),
    msgcolor: "#FFFF00"
  }
  text.startsWith("investigating") => {
    icon: "🔍",
    message: text.replace("investigating: ", ""),
    msgcolor: "#00FFFF"
  }
  text.startsWith("testing") => {
    icon: "🧪",
    message: text.replace("testing: ", "")
  }
  * => {}  // empty = all defaults
]

/show @parse(@status)
/export { @parse }
```

## Default Icons

When no custom icon is returned:

| Presence | Icon |
|----------|------|
| `active`, `spawning`, `prompting`, `prompted` | `\|>` |
| `idle` | `<>` |
| `offline` | `--` |
| `error` | `!!` |

## Integration

1. Activity panel detects status message changes per agent
2. If status changed, invokes: `mlld .fray/llm/status.mld --payload '{"status": "..."}'`
3. Caches result until status changes again
4. Merges returned `StatusDisplay` with defaults
5. Renders agent row with customized styling

## Status Lifecycle

- `fray bye`: Clears status (fresh start next session)
- `fray back`: Clears status (clean slate)
- `active → idle`: Status persists (same task, just waiting)
- `fray status @agent "message"`: Updates status anytime

## Visual Example

```
🟢 ⊘ opus: blocked: waiting on API access     (active + blocked → green icon, red row)
🟡 ⧖ pm: awaiting PR review                   (idle + awaiting → yellow hourglass)
🟢 |> dev: working on feature X               (active + no match → green default)
⚪ |> tok: (spawning)                          (spawning → white default)
🔴 !! architect: build failed                  (error → red !!)
⬜ -- designer: offline                        (offline → gray --)
```

## File Location

```
.fray/
  llm/
    status.mld    # Custom status parsing script
```

If `status.mld` doesn't exist, all defaults apply.
