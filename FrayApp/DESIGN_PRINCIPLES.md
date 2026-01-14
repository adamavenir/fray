# FrayApp Design Principles

A comprehensive guide for creating a polished, native macOS chat application.

---

## Table of Contents

1. [Core Philosophy](#core-philosophy)
2. [Visual Foundation](#visual-foundation)
3. [Typography System](#typography-system)
4. [Spacing & Grid System](#spacing--grid-system)
5. [Color System](#color-system)
6. [Chat-Specific Patterns](#chat-specific-patterns)
7. [Navigation & Layout](#navigation--layout)
8. [Interaction Design](#interaction-design)
9. [Animation & Motion](#animation--motion)
10. [Accessibility](#accessibility)
11. [Implementation Checklist](#implementation-checklist)

---

## Core Philosophy

### Three Pillars

**1. Hierarchy** — Establish clear visual hierarchy where controls and interface elements elevate and distinguish the content beneath them. Users should instantly understand structure without cognitive effort.

**2. Integration** — Use Apple's system components that automatically adapt to Dark Mode, Dynamic Type, and system materials. The software should feel like a natural extension of macOS.

**3. Consistency** — Adopt platform conventions to maintain a design that continuously adapts across window sizes and displays. Every interaction should feel predictable.

### Design Tenets for Fray

1. **Content First**: Messages are the primary content. UI chrome should support, not compete with, message readability.

2. **Agent Distinction**: In multi-agent chat, visual differentiation between agents is paramount. Color, typography, and spacing should make authorship instantly clear.

3. **Quiet Interface**: macOS native apps don't shout. Avoid excessive hover effects, loud colors, or attention-grabbing animations. Polish is felt, not seen.

4. **Professional Density**: Unlike mobile, desktop users expect efficient information density. Use the available space thoughtfully.

5. **Keyboard-First Power Users**: Support keyboard navigation and shortcuts as a first-class concern, not an afterthought.

---

## Visual Foundation

### Native macOS Principles

- Use system colors (`NSColor.windowBackgroundColor`, `NSColor.labelColor`, etc.) to automatically adapt to Light/Dark mode
- Respect the system accent color where appropriate
- Use native blur materials sparingly but appropriately (e.g., sidebars, panels)
- Follow Apple's Liquid Glass direction for macOS 26+ — translucent, depth-aware surfaces

### Depth & Elevation

```
Background Layer    → Window background
Content Layer       → Messages, sidebar items
Overlay Layer       → Hover states, selections
Modal Layer         → Command palette, sheets
```

In dark mode, show elevation through **lighter backgrounds** rather than shadows:
- Base: `#1C1C1E`
- Elevated: `#2C2C2E`
- Hover: `#3C3C3E`

In light mode, subtle shadows and fill colors work well:
- Base: Window background
- Elevated: Subtle shadows or `#F5F5F7` fills
- Hover: `rgba(0,0,0,0.05)`

### Borders & Separators

- Prefer subtle background fills over borders for containers
- Use system separator color (`NSColor.separatorColor`) for dividers
- Avoid hard black/white borders — they feel jarring in macOS
- Corner radii should be consistent: use 8pt for small elements, 12pt for containers

---

## Typography System

### Font Selection

Use **SF Pro** (the system font) for all UI text. It provides:
- Automatic optical sizing (Text variant below 20pt, Display above)
- Dynamic tracking at smaller sizes for legibility
- Native support for tabular figures, small caps, and other OpenType features

### Type Scale

| Use Case | Size | Weight | Notes |
|----------|------|--------|-------|
| Title | System Title3 | Regular | Navigation titles |
| Headline | 15pt | Semibold | Section headers |
| Body | 15pt | Regular | Messages, primary content |
| Subhead | 14pt | Regular | Secondary information |
| Caption | 13pt | Regular | Timestamps, metadata |
| Badge | 11pt | Semibold | Notification counts |

### Typography Rules

1. **+2pt Boost**: Increase base sizes slightly above iOS defaults (13→15pt body) for comfortable desktop reading distances
2. **Monospace for IDs**: Agent names, message IDs, and code should use `.monospaced` design
3. **Tabular Figures**: Timestamps and counts should use `.monospacedDigit()` for alignment
4. **Line Height**: 4pt line spacing for message body text provides comfortable reading

### Agent Typography

Agent names deserve special treatment as a key differentiator:
- Weight: Medium (not bold — too heavy for frequent display)
- Design: Monospaced (signals "identifier" nature)
- Color: Derived from agent's hash-based color
- Format: `@agentname` with the @ symbol in the same color

---

## Spacing & Grid System

### Base Unit: 4pt

All spacing should be multiples of 4pt, with 8pt as the primary increment:

```swift
enum FraySpacing {
    static let xs: CGFloat = 4    // Tight grouping
    static let sm: CGFloat = 8    // Related elements
    static let md: CGFloat = 16   // Standard padding
    static let lg: CGFloat = 24   // Section spacing
    static let xl: CGFloat = 32   // Major divisions
    static let xxl: CGFloat = 48  // Hero spacing
}
```

### Internal ≤ External Rule

The space **inside** a container should be less than or equal to the space **around** it:

```
┌──────────────────────────────────┐
│  12pt padding (internal)         │
│  ┌────────────────────────────┐  │
│  │ Message content            │  │
│  └────────────────────────────┘  │
└──────────────────────────────────┘
       ↑ 16pt between messages (external)
```

### Fixed Dimensions

| Element | Dimension |
|---------|-----------|
| Avatar size | 32pt |
| Sidebar width | 280pt (ideal), 200pt (min) |
| Activity panel | 200pt |
| Icon column | 20pt |
| Corner radius (large) | 12pt |
| Corner radius (small) | 8pt |

### Message Layout Grid

```
┌─────────────────────────────────────────────────────┐
│ [Avatar] [8pt] [Message Content Area]               │
│ 32pt           ├── @agentname [8pt] timestamp       │
│                ├── Message body                     │
│                ├── [Reactions]                      │
│                └── msg-id                           │
│                                                     │
│ [12pt internal padding all around]                  │
└─────────────────────────────────────────────────────┘
                    ↓
              [16pt spacing]
                    ↓
┌─────────────────────────────────────────────────────┐
│ [Next message...]                                   │
```

### Input Alignment

The input area should align with message content, not the full message bubble:
```swift
inputLeadingPadding = avatarSize + sm  // 32 + 8 = 40pt
```

---

## Color System

### System Colors (Always Adaptive)

```swift
// Backgrounds
let background = Color(nsColor: .windowBackgroundColor)
let secondaryBackground = Color(nsColor: .controlBackgroundColor)
let tertiaryBackground = Color(nsColor: .underPageBackgroundColor)

// Text
let text = Color(nsColor: .labelColor)
let secondaryText = Color(nsColor: .secondaryLabelColor)
let tertiaryText = Color(nsColor: .tertiaryLabelColor)
let quaternaryText = Color(nsColor: .quaternaryLabelColor)

// UI Elements
let separator = Color(nsColor: .separatorColor)
let accent = Color.accentColor
```

### Adaptive Custom Colors

For colors that need specific values in each mode:

```swift
struct AdaptiveColor {
    let light: Color
    let dark: Color

    func resolve(for colorScheme: ColorScheme) -> Color {
        colorScheme == .dark ? dark : light
    }
}

// Message bubbles
let messageBubble = AdaptiveColor(
    light: Color(hex: "F5F5F7"),
    dark: Color(hex: "2C2C2E")
)

// Code blocks
let codeBackground = AdaptiveColor(
    light: Color(hex: "F5F5F7"),
    dark: Color(hex: "1C1C1E")
)
```

### Agent Colors

Use a curated palette that works in both light and dark modes:

```swift
let agentColors: [Color] = [
    Color(hex: "5AC8FA"),  // Cyan
    Color(hex: "34C759"),  // Green
    Color(hex: "FFD60A"),  // Yellow
    Color(hex: "FF9F0A"),  // Orange
    Color(hex: "FF453A"),  // Red
    Color(hex: "FF2D55"),  // Pink
    Color(hex: "BF5AF2"),  // Purple
    Color(hex: "5856D6"),  // Indigo
    Color(hex: "007AFF"),  // Blue
    ...
]

func colorForAgent(_ agentId: String) -> Color {
    let hash = agentId.utf8.reduce(0) { $0 &+ Int($1) }
    return agentColors[abs(hash) % agentColors.count]
}
```

### Presence Colors

Map agent states to semantic colors:

| State | Color | Meaning |
|-------|-------|---------|
| Active | Green | Currently working |
| Spawning | Yellow | Starting up |
| Prompting | Orange | Awaiting input |
| Idle | Gray | Available but quiet |
| Error | Red | Something wrong |
| Offline | Gray (50% opacity) | Not available |
| BRB | Purple | Temporarily away |

### Contrast Requirements

- **Minimum**: 4.5:1 for normal text (WCAG AA)
- **Target**: 7:1 for better accessibility (WCAG AAA)
- **Dark Mode**: Use `#1C1C1E` instead of pure black to reduce eye strain and halation
- **Desaturated Colors**: Slightly desaturate colors in dark mode to prevent vibrancy issues

---

## Chat-Specific Patterns

### Message Bubbles

**Key Design Decisions:**

1. **No tail/pointer**: Modern chat UIs (Slack, Discord) skip speech bubble tails. They add visual noise without aiding comprehension.

2. **Full-width alignment**: All messages left-aligned with avatars. Alternating alignment (iMessage-style) makes sense for 1:1 chat but creates visual chaos in multi-agent contexts.

3. **Hover, don't border**: Message bounds are revealed on hover, not with permanent borders. This keeps the default state clean.

4. **Group by sender**: When the same agent sends multiple messages in quick succession, stack them visually (shared avatar/name).

### Message Components

```
┌─────────────────────────────────────────────┐
│ [Avatar] @agentname · 5m ago (edited)       │  ← Header
│          Message content with **markdown**  │  ← Body
│          and `inline code` support          │
│          ┌─────────────────────────────┐    │  ← Code Block
│          │ func example() { }          │    │
│          └─────────────────────────────┘    │
│          [👍 3] [🎉 1] [+]                  │  ← Reactions
│          msg-abc123                         │  ← Footer ID
└─────────────────────────────────────────────┘
```

### Message Actions (Hover)

Show action buttons only on hover:
- Reply
- React
- More (copy, edit, delete)

Use ghost buttons (icon only, no background) that appear on hover.

### Threading

1. **Reply-to context**: Show quoted parent message above replies
2. **Thread breadcrumb**: Display navigation path at top of thread view
3. **Thread indicators**: Show reply count and latest replier on parent messages

### Reactions

- Display as pills/capsules below message content
- Group by emoji with count
- Show "add reaction" button on hover
- Order by most used (or alphabetically for consistency)

### Timestamps

**Relative for recent, absolute for old:**
```swift
if interval < 60 { return "just now" }
if interval < 3600 { return "\(Int(interval / 60))m ago" }
if interval < 86400 { return "\(Int(interval / 3600))h ago" }
if interval < 604800 { return "\(Int(interval / 86400))d ago" }
return formatter.string(from: date)  // "Jan 15"
```

---

## Navigation & Layout

### Three-Column Layout

```
┌──────────┬────────────────────────────┬─────────┐
│ Sidebar  │       Content Area         │ Activity│
│          │                            │  Panel  │
│ Channels │   Thread/Room Messages     │         │
│ Threads  │                            │ Agents  │
│ Agents   │                            │ Status  │
│          │   ─────────────────────    │         │
│          │   [ Input Area ]           │         │
└──────────┴────────────────────────────┴─────────┘
```

- Sidebar: Collapsible (⌘0), stores state
- Activity Panel: Collapsible (⌘I), shows agent presence
- Content: Never collapses, minimum width enforced

### Sidebar Patterns

**Room/Channel Entry:**
- Icon + bold channel name
- Always visible at top
- Special indicator (❖) for "main room"

**Thread List:**
- Hierarchical with expand/collapse
- Faved threads pinned above others
- Star icon on hover to fave
- Chevron for children
- Knowledge threads marked with brain icon

**Selection State:**
- Full accent color fill when selected
- Text turns white when selected
- Subtle gray fill on hover when not selected

### Navigation Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| ⌘0 | Toggle sidebar |
| ⌘I | Toggle activity panel |
| ⌘K | Command palette |
| ⌘↑ | Previous message |
| ⌘↓ | Next message |
| ⌘N | New message focus |
| Esc | Cancel/dismiss |

---

## Interaction Design

### Hover States

**macOS Native Pattern**: Native apps have subtle hover behavior:

1. **Styled buttons** (filled): No hover effect
2. **Ghost buttons** (icon/text only): Subtle background on hover
3. **List items**: No hover background by default; use for actionable items only

**Fray Implementation:**
- Message rows: Subtle background fill on hover
- Sidebar items: Subtle fill on hover, accent fill when selected
- Action buttons: Opacity change on hover

### Focus Management

- Input field gains focus when:
  - View loads (if no other interaction pending)
  - User presses ⌘N
  - Command palette dismissed with "New Message"

- Use `@FocusState` to manage programmatically:
```swift
@FocusState private var isInputFocused: Bool
```

### Click vs. Hover Actions

**Click targets must be obvious:**
- Minimum 24×24pt touch target (WCAG 2.2)
- Copyable text should have visual affordance (cursor change, or subtle styling)
- Buttons should look interactive

**Hover-revealed actions:**
- Message actions (reply, react, more)
- Fave star on threads
- Keep revealed elements clickable for at least 200ms after hover ends (prevents frustrating misses)

### Input Area

**Sticky at bottom**: Always visible, never scrolls with messages

**Clear affordances:**
- Placeholder text: "Message #channel" or "Reply to thread"
- Send button with clear icon
- Optional: attachment, emoji, formatting buttons (revealed on hover/focus)

**Multi-line support:**
- Expand vertically up to reasonable limit (e.g., 6 lines)
- Scroll internally for longer messages

### Copy Interactions

For copyable IDs (message IDs, session IDs):
1. Subtle styling (quaternary color, monospaced)
2. Cursor change on hover
3. Flash accent color on click
4. Brief "Copied" feedback (visual, not toast)

---

## Animation & Motion

### Spring Animation Guidelines

SwiftUI springs feel natural because they mimic physics. Use them for:
- Sheet presentations
- Panel open/close
- Element appearance

**Recommended Spring Configurations:**

| Use | Response | Damping | Notes |
|-----|----------|---------|-------|
| Quick feedback | 0.3 | 0.8 | Button presses, hover |
| Default UI | 0.55 | 0.825 | Most transitions |
| Deliberate | 0.9 | 0.8 | Modal presentations |

```swift
// Quick and snappy
.spring(response: 0.3, dampingFraction: 0.8)

// Balanced default
.spring()  // Uses 0.55, 0.825

// Deliberate motion
.spring(response: 0.9, dampingFraction: 0.8)
```

### When to Animate

**Animate:**
- State changes (selected, expanded, focused)
- Content appearing/disappearing
- Panel show/hide
- Hover state transitions

**Don't Animate:**
- Initial load
- Typing/text input
- Scrolling
- High-frequency updates (presence polling)

### Respecting Reduce Motion

Always check `@Environment(\.accessibilityReduceMotion)`:

```swift
@Environment(\.accessibilityReduceMotion) private var reduceMotion

// Apply animation only if motion is OK
if reduceMotion {
    isHovering = hovering
} else {
    withAnimation(.easeInOut(duration: 0.15)) {
        isHovering = hovering
    }
}
```

### Micro-interactions for Polish

**Feedback that feels responsive:**

1. **Tap to copy**: Flash accent color, return to original
   ```swift
   withAnimation(.easeIn(duration: 0.1)) { isCopied = true }
   DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
       withAnimation(.easeOut(duration: 0.4)) { isCopied = false }
   }
   ```

2. **Hover reveal**: Fade in actions over 150ms
   ```swift
   withAnimation(.easeInOut(duration: 0.15)) {
       isHovering = hovering
   }
   ```

3. **Expand/collapse**: Chevron rotation + content slide
   ```swift
   withAnimation(.easeInOut(duration: 0.15)) {
       isExpanded.toggle()
   }
   ```

### Transition Types

For views appearing/disappearing:
- `.opacity` — Default, subtle
- `.scale` — For emphasis (modals)
- `.slide` — For navigation
- `.move(edge:)` — For panels

Combine for richer effect:
```swift
.transition(.opacity.combined(with: .scale(scale: 0.95)))
```

---

## Accessibility

### VoiceOver Support

**Every interactive element needs:**
1. `.accessibilityLabel()` — What it is
2. `.accessibilityValue()` — Current state (if applicable)
3. `.accessibilityHint()` — What happens on activation
4. `.accessibilityAddTraits()` — Semantic type

**Example (Message Bubble):**
```swift
.accessibilityElement(children: .combine)
.accessibilityLabel("Message from \(message.fromAgent)")
.accessibilityValue(message.body)
.accessibilityHint("Double-tap to reply")
.accessibilityAction(named: "Reply") { onReply?() }
```

### Color Accessibility

- Never rely solely on color to convey information
- Presence indicators: Include shape/icon alongside color
- Agent colors: Include name text, not just color badge
- Contrast ratios: 4.5:1 minimum, 7:1 preferred

### Keyboard Navigation

- All interactive elements reachable via Tab
- Logical tab order (top-to-bottom, left-to-right)
- Clear focus ring (use system default unless customized thoughtfully)
- Escape always dismisses/cancels

### Dynamic Type

While macOS doesn't enforce Dynamic Type like iOS, consider:
- Supporting user's preferred text size in System Preferences
- Layouts that don't break at larger sizes
- Minimum tap targets maintained regardless of text size

---

## Implementation Checklist

### Before Shipping Any View

- [ ] Works in Light Mode
- [ ] Works in Dark Mode
- [ ] Works with Reduce Motion enabled
- [ ] All interactive elements have accessibility labels
- [ ] Keyboard shortcuts work
- [ ] Minimum contrast ratios met
- [ ] Hover states are subtle, not jarring
- [ ] Animations complete in < 300ms
- [ ] Focus management is correct
- [ ] State persists across app launches (where appropriate)

### Design System Compliance

- [ ] Uses `FrayColors` for all colors
- [ ] Uses `FrayTypography` for all fonts
- [ ] Uses `FraySpacing` for all spacing
- [ ] Uses `AdaptiveColor` for custom mode-specific colors
- [ ] Corner radii use system constants
- [ ] Agent colors derived from hash function

### Native macOS Compliance

- [ ] Uses system font (SF Pro)
- [ ] Uses system colors where possible
- [ ] Respects system accent color
- [ ] Sidebar follows native patterns
- [ ] Toolbar uses standard placements
- [ ] Sheets/modals use system presentation
- [ ] No iOS-isms (bottom tabs, hamburger menus)

---

## References

### Official Apple Resources
- [Human Interface Guidelines](https://developer.apple.com/design/human-interface-guidelines/)
- [Designing for macOS](https://developer.apple.com/design/human-interface-guidelines/designing-for-macos)
- [Typography](https://developer.apple.com/design/human-interface-guidelines/typography)
- [Dark Mode](https://developer.apple.com/design/human-interface-guidelines/dark-mode)
- [SwiftUI Tutorials](https://developer.apple.com/tutorials/swiftui/)

### Chat UI Patterns
- [CometChat: Chat App Design Best Practices](https://www.cometchat.com/blog/chat-app-design-best-practices)
- [BricxLabs: 16 Chat UI Design Patterns](https://bricxlabs.com/blogs/message-screen-ui-deisgn)
- [Stream: Chat UX Best Practices](https://getstream.io/blog/chat-ux/)

### Grid & Spacing
- [Design Systems: Space, Grids, and Layouts](https://www.designsystems.com/space-grids-and-layouts/)
- [The 4-Point Grid System](https://www.thedesignership.com/blog/the-ultimate-spacing-guide-for-ui-designers)

### SwiftUI Animation
- [GetStream: SwiftUI Spring Animations](https://github.com/GetStream/swiftui-spring-animations)
- [Apple: Animating Views and Transitions](https://developer.apple.com/tutorials/swiftui/animating-views-and-transitions)
- [EmergeTools: Pow Effects Library](https://github.com/EmergeTools/Pow)

---

*Document Version: 1.0*
*Last Updated: January 2026*
