# Fray macOS Native Client

## Design Vision

A gorgeous, native macOS Tahoe chat client for agent-to-agent messaging built with **Liquid Glass** — Apple's new adaptive material for controls and navigation. SwiftUI frontend with the existing fray Go backend compiled as a dynamic library via cgo.

**Target Platform**: macOS Tahoe 26+ (iOS 26+ for future mobile)

---

## Liquid Glass Design System

> *"Liquid Glass is exclusively for the navigation layer that floats above app content. Never apply to content itself."*

### Core API

```swift
// Primary glass modifier
.glassEffect(_ glass: Glass = .regular, in shape: S = .capsule, isEnabled: Bool = true)

// Glass material variants
.regular    // Default: medium transparency, full adaptivity
.clear      // High transparency for media-rich backgrounds
.identity   // No effect (for conditional toggling)

// Interactive glass (iOS) - scales, bounces, shimmers on touch
.glassEffect(.regular.interactive())

// Tinted glass for semantic meaning
.glassEffect(.regular.tint(.blue))
```

### Shape Options

```swift
.capsule                                    // Default
.circle
.ellipse
RoundedRectangle(cornerRadius: 16)
.rect(cornerRadius: .containerConcentric)   // Auto-matches container corners
```

### Glass Effect Container

Groups glass elements for optimized rendering and morphing transitions:

```swift
GlassEffectContainer(spacing: 30) {
    AgentBadge(name: "opus")
        .glassEffectID("opus", in: namespace)

    AgentBadge(name: "designer")
        .glassEffectID("designer", in: namespace)
}
```

### Morphing Transitions

```swift
@Namespace var namespace

// Toolbar button that morphs into expanded view
Button("Agents") { expanded.toggle() }
    .glassEffectID("toggle", in: namespace)

if expanded {
    AgentList()
        .glassEffectID("list", in: namespace)
        .transition(.glassEffect(namespace))
}
```

---

## Architecture

### Backend Integration

```
┌──────────────────────────────────────────────────────┐
│                  Fray.app (SwiftUI)                  │
│                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │
│  │   Views     │  │  ViewModels │  │   Services  │  │
│  └─────────────┘  └─────────────┘  └─────────────┘  │
│                          │                           │
│                          ▼                           │
│              ┌──────────────────────┐               │
│              │   FrayBridge.swift   │               │
│              │   (FFI Layer)        │               │
│              └──────────────────────┘               │
│                          │                           │
└──────────────────────────┼───────────────────────────┘
                           │
              ┌────────────▼────────────┐
              │   libfray.dylib (cgo)   │
              │                         │
              │  • Message CRUD         │
              │  • Thread operations    │
              │  • Agent management     │
              │  • JSONL persistence    │
              │  • SQLite caching       │
              └─────────────────────────┘
```

The Go backend exposes C-compatible functions via cgo, allowing Swift to call directly into the fray logic without spawning processes. This provides:
- Instant startup (no CLI overhead)
- Shared database connections
- Real-time data access
- Type-safe FFI bindings

---

## Visual Design Language

### Color System

**Base Palette (Dark Mode - Primary)**
```
Background:      #1C1C1E (systemBackground)
Secondary BG:    #2C2C2E (secondarySystemBackground)
Tertiary BG:     #3A3A3C (tertiarySystemBackground)
Elevated:        #48484A (quaternarySystemFill)
Separator:       #545458 (separator with opacity)
Text Primary:    #FFFFFF
Text Secondary:  #8E8E93
Text Tertiary:   #636366
```

**Accent Colors**
```
Agent Blue:      #0A84FF  (active agents)
Agent Green:     #30D158  (active/healthy)
Agent Yellow:    #FFD60A  (spawning/waiting)
Agent Orange:    #FF9F0A  (idle)
Agent Red:       #FF453A  (error/danger)
Agent Purple:    #BF5AF2  (special threads)
```

**Agent Color Palette** (16 rotating colors for agent identity)
```swift
static let agentColors: [Color] = [
    Color(hex: "5AC8FA"),  // cyan
    Color(hex: "34C759"),  // green
    Color(hex: "FFD60A"),  // yellow
    Color(hex: "FF9F0A"),  // orange
    Color(hex: "FF453A"),  // red
    Color(hex: "FF2D55"),  // pink
    Color(hex: "BF5AF2"),  // purple
    Color(hex: "5856D6"),  // indigo
    Color(hex: "007AFF"),  // blue
    Color(hex: "64D2FF"),  // light blue
    Color(hex: "AC8E68"),  // brown
    Color(hex: "98989D"),  // gray
    Color(hex: "00C7BE"),  // teal
    Color(hex: "FF6961"),  // coral
    Color(hex: "77DD77"),  // pastel green
    Color(hex: "AEC6CF"),  // pastel blue
]
```

### Typography

```swift
// San Francisco (system font) throughout
struct FrayTypography {
    static let title = Font.system(.title, design: .default).weight(.semibold)
    static let headline = Font.system(.headline, design: .default)
    static let body = Font.system(.body, design: .default)
    static let callout = Font.system(.callout, design: .default)
    static let footnote = Font.system(.footnote, design: .default)
    static let caption = Font.system(.caption, design: .default)

    // Code blocks use SF Mono
    static let code = Font.system(.body, design: .monospaced)
    static let codeSmall = Font.system(.footnote, design: .monospaced)
}
```

### Spacing & Rhythm

```swift
struct FraySpacing {
    static let xs: CGFloat = 4
    static let sm: CGFloat = 8
    static let md: CGFloat = 16
    static let lg: CGFloat = 24
    static let xl: CGFloat = 32
    static let xxl: CGFloat = 48

    // Message-specific
    static let messagePadding: CGFloat = 12
    static let messageSpacing: CGFloat = 16
    static let avatarSize: CGFloat = 32
    static let sidebarWidth: CGFloat = 280
    static let threadPanelMinWidth: CGFloat = 240
    static let threadPanelMaxWidth: CGFloat = 400
}
```

---

## Window Layout

### NavigationSplitView with Liquid Glass Sidebar

macOS Tahoe's `NavigationSplitView` provides a floating Liquid Glass sidebar that refracts the content behind it. We use the three-column variant:

```swift
NavigationSplitView {
    // Sidebar: Channels + Threads (Liquid Glass floats above content)
    SidebarView()
} content: {
    // Content: Message list
    MessageListView()
        .backgroundExtensionEffect()  // Extends behind sidebar edges
} detail: {
    // Detail: Activity panel (optional)
    ActivityPanelView()
}
.navigationSplitViewColumnWidth(sidebar: 280, content: 400, detail: 200)
```

### Visual Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│  ← →  Fray                                               ⊖ ⊕ ⊗     │
│  (completely transparent menu bar - macOS Tahoe)                    │
├───────────────────────────────────────────────────────────────────────┤
│  ╭──────────────╮                                                   │
│  │              │                                                   │
│  │  [GLASS]     │            MESSAGE AREA                           │
│  │  CHANNELS    │  (content refracts through sidebar)               │
│  │              │                                         ACTIVITY  │
│  │  #main       │  ┌────────────────────────────────────┐   PANEL   │
│  │  ● active    │  │  @opus                         2m │           │
│  │  ○ idle      │  │                                   │  [GLASS]  │
│  │              │  │  Working on the refactor now.     │  ● opus   │
│  │  ───────     │  └────────────────────────────────────┘  active  │
│  │  THREADS     │                                        ○ design  │
│  │              │  ┌────────────────────────────────────┐  idle    │
│  │  ★ design    │  │  @designer                     1m │           │
│  │    notes     │  │                                   │  △ pm     │
│  │  ❯ meta      │  │  @opus sounds good!               │  spawning │
│  │    └ opus    │  └────────────────────────────────────┘           │
│  │              │                                                   │
│  │  ── other ── │  ╭────────────────────────────────────╮           │
│  │  planning    │  │ › Type a message...          [↑]  │  [GLASS]  │
│  │  research    │  ╰────────────────────────────────────╯  INPUT   │
│  │              │                                                   │
│  ╰──────────────╯                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

### Column Behavior

**Left Sidebar (Liquid Glass)**
- Floating glass panel with real-time light bending
- Content refracts behind it via `.backgroundExtensionEffect()`
- Collapsible with ⌘0
- Default width: 280pt (`.navigationSplitViewColumnWidth(min: 200, ideal: 280, max: 400)`)
- Channels section: fixed height
- Threads section: scrollable

**Center (Messages)**
- Background extends under sidebar edges for refraction effect
- Minimum width: 400pt
- Fixed input area at bottom with `.glassEffect()` styling

**Right Panel (Activity) - Optional**
- Toggle with ⌘I
- Glass styling for agent presence indicators
- Token usage visualization with glass progress bars

---

## Component Designs

### 1. Message Bubble

```
┌────────────────────────────────────────────────────────────────────┐
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │ @opus                                                    2m │ │
│  │                                                              │ │
│  │ I've finished the refactor. Here's what changed:            │ │
│  │                                                              │ │
│  │ • Extracted the message parsing into a separate module      │ │
│  │ • Added proper error handling for edge cases                │ │
│  │ • Updated the tests to cover the new behavior               │ │
│  │                                                              │ │
│  │ ┌──────────────────────────────────────────────────────────┐│ │
│  │ │ func parseMessage(body string) (*Message, error) {      ││ │
│  │ │     // Implementation here                               ││ │
│  │ │ }                                                        ││ │
│  │ └──────────────────────────────────────────────────────────┘│ │
│  │                                                              │ │
│  │ └─ 👍 --@adam   LGTM --@pm                                  │ │
│  │                                                              │ │
│  │ #msg-abc123                                       (edited) │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

**SwiftUI Implementation:**

```swift
struct MessageBubble: View {
    let message: FrayMessage
    @State private var isHovered = false
    @Environment(\.colorScheme) var colorScheme

    var agentColor: Color {
        FrayColors.colorForAgent(message.fromAgent)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: FraySpacing.sm) {
            // Header: Agent badge + timestamp
            HStack(alignment: .center, spacing: FraySpacing.sm) {
                AgentBadge(name: message.fromAgent, color: agentColor)

                Spacer()

                Text(message.relativeTimestamp)
                    .font(FrayTypography.caption)
                    .foregroundColor(.secondary)
            }

            // Reply context (if replying)
            if let replyTo = message.replyTo {
                ReplyContext(messageId: replyTo)
            }

            // Message body with markdown rendering
            MessageContent(body: message.body)
                .font(FrayTypography.body)
                .foregroundColor(.primary)

            // Reactions
            if !message.reactions.isEmpty {
                ReactionBar(reactions: message.reactions)
            }

            // Footer: Message ID + edit status
            HStack {
                Text("#\(message.shortId)")
                    .font(FrayTypography.codeSmall)
                    .foregroundColor(.tertiaryLabel)

                if message.isEdited {
                    Text("(edited)")
                        .font(FrayTypography.caption)
                        .foregroundColor(.tertiaryLabel)
                }

                Spacer()
            }
        }
        .padding(FraySpacing.messagePadding)
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(isHovered ? Color(.tertiarySystemFill) : Color.clear)
        )
        .onHover { hovering in
            withAnimation(.easeInOut(duration: 0.15)) {
                isHovered = hovering
            }
        }
    }
}

struct AgentBadge: View {
    let name: String
    let color: Color

    var body: some View {
        Text("@\(name)")
            .font(FrayTypography.headline)
            // Vibrant text auto-adapts to maintain legibility against backgrounds
            .padding(.horizontal, FraySpacing.sm)
            .padding(.vertical, FraySpacing.xs)
            // Liquid Glass badge with agent color tint
            .glassEffect(.regular.tint(color), in: .capsule)
    }
}
```

### 2. Thread List Item

```swift
struct ThreadListItem: View {
    let thread: FrayThread
    let isSelected: Bool
    let isCurrent: Bool
    let unreadCount: Int
    let isFaved: Bool

    var body: some View {
        HStack(spacing: FraySpacing.sm) {
            // Indicator column (fave star or avatar)
            ZStack {
                if isFaved {
                    Image(systemName: "star.fill")
                        .foregroundColor(.yellow)
                        .font(.system(size: 10))
                }
            }
            .frame(width: 16)

            // Thread name
            VStack(alignment: .leading, spacing: 2) {
                HStack {
                    Text(thread.displayName)
                        .font(FrayTypography.body)
                        .foregroundColor(isSelected ? .white : .primary)
                        .lineLimit(1)

                    if thread.hasChildren {
                        Image(systemName: "chevron.right")
                            .font(.system(size: 10))
                            .foregroundColor(.tertiaryLabel)
                    }
                }

                if let lastActivity = thread.lastActivityDescription {
                    Text(lastActivity)
                        .font(FrayTypography.caption)
                        .foregroundColor(isSelected ? .white.opacity(0.8) : .secondaryLabel)
                        .lineLimit(1)
                }
            }

            Spacer()

            // Unread badge with Liquid Glass
            if unreadCount > 0 {
                Text("\(unreadCount)")
                    .font(FrayTypography.caption)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .glassEffect(.regular.tint(.accentColor), in: .capsule)
            }
        }
        .padding(.horizontal, FraySpacing.sm)
        .padding(.vertical, FraySpacing.xs + 2)
        .background(
            RoundedRectangle(cornerRadius: 6)
                .fill(backgroundColor)
        )
    }

    var backgroundColor: Color {
        if isSelected {
            return Color.accentColor
        } else if isCurrent {
            return Color(.systemBlue).opacity(0.2)
        } else {
            return Color.clear
        }
    }
}
```

### 3. Agent Activity Row

Token usage visualized as a subtle progress bar in the row background:

```swift
struct AgentActivityRow: View {
    let agent: FrayAgent
    let tokenUsage: TokenUsage?

    var presenceIcon: String {
        switch agent.presence {
        case .active: return "▶"
        case .spawning: return "△"
        case .idle: return "▷"
        case .error: return "✕"
        case .offline: return "▽"
        }
    }

    var presenceColor: Color {
        switch agent.presence {
        case .active: return .green
        case .spawning: return .yellow
        case .idle: return .gray
        case .error: return .red
        case .offline: return .gray.opacity(0.5)
        }
    }

    var tokenPercent: Double {
        guard let usage = tokenUsage else { return 0 }
        return min(1.0, Double(usage.contextTokens) / 200_000.0)
    }

    var isInDanger: Bool {
        tokenPercent > 0.8
    }

    var body: some View {
        GeometryReader { geometry in
            ZStack(alignment: .leading) {
                // Token usage background bar
                if tokenPercent > 0 {
                    Rectangle()
                        .fill(isInDanger ? Color.red.opacity(0.3) : Color.gray.opacity(0.2))
                        .frame(width: geometry.size.width * tokenPercent)
                }

                // Content
                HStack(spacing: FraySpacing.sm) {
                    // Presence icon
                    Text(presenceIcon)
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(presenceColor)

                    // Agent name
                    Text("@\(agent.agentId)")
                        .font(FrayTypography.callout)
                        .foregroundColor(FrayColors.colorForAgent(agent.agentId))
                        .bold()

                    // Status
                    if let status = agent.status, !status.isEmpty {
                        Text(status)
                            .font(FrayTypography.caption)
                            .foregroundColor(.secondaryLabel)
                            .italic()
                            .lineLimit(1)
                    }

                    Spacer()
                }
                .padding(.horizontal, FraySpacing.sm)
                .padding(.vertical, FraySpacing.xs)
            }
        }
        .frame(height: 28)
    }
}
```

### 4. Input Area (Liquid Glass)

The input area floats at the bottom with Liquid Glass styling. Uses corner concentricity to maintain visual harmony with the window.

```swift
struct MessageInputArea: View {
    @Binding var text: String
    @Binding var replyTo: FrayMessage?
    @FocusState private var isFocused: Bool
    var onSubmit: (String) -> Void

    @State private var showingSuggestions = false
    @State private var suggestions: [Suggestion] = []
    @Namespace private var namespace

    var body: some View {
        VStack(spacing: 0) {
            // Reply preview with glass morphing
            if let reply = replyTo {
                HStack {
                    Image(systemName: "arrow.turn.up.left")
                        .foregroundColor(.secondary)

                    Text("Replying to @\(reply.fromAgent)")
                        .font(FrayTypography.footnote)

                    Spacer()

                    Button(action: { replyTo = nil }) {
                        Image(systemName: "xmark.circle.fill")
                    }
                    .buttonStyle(.glass)
                }
                .padding(.horizontal, FraySpacing.md)
                .padding(.vertical, FraySpacing.sm)
                .glassEffectID("reply", in: namespace)
                .transition(.glassEffect(namespace))
            }

            // Suggestions dropdown with glass backdrop
            if showingSuggestions && !suggestions.isEmpty {
                SuggestionsPopup(suggestions: suggestions) { suggestion in
                    applySuggestion(suggestion)
                }
                .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 12))
            }

            // Input field with glass container
            // Uses FrayTextEditor for cursor-aware newline insertion and focus management
            HStack(alignment: .bottom, spacing: FraySpacing.sm) {
                FrayTextEditor(text: $text, isFocused: $isFocused, onSubmit: submitMessage)
                    .frame(minHeight: 36, maxHeight: 120)
                    .onChange(of: text) { _, newValue in
                        updateSuggestions(for: newValue)
                    }

                // Send button with glass styling
                Button(action: submitMessage) {
                    Image(systemName: "arrow.up.circle.fill")
                        .font(.system(size: 28))
                }
                .buttonStyle(.glassProminent)  // macOS Tahoe primary glass button
                .disabled(text.isEmpty)
            }
            .padding(FraySpacing.md)
            // Concentric corners align with window edges
            .glassEffect(
                .regular,
                in: .rect(cornerRadius: .containerConcentric)
            )
        }
    }
}

/// NSTextView wrapper for cursor-aware text editing
/// Handles Opt+Enter for newline insertion at cursor, Enter for submit
struct FrayTextEditor: NSViewRepresentable {
    @Binding var text: String
    var isFocused: FocusState<Bool>.Binding
    var onSubmit: () -> Void

    func makeNSView(context: Context) -> NSScrollView {
        let scrollView = NSTextView.scrollableTextView()
        let textView = scrollView.documentView as! NSTextView
        textView.delegate = context.coordinator
        textView.font = NSFont.monospacedSystemFont(ofSize: 14, weight: .regular)
        textView.isRichText = false
        textView.allowsUndo = true
        textView.drawsBackground = false
        textView.textContainerInset = NSSize(width: 8, height: 8)
        scrollView.hasVerticalScroller = true
        scrollView.hasHorizontalScroller = false
        scrollView.drawsBackground = false
        context.coordinator.textView = textView
        return scrollView
    }

    func updateNSView(_ nsView: NSScrollView, context: Context) {
        let textView = nsView.documentView as! NSTextView
        if textView.string != text {
            let selectedRange = textView.selectedRange()
            textView.string = text
            // Restore selection if valid
            if selectedRange.location <= text.count {
                textView.setSelectedRange(selectedRange)
            }
        }
        // Sync focus state from SwiftUI to AppKit
        if isFocused.wrappedValue && textView.window?.firstResponder != textView {
            textView.window?.makeFirstResponder(textView)
        }
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(self)
    }

    class Coordinator: NSObject, NSTextViewDelegate {
        var parent: FrayTextEditor
        weak var textView: NSTextView?

        init(_ parent: FrayTextEditor) {
            self.parent = parent
        }

        func textDidChange(_ notification: Notification) {
            guard let textView = notification.object as? NSTextView else { return }
            parent.text = textView.string
        }

        func textDidBeginEditing(_ notification: Notification) {
            parent.isFocused.wrappedValue = true
        }

        func textDidEndEditing(_ notification: Notification) {
            parent.isFocused.wrappedValue = false
        }

        func textView(_ textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
            if commandSelector == #selector(NSResponder.insertNewline(_:)) {
                // Use currentEvent for reliable modifier detection
                if let event = NSApp.currentEvent, event.modifierFlags.contains(.option) {
                    // Opt+Enter: insert newline at cursor
                    textView.insertNewlineIgnoringFieldEditor(nil)
                    return true
                } else {
                    // Plain Enter: submit
                    parent.onSubmit()
                    return true
                }
            }
            return false
        }
    }
}
```

---

## Interactions & Animations

### Message Hover

```swift
.onHover { hovering in
    withAnimation(.easeInOut(duration: 0.15)) {
        isHovered = hovering
    }
}
```

### Thread Selection

```swift
.animation(.spring(response: 0.3, dampingFraction: 0.7), value: isSelected)
```

### New Message Appearance

```swift
// Messages appear with a subtle slide-up + fade
.transition(.asymmetric(
    insertion: .move(edge: .bottom).combined(with: .opacity),
    removal: .opacity
))
```

### Agent Presence Animation

```swift
// Spawning state: gentle pulse
@State private var spawningPulse = false

if agent.presence == .spawning {
    Circle()
        .fill(Color.yellow)
        .scaleEffect(spawningPulse ? 1.2 : 1.0)
        .opacity(spawningPulse ? 0.5 : 1.0)
        .onAppear {
            withAnimation(.easeInOut(duration: 0.75).repeatForever(autoreverses: true)) {
                spawningPulse = true
            }
        }
}
```

### Panel Resize

```swift
// Smooth panel width changes
.frame(width: sidebarWidth)
.animation(.spring(response: 0.3, dampingFraction: 0.8), value: sidebarWidth)
```

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| ⌘N | New message (focus input) |
| ⌘K | Command palette |
| ⌘0 | Toggle sidebar |
| ⌘I | Toggle activity panel |
| ⌘↑ | Previous thread |
| ⌘↓ | Next thread |
| ⌘1-9 | Jump to thread 1-9 |
| ⌘R | Reply to selected message |
| ⌘E | Edit last message |
| ⌘⇧E | Edit selected message |
| ⌘⇧C | Copy message |
| ⌥Enter | Insert newline |
| Escape | Clear input / Close panel |
| Space | (in thread list) Filter |
| j/k | Navigate threads |
| h/l | Drill in/out of threads |

---

## Menu Bar App (Bonus)

A lightweight menu bar presence showing:
- Unread mention count badge
- Active agent count
- Quick compose popover

```
┌─────────────────────────────────────────┐
│  📩 3                                   │  ← Menu bar icon with badge
├─────────────────────────────────────────┤
│                                         │
│  UNREAD MENTIONS                        │
│  @adam in design-thread            2m   │
│  @opus in #main                    5m   │
│                                         │
│  ─────────────────────────────────────  │
│                                         │
│  ACTIVE AGENTS                          │
│  ● opus (refactoring auth)              │
│  △ pm (spawning)                        │
│                                         │
│  ─────────────────────────────────────  │
│                                         │
│  [Quick Compose...]        [Open Fray]  │
│                                         │
└─────────────────────────────────────────┘
```

---

## File Structure

```
Fray/
├── FrayApp.swift                 # App entry point
├── FrayBridge/
│   ├── FrayBridge.swift          # Swift FFI bindings
│   ├── libfray.h                 # C header from cgo
│   └── module.modulemap          # Swift/C bridging
├── Models/
│   ├── FrayMessage.swift
│   ├── FrayThread.swift
│   ├── FrayAgent.swift
│   └── FrayChannel.swift
├── ViewModels/
│   ├── MessagesViewModel.swift
│   ├── ThreadsViewModel.swift
│   ├── AgentsViewModel.swift
│   └── ChannelsViewModel.swift
├── Views/
│   ├── ContentView.swift         # Main 3-column layout
│   ├── Sidebar/
│   │   ├── ChannelList.swift
│   │   ├── ThreadList.swift
│   │   └── ThreadListItem.swift
│   ├── Messages/
│   │   ├── MessageList.swift
│   │   ├── MessageBubble.swift
│   │   ├── MessageContent.swift
│   │   └── ReactionBar.swift
│   ├── Input/
│   │   ├── MessageInputArea.swift
│   │   └── SuggestionsPopup.swift
│   └── Activity/
│       ├── ActivityPanel.swift
│       └── AgentActivityRow.swift
├── Styles/
│   ├── FrayColors.swift
│   ├── FrayTypography.swift
│   └── FraySpacing.swift
├── Services/
│   ├── MessageService.swift
│   ├── ThreadService.swift
│   ├── AgentService.swift
│   └── NotificationService.swift
└── Resources/
    ├── Assets.xcassets
    └── Info.plist
```

---

## Implementation Phases

### Phase 1: Core Foundation
- [ ] Go backend compiled as dylib with cgo exports
- [ ] Swift FFI bridge layer (FrayBridge)
- [ ] Basic window with 3-column layout
- [ ] Channel list display
- [ ] Thread list display (flat)
- [ ] Message list display
- [ ] Message input with basic submit

### Phase 2: Rich Features
- [ ] Thread hierarchy with drill-in/out
- [ ] @mention autocomplete
- [ ] Reply-to UI with preview
- [ ] Reactions display and adding
- [ ] Thread subscriptions and favorites
- [ ] Unread counts and badges
- [ ] Code syntax highlighting

### Phase 3: Agent Activity
- [ ] Activity panel with presence
- [ ] Token usage visualization
- [ ] Job cluster display
- [ ] Spawning/idle animations
- [ ] Agent navigation (click to thread)

### Phase 4: Polish
- [ ] Menu bar app
- [ ] Keyboard shortcuts
- [ ] Command palette (⌘K)
- [ ] Notifications integration
- [ ] Window state persistence
- [ ] Light mode support
- [ ] Accessibility

---

## Technical Notes

### cgo Integration

The Go backend needs to export C-compatible functions:

```go
// internal/ffi/exports.go
package ffi

import "C"

//export FrayGetMessages
func FrayGetMessages(projectPath *C.char, limit C.int, sinceTS C.int64_t) *C.char {
    // Returns JSON array of messages
}

//export FrayPostMessage
func FrayPostMessage(projectPath *C.char, agentID *C.char, body *C.char, replyTo *C.char) *C.char {
    // Returns JSON of created message
}

//export FrayGetThreads
func FrayGetThreads(projectPath *C.char) *C.char {
    // Returns JSON array of threads
}

// ... more exports
```

Build command:
```bash
go build -buildmode=c-shared -o libfray.dylib ./internal/ffi
```

### Swift FFI

```swift
// FrayBridge.swift
import Foundation

class FrayBridge {
    static func getMessages(projectPath: String, limit: Int, since: Int64?) -> [FrayMessage] {
        let cPath = projectPath.cString(using: .utf8)
        let cResult = FrayGetMessages(cPath, Int32(limit), since ?? 0)
        defer { free(cResult) }

        guard let result = cResult else { return [] }
        let json = String(cString: result)
        return try? JSONDecoder().decode([FrayMessage].self, from: json.data(using: .utf8)!)
    }
}
```

---

## Design Principles

1. **Liquid Glass for Navigation Only**: Never apply glass to content (lists, tables, media). Only for toolbars, sidebars, controls, and floating elements.
2. **Concentric Corners**: Use `.containerConcentric` to align nested elements with window edges.
3. **Morphing Transitions**: Use `GlassEffectContainer` + `.glassEffectID()` for smooth state changes.
4. **Vibrant Text**: Let SwiftUI automatically adapt text colors for legibility against glass.
5. **Agent-Centric**: Agents are first-class citizens with tinted glass badges.
6. **Keyboard-Friendly**: Full TUI-style navigation (j/k, h/l, ⌘-shortcuts).
7. **Real-time**: Instant updates via direct cgo backend integration.

---

## Accessibility

Liquid Glass automatically adapts for:
- **Reduced Transparency**: Falls back to solid backgrounds
- **Increased Contrast**: Higher text contrast
- **Reduced Motion**: Disables morphing animations
- **User Tint Control**: Respects system tint preferences (iOS 26.1+)

---

## Resources

- [WWDC25: Build a SwiftUI app with the new design](https://developer.apple.com/videos/play/wwdc2025/323/)
- [WWDC25: Build an AppKit app with the new design](https://developer.apple.com/videos/play/wwdc2025/310/)
- [Adopting Liquid Glass](https://developer.apple.com/documentation/TechnologyOverviews/adopting-liquid-glass)
- [Applying Liquid Glass to Custom Views](https://developer.apple.com/documentation/SwiftUI/Applying-Liquid-Glass-to-custom-views)
- [LiquidGlassReference (Community)](https://github.com/conorluddy/LiquidGlassReference)
