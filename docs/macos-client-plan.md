# Fray macOS Native Client: Implementation Plan

A comprehensive plan for building a native macOS client for fray, following the Liquid Glass design spec.

---

## Executive Summary

Build a native SwiftUI macOS client backed by the existing fray Go backend via cgo FFI. The client will provide real-time agent messaging, thread navigation, and presence visualization using Apple's Liquid Glass design system for macOS Tahoe 26+.

**Key Technical Decisions:**
- **FFI approach**: Go dylib via cgo with JSON serialization
- **State management**: SwiftUI's `@Observable` macro (Swift 5.9+)
- **Polling vs. Push**: File-based polling with debouncing (matches CLI model)
- **Target**: macOS Tahoe 26+ (Xcode 17+, Swift 6)

---

## Phase 1: Foundation Layer

### 1.1 Go FFI Library (`libfray.dylib`)

Create a new package `internal/ffi/` that exposes C-compatible functions.

**Directory structure:**
```
internal/ffi/
├── exports.go      # //export functions
├── handles.go      # Handle management (opaque pointers)
├── json.go         # JSON serialization helpers
└── errors.go       # Error encoding
```

**Core exports needed:**

```go
// Project management
//export FrayDiscoverProject
func FrayDiscoverProject(startDir *C.char) *C.char  // → {root, dbPath, error}

//export FrayOpenDatabase
func FrayOpenDatabase(projectPath *C.char) C.uint64  // → handle ID

//export FrayCloseDatabase
func FrayCloseDatabase(handle C.uint64)

// Messages
//export FrayGetMessages
func FrayGetMessages(handle C.uint64, home *C.char, limit C.int, sinceCursor *C.char) *C.char
// home: NULL → main room ("room"), empty string → all homes (no filter), non-empty → specific thread/home
// sinceCursor format: "guid:ts" or NULL for no cursor
// Returns: {ok, data: {messages: [], cursor: {guid, ts}}, error}

//export FrayPostMessage
func FrayPostMessage(handle C.uint64, body, fromAgent, home, replyTo *C.char) *C.char

//export FrayEditMessage
func FrayEditMessage(handle C.uint64, msgID, newBody, reason *C.char) *C.char

//export FrayAddReaction
func FrayAddReaction(handle C.uint64, msgID, emoji, agent *C.char) *C.char

// Agents
//export FrayGetAgents
func FrayGetAgents(handle C.uint64, managedOnly C.int) *C.char

//export FrayGetAgent
func FrayGetAgent(handle C.uint64, agentID *C.char) *C.char

// Threads
//export FrayGetThreads
func FrayGetThreads(handle C.uint64, parentThread *C.char, includeArchived C.int) *C.char

//export FrayGetThread
func FrayGetThread(handle C.uint64, threadRef *C.char) *C.char

//export FrayGetThreadMessages
func FrayGetThreadMessages(handle C.uint64, threadGUID *C.char, limit C.int, sinceCursor *C.char) *C.char
// sinceCursor format: "guid:ts" or NULL for no cursor
// Returns: {ok, data: {messages: [], cursor: {guid, ts}}, error}
// NOTE: Current db.GetThreadMessages lacks limit/cursor support. FFI implementation should:
// 1. Fetch all messages via GetThreadMessages
// 2. Apply cursor filtering in memory (skip messages before cursor)
// 3. Apply limit in memory
// Future: Add GetThreadMessagesPaginated to internal/db/queries_threads.go

// Subscriptions & Faves
//export FraySubscribeToThread
func FraySubscribeToThread(handle C.uint64, threadGUID, agentID *C.char) *C.char

//export FrayFaveItem
func FrayFaveItem(handle C.uint64, itemGUID, agentID *C.char) *C.char

// Read tracking
//export FrayGetReadTo
func FrayGetReadTo(handle C.uint64, agentID, home *C.char) *C.char

//export FraySetReadTo
func FraySetReadTo(handle C.uint64, agentID, home, msgID *C.char) *C.char

// Free allocated strings
//export FrayFreeString
func FrayFreeString(ptr *C.char)
```

**Handle management pattern:**
```go
// handles.go
var (
    handleMu    sync.RWMutex
    handles     = make(map[uint64]*db.Database)
    nextHandle  uint64 = 1
)

func registerHandle(db *db.Database) uint64 {
    handleMu.Lock()
    defer handleMu.Unlock()
    id := nextHandle
    nextHandle++
    handles[id] = db
    return id
}

func getHandle(id uint64) (*db.Database, bool) {
    handleMu.RLock()
    defer handleMu.RUnlock()
    db, ok := handles[id]
    return db, ok
}
```

**JSON response format:**
```json
{
  "ok": true,
  "data": { ... },
  "error": null
}
// or
{
  "ok": false,
  "data": null,
  "error": "message not found"
}
```

**Build command:**
```bash
cd /Users/adam/dev/fray
CGO_ENABLED=1 go build -buildmode=c-shared -o build/libfray.dylib ./internal/ffi
```

**Deliverables:**
- [ ] `internal/ffi/exports.go` with all exports
- [ ] `internal/ffi/handles.go` for handle management
- [ ] `internal/ffi/json.go` for response encoding
- [ ] Generated `libfray.h` from cgo
- [ ] Build script in `Makefile` or `scripts/`

---

### 1.2 Swift FFI Bridge (`FrayBridge/`)

Create Swift layer that wraps the C FFI.

**Files:**
```
Fray/FrayBridge/
├── module.modulemap    # Swift-C bridging
├── FrayBridge.swift    # Main bridge class
├── FrayTypes.swift     # Codable Swift types
└── FrayError.swift     # Error handling
```

**module.modulemap:**
```
module libfray {
    header "libfray.h"
    link "fray"
    export *
}
```

**FrayBridge.swift:**
```swift
import Foundation

@Observable
final class FrayBridge {
    private var handle: UInt64 = 0
    private(set) var projectPath: String?
    private(set) var isConnected: Bool = false

    func connect(projectPath: String) throws {
        guard handle == 0 else { return }

        let cPath = projectPath.withCString { strdup($0) }
        defer { free(cPath) }

        handle = FrayOpenDatabase(cPath)
        guard handle != 0 else {
            throw FrayError.connectionFailed
        }

        self.projectPath = projectPath
        isConnected = true
    }

    func disconnect() {
        guard handle != 0 else { return }
        FrayCloseDatabase(handle)
        handle = 0
        isConnected = false
    }

    // Safe string-to-C bridging: use withCString to ensure pointer validity
    // throughout the FFI call. Never use .cString or strdup without proper lifetime management.

    /// Fetch messages from room or thread
    /// Returns messages + cursor for pagination
    func getMessages(home: String?, limit: Int = 100, since: MessageCursor? = nil) throws -> MessagePage {
        let result = withOptionalCString(home) { cHome in
            withOptionalCString(since.map { "\($0.guid):\($0.ts)" }) { cCursor in
                callFFI { FrayGetMessages(handle, cHome, Int32(limit), cCursor) }
            }
        }
        return try decode(MessagePage.self, from: result)
    }

    /// Fetch messages from a specific thread (playlist members)
    /// Returns messages + cursor for pagination
    func getThreadMessages(threadGuid: String, limit: Int = 100, since: MessageCursor? = nil) throws -> MessagePage {
        let result = threadGuid.withCString { cThreadGuid in
            withOptionalCString(since.map { "\($0.guid):\($0.ts)" }) { cCursor in
                callFFI { FrayGetThreadMessages(handle, cThreadGuid, Int32(limit), cCursor) }
            }
        }
        return try decode(MessagePage.self, from: result)
    }

    func postMessage(body: String, from agent: String, in home: String?, replyTo: String?) throws -> FrayMessage {
        let result = body.withCString { cBody in
            agent.withCString { cAgent in
                withOptionalCString(home) { cHome in
                    withOptionalCString(replyTo) { cReplyTo in
                        callFFI { FrayPostMessage(handle, cBody, cAgent, cHome, cReplyTo) }
                    }
                }
            }
        }
        return try decode(FrayMessage.self, from: result)
    }

    // ... more methods follow same pattern

    /// Safely call FFI with optional string, passing NULL if nil
    private func withOptionalCString<R>(_ string: String?, _ body: (UnsafePointer<CChar>?) -> R) -> R {
        if let s = string {
            return s.withCString { body($0) }
        } else {
            return body(nil)
        }
    }

    private func callFFI(_ call: () -> UnsafeMutablePointer<CChar>?) -> String {
        guard let ptr = call() else { return "{\"ok\":false,\"error\":\"null response\"}" }
        defer { FrayFreeString(ptr) }
        return String(cString: ptr)
    }

    private func decode<T: Decodable>(_ type: T.Type, from json: String) throws -> T {
        let response = try JSONDecoder.fray.decode(FFIResponse<T>.self, from: json.data(using: .utf8)!)
        guard response.ok, let data = response.data else {
            throw FrayError.ffi(response.error ?? "unknown error")
        }
        return data
    }
}

struct FFIResponse<T: Decodable>: Decodable {
    let ok: Bool
    let data: T?
    let error: String?
}
```

**FrayTypes.swift:**

Use `keyDecodingStrategy = .convertFromSnakeCase` for cleaner Swift names while matching Go's `json:"snake_case"` tags.

```swift
// Configure decoder globally
extension JSONDecoder {
    static let fray: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }()
}

// Message type - matches internal/types/types.go:86
struct FrayMessage: Codable, Identifiable, Equatable {
    let id: String              // json:"id" (Go uses "id" not "guid")
    let ts: Int64
    let channelId: String?      // json:"channel_id"
    let home: String?           // json:"home"
    let fromAgent: String       // json:"from_agent"
    let sessionId: String?      // json:"session_id"
    let body: String
    let mentions: [String]
    let forkSessions: [String: String]?  // json:"fork_sessions"
    let reactions: [String: [ReactionEntry]]
    let type: MessageType
    let references: String?     // json:"references"
    let surfaceMessage: String? // json:"surface_message"
    let replyTo: String?        // json:"reply_to"
    let quoteMessageGuid: String?  // json:"quote_message_guid"
    let editedAt: Int64?        // json:"edited_at"
    let edited: Bool?           // json:"edited"
    let editCount: Int?         // json:"edit_count"
    let archivedAt: Int64?      // json:"archived_at"

    var shortId: String { String(id.dropFirst(4).prefix(8)) }

    enum MessageType: String, Codable {
        case agent, user, event, surface, tombstone
    }
}

// Reaction entry - matches internal/types/types.go:73
struct ReactionEntry: Codable, Equatable {
    let agentId: String    // json:"agent_id"
    let reactedAt: Int64   // json:"reacted_at"
}

// Agent - matches internal/types/types.go:50
struct FrayAgent: Codable, Identifiable, Equatable {
    let guid: String
    let agentId: String         // json:"agent_id"
    let status: String?
    let purpose: String?
    let avatar: String?
    let registeredAt: Int64     // json:"registered_at"
    let lastSeen: Int64         // json:"last_seen"
    let leftAt: Int64?          // json:"left_at"
    let managed: Bool?
    let invoke: InvokeConfig?
    let presence: AgentPresence?
    let mentionWatermark: String?   // json:"mention_watermark"
    let reactionWatermark: Int64?   // json:"reaction_watermark"
    let lastHeartbeat: Int64?       // json:"last_heartbeat"
    let lastSessionId: String?      // json:"last_session_id"
    let sessionMode: String?        // json:"session_mode"
    let jobId: String?              // json:"job_id"
    let jobIdx: Int?                // json:"job_idx"
    let isEphemeral: Bool?          // json:"is_ephemeral"

    var id: String { guid }

    enum AgentPresence: String, Codable {
        case active, spawning, prompting, prompted, idle, error, offline, brb
    }
}

// Invoke config - matches internal/types/types.go:37
struct InvokeConfig: Codable, Equatable {
    let driver: String?
    let model: String?
    let trust: [String]?
    let config: [String: JSONValue]?  // Uses JSONValue for nested structures
    let promptDelivery: String?       // json:"prompt_delivery"
    let spawnTimeoutMs: Int64?        // json:"spawn_timeout_ms"
    let idleAfterMs: Int64?           // json:"idle_after_ms"
    let minCheckinMs: Int64?          // json:"min_checkin_ms"
    let maxRuntimeMs: Int64?          // json:"max_runtime_ms"
}

// Thread - matches internal/types/types.go:291
struct FrayThread: Codable, Identifiable, Equatable {
    let guid: String
    let name: String
    let parentThread: String?       // json:"parent_thread"
    let status: ThreadStatus
    let type: ThreadType?
    let createdAt: Int64            // json:"created_at"
    let createdBy: String?          // json:"created_by"
    let ownerAgent: String?         // json:"owner_agent"
    let anchorMessageGuid: String?  // json:"anchor_message_guid"
    let anchorHidden: Bool?         // json:"anchor_hidden"
    let lastActivityAt: Int64?      // json:"last_activity_at"

    var id: String { guid }

    enum ThreadStatus: String, Codable {
        case open, archived
    }

    enum ThreadType: String, Codable {
        case standard, knowledge, system
    }
}

// Message cursor - matches internal/types/types.go:217
struct MessageCursor: Codable, Equatable {
    let guid: String
    let ts: Int64
}

/// Paginated message response from FFI
struct MessagePage: Codable {
    let messages: [FrayMessage]
    let cursor: MessageCursor?  // nil when no more messages
}

/// Recursive JSON value wrapper for arbitrary config structures
/// Supports scalars, arrays, and nested objects
enum JSONValue: Codable, Equatable {
    case null
    case bool(Bool)
    case int(Int)
    case double(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()

        if container.decodeNil() {
            self = .null
        } else if let bool = try? container.decode(Bool.self) {
            self = .bool(bool)
        } else if let int = try? container.decode(Int.self) {
            self = .int(int)
        } else if let double = try? container.decode(Double.self) {
            self = .double(double)
        } else if let string = try? container.decode(String.self) {
            self = .string(string)
        } else if let array = try? container.decode([JSONValue].self) {
            self = .array(array)
        } else if let object = try? container.decode([String: JSONValue].self) {
            self = .object(object)
        } else {
            throw DecodingError.typeMismatch(JSONValue.self,
                DecodingError.Context(codingPath: decoder.codingPath,
                    debugDescription: "Unable to decode JSON value"))
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch self {
        case .null: try container.encodeNil()
        case .bool(let v): try container.encode(v)
        case .int(let v): try container.encode(v)
        case .double(let v): try container.encode(v)
        case .string(let v): try container.encode(v)
        case .array(let v): try container.encode(v)
        case .object(let v): try container.encode(v)
        }
    }
}
```

**Deliverables:**
- [ ] Swift package or Xcode project target for FrayBridge
- [ ] Type-safe wrappers for all FFI functions
- [ ] Codable Swift types matching Go types
- [ ] Unit tests for FFI bridge (mock responses)

---

### 1.3 Xcode Project Setup

**Project structure:**
```
FrayApp/
├── FrayApp.xcodeproj
├── Fray/
│   ├── FrayApp.swift           # App entry point
│   ├── ContentView.swift       # Main 3-column layout
│   ├── FrayBridge/            # FFI layer
│   ├── Models/                # ViewModels
│   ├── Views/                 # SwiftUI views
│   ├── Styles/                # Design system
│   └── Services/              # Polling, notifications
├── FrayTests/
├── Resources/
│   └── Assets.xcassets
└── build/
    └── libfray.dylib          # Linked Go library
```

**Xcode configuration:**
- Deployment target: macOS 26.0
- Swift version: 6.0
- Library search paths: `$(PROJECT_DIR)/build`
- Linked frameworks: `libfray.dylib`
- Code signing: Development signing for testing

**Build phases:**
1. Run Script: Build `libfray.dylib` if modified
2. Copy Files: Copy dylib to app bundle `Frameworks/`

**Deliverables:**
- [ ] Xcode project with correct structure
- [ ] Build phase for dylib compilation
- [ ] App signing configuration
- [ ] Info.plist with required entitlements

---

## Phase 2: Core UI Components

### 2.1 Design System (`Styles/`)

**FrayColors.swift:**
```swift
import SwiftUI

enum FrayColors {
    static let agentColors: [Color] = [
        Color(hex: "5AC8FA"), Color(hex: "34C759"), Color(hex: "FFD60A"),
        Color(hex: "FF9F0A"), Color(hex: "FF453A"), Color(hex: "FF2D55"),
        Color(hex: "BF5AF2"), Color(hex: "5856D6"), Color(hex: "007AFF"),
        Color(hex: "64D2FF"), Color(hex: "AC8E68"), Color(hex: "98989D"),
        Color(hex: "00C7BE"), Color(hex: "FF6961"), Color(hex: "77DD77"),
        Color(hex: "AEC6CF")
    ]

    static func colorForAgent(_ agentId: String) -> Color {
        let hash = agentId.utf8.reduce(0) { $0 &+ Int($1) }
        return agentColors[abs(hash) % agentColors.count]
    }

    static let presence: [FrayAgent.AgentPresence: Color] = [
        .active: .green,
        .spawning: .yellow,
        .prompting: .orange,
        .prompted: .orange,
        .idle: .gray,
        .error: .red,
        .offline: .gray.opacity(0.5),
        .brb: .purple
    ]
}

extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let r = Double((int >> 16) & 0xFF) / 255
        let g = Double((int >> 8) & 0xFF) / 255
        let b = Double(int & 0xFF) / 255
        self.init(red: r, green: g, blue: b)
    }
}
```

**FraySpacing.swift:**
```swift
enum FraySpacing {
    static let xs: CGFloat = 4
    static let sm: CGFloat = 8
    static let md: CGFloat = 16
    static let lg: CGFloat = 24
    static let xl: CGFloat = 32
    static let xxl: CGFloat = 48

    static let messagePadding: CGFloat = 12
    static let messageSpacing: CGFloat = 16
    static let avatarSize: CGFloat = 32
    static let sidebarWidth: CGFloat = 280
}
```

**Deliverables:**
- [ ] `FrayColors.swift` with full palette
- [ ] `FraySpacing.swift` with spacing constants
- [ ] `FrayTypography.swift` with font definitions
- [ ] Unit tests for color hash consistency

---

### 2.2 Main Layout (`ContentView.swift`)

Three-column NavigationSplitView with Liquid Glass sidebar:

```swift
import SwiftUI

struct ContentView: View {
    @Environment(FrayBridge.self) private var bridge
    @State private var selectedThread: FrayThread?
    @State private var columnVisibility: NavigationSplitViewVisibility = .all

    var body: some View {
        NavigationSplitView(columnVisibility: $columnVisibility) {
            SidebarView(selectedThread: $selectedThread)
        } content: {
            if let thread = selectedThread {
                MessageListView(thread: thread)
            } else {
                RoomView()
            }
        } detail: {
            ActivityPanelView()
        }
        .navigationSplitViewColumnWidth(
            sidebar: FraySpacing.sidebarWidth,
            content: 400,
            detail: 200
        )
    }
}
```

**Deliverables:**
- [ ] `ContentView.swift` with NavigationSplitView
- [ ] Column width management
- [ ] Keyboard shortcuts (⌘0 sidebar, ⌘I activity)

---

### 2.3 Sidebar Components

**SidebarView.swift:**
```swift
struct SidebarView: View {
    @Binding var selectedThread: FrayThread?
    @Environment(ThreadsViewModel.self) private var threadsVM

    var body: some View {
        List(selection: $selectedThread) {
            Section("Channels") {
                ChannelListSection()
            }

            Section("Threads") {
                ThreadListSection()
            }
        }
        .listStyle(.sidebar)
        .navigationTitle("Fray")
    }
}
```

**ThreadListItem.swift** - per design spec with fave stars, unread badges, drill-in chevrons.

**Deliverables:**
- [ ] `SidebarView.swift` with sections
- [ ] `ChannelListSection.swift`
- [ ] `ThreadListSection.swift` with hierarchy
- [ ] `ThreadListItem.swift` with Liquid Glass badges
- [ ] Thread filtering (faved, following, muted)

---

### 2.4 Message Components

**MessageListView.swift:**

Thread view calls `FrayGetThreadMessages(thread.guid)` to fetch thread-specific messages.
Room view (nil thread) calls `FrayGetMessages(home: nil)` for main room.

```swift
struct MessageListView: View {
    let thread: FrayThread?
    @Environment(MessagesViewModel.self) private var messagesVM
    @Environment(AppState.self) private var appState
    @State private var scrollPosition: String?
    @State private var inputText: String = ""
    @State private var replyTo: FrayMessage?

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: FraySpacing.messageSpacing) {
                    ForEach(messagesVM.messages) { message in
                        MessageBubble(message: message)
                            .id(message.id)
                    }
                }
                .padding()
            }
            .scrollPosition(id: $scrollPosition, anchor: .bottom)
            .onChange(of: messagesVM.messages.count) {
                if let last = messagesVM.messages.last {
                    withAnimation {
                        proxy.scrollTo(last.id, anchor: .bottom)
                    }
                }
            }
        }
        .safeAreaInset(edge: .bottom) {
            // MessageInputArea per design spec: bindings + onSubmit callback
            MessageInputArea(text: $inputText, replyTo: $replyTo) { body in
                Task {
                    guard let agentId = appState.currentAgentId else { return }
                    try await messagesVM.postMessage(
                        body: body,
                        from: agentId,
                        replyTo: replyTo?.id
                    )
                    inputText = ""
                    replyTo = nil
                }
            }
            .disabled(appState.currentAgentId == nil)
        }
        .task(id: thread?.guid) {
            // Load messages when thread changes
            await messagesVM.loadMessages(thread: thread)
        }
        .onAppear {
            messagesVM.startPolling(thread: thread)
        }
        .onDisappear {
            messagesVM.stopPolling()
        }
    }
}
```

**MessageBubble.swift** - per design spec with:
- Agent badge (Liquid Glass with tint)
- Message body with markdown rendering
- Code syntax highlighting
- Reaction bar
- Hover states
- Edit indicator

**MessageContent.swift** - Markdown parser with:
- Code blocks (syntax highlighted)
- @mentions (clickable)
- #thread references (clickable)
- Links

**ReactionBar.swift:**
```swift
struct ReactionBar: View {
    let reactions: [String: [ReactionEntry]]
    let onReact: (String) -> Void

    var body: some View {
        HStack(spacing: FraySpacing.xs) {
            ForEach(Array(reactions.keys.sorted()), id: \.self) { emoji in
                ReactionPill(emoji: emoji, entries: reactions[emoji] ?? [])
            }

            // Add reaction button
            Button(action: { /* show picker */ }) {
                Image(systemName: "face.smiling")
                    .font(.caption)
            }
            .buttonStyle(.borderless)
            .opacity(0.5)
        }
    }
}
```

**Deliverables:**
- [ ] `MessageListView.swift` with virtual scrolling
- [ ] `MessageBubble.swift` per design spec
- [ ] `MessageContent.swift` with markdown
- [ ] `ReactionBar.swift` + `ReactionPill.swift`
- [ ] `ReplyContext.swift` for reply previews
- [ ] Code syntax highlighting (Splash or similar)

---

### 2.5 Input Area

**MessageInputArea.swift** - per design spec:
- Liquid Glass container
- Reply preview with morphing transitions
- @mention autocomplete
- Multi-line input
- Send button

**SuggestionsPopup.swift:**
```swift
struct SuggestionsPopup: View {
    let suggestions: [Suggestion]
    let onSelect: (Suggestion) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(suggestions) { suggestion in
                SuggestionRow(suggestion: suggestion)
                    .onTapGesture { onSelect(suggestion) }
            }
        }
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 12))
    }
}

enum Suggestion: Identifiable {
    case agent(FrayAgent)
    case thread(FrayThread)
    case command(String, String) // name, description

    var id: String { /* ... */ }
}
```

**Deliverables:**
- [ ] `MessageInputArea.swift` with Liquid Glass
- [ ] `SuggestionsPopup.swift` for autocomplete
- [ ] @mention extraction and validation
- [ ] Keyboard handling (Enter to send, Opt+Enter for newline)

---

## Phase 3: ViewModels & State

### 3.1 MessagesViewModel

Handles both room messages and thread messages with cursor-based pagination.

```swift
@Observable
final class MessagesViewModel {
    private let bridge: FrayBridge
    private var currentThread: FrayThread?
    private var pollTimer: Timer?
    private var cursor: MessageCursor?  // Cursor for pagination (guid + ts)

    private(set) var messages: [FrayMessage] = []
    private(set) var isLoading: Bool = false
    private(set) var error: String?

    /// Load messages for a thread (or room if nil)
    func loadMessages(thread: FrayThread?, limit: Int = 100) async {
        isLoading = true
        defer { isLoading = false }

        do {
            currentThread = thread
            cursor = nil  // Reset cursor on context change

            if let thread = thread {
                // Thread-specific messages via FrayGetThreadMessages
                let result = try bridge.getThreadMessages(threadGuid: thread.guid, limit: limit)
                messages = result.messages
                cursor = result.cursor
            } else {
                // Room messages via FrayGetMessages
                let result = try bridge.getMessages(home: nil, limit: limit, since: nil)
                messages = result.messages
                cursor = result.cursor
            }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    func startPolling(thread: FrayThread?, interval: TimeInterval = 1.0) {
        stopPolling()
        pollTimer = Timer.scheduledTimer(withTimeInterval: interval, repeats: true) { [weak self] _ in
            Task { await self?.pollNewMessages() }
        }
    }

    func stopPolling() {
        pollTimer?.invalidate()
        pollTimer = nil
    }

    /// Poll for new messages using cursor (guid + ts) to avoid duplicates
    /// When cursor is nil (empty thread/room), fetches latest to initialize
    private func pollNewMessages() async {
        guard !isLoading else { return }
        do {
            let result: MessagePage
            if let thread = currentThread {
                result = try bridge.getThreadMessages(threadGuid: thread.guid, limit: 50, since: cursor)
            } else {
                result = try bridge.getMessages(home: nil, limit: 50, since: cursor)
            }

            if !result.messages.isEmpty {
                // Dedupe by id before appending
                let existingIds = Set(messages.map { $0.id })
                let newMessages = result.messages.filter { !existingIds.contains($0.id) }
                messages.append(contentsOf: newMessages)
            }
            // Always update cursor (even when nil → first cursor)
            if let newCursor = result.cursor {
                self.cursor = newCursor
            }
        } catch {
            // Silent failure for polling
        }
    }

    /// Post a message to the current context (thread or room)
    func postMessage(body: String, from agent: String, replyTo: String?) async throws {
        // Derive home from current context: thread guid or nil for room
        let home = currentThread?.guid
        let message = try bridge.postMessage(
            body: body,
            from: agent,
            in: home,
            replyTo: replyTo
        )
        messages.append(message)
        // Update cursor to include new message
        cursor = MessageCursor(guid: message.id, ts: message.ts)
    }

    func addReaction(to messageId: String, emoji: String, from agent: String) async throws {
        try bridge.addReaction(to: messageId, emoji: emoji, from: agent)
        // Reload message to get updated reactions
        await refreshMessage(messageId)
    }
}
```

### 3.2 ThreadsViewModel

```swift
@Observable
final class ThreadsViewModel {
    private let bridge: FrayBridge

    private(set) var threads: [FrayThread] = []
    private(set) var threadTree: [String: [FrayThread]] = [:] // parentId -> children
    private(set) var rootThreads: [FrayThread] = []
    private(set) var isLoading: Bool = false

    var favedThreads: [FrayThread] {
        threads.filter { faves.contains($0.guid) }
    }

    var followingThreads: [FrayThread] {
        threads.filter { subscriptions.contains($0.guid) }
    }

    private var faves: Set<String> = []
    private var subscriptions: Set<String> = []

    func loadThreads() async {
        isLoading = true
        defer { isLoading = false }

        do {
            threads = try bridge.getThreads(parent: nil, includeArchived: false)
            buildTree()
        } catch {
            // Handle error
        }
    }

    private func buildTree() {
        threadTree = Dictionary(grouping: threads) { $0.parentThread ?? "" }
        rootThreads = threadTree[""] ?? []
    }

    func children(of thread: FrayThread) -> [FrayThread] {
        threadTree[thread.guid] ?? []
    }
}
```

### 3.3 AgentsViewModel

```swift
@Observable
final class AgentsViewModel {
    private let bridge: FrayBridge

    private(set) var agents: [FrayAgent] = []
    private(set) var activeAgents: [FrayAgent] = []
    private(set) var managedAgents: [FrayAgent] = []

    func loadAgents() async {
        do {
            agents = try bridge.getAgents()
            activeAgents = agents.filter { $0.presence == .active || $0.presence == .spawning }
            managedAgents = agents.filter { $0.managed }
        } catch {
            // Handle error
        }
    }
}
```

**AppState.swift** - coordinates global app state including current user identity:
```swift
import SwiftUI

@Observable
final class AppState {
    /// Currently logged-in agent ID (set during app launch or agent selection)
    var currentAgentId: String?

    /// Currently selected channel (project path)
    var currentChannel: String?

    /// User preferences
    var showTimestamps: Bool = true
    var compactMode: Bool = false
}
```

**Deliverables:**
- [ ] `MessagesViewModel.swift` with polling
- [ ] `ThreadsViewModel.swift` with tree building
- [ ] `AgentsViewModel.swift` with presence filtering
- [ ] `ChannelsViewModel.swift` for multi-project support
- [ ] `AppState.swift` for global state coordination

---

## Phase 4: Activity Panel & Agent Presence

### 4.1 ActivityPanelView

```swift
struct ActivityPanelView: View {
    @Environment(AgentsViewModel.self) private var agentsVM

    var body: some View {
        List {
            Section("Active") {
                ForEach(agentsVM.activeAgents) { agent in
                    AgentActivityRow(agent: agent)
                }
            }

            Section("Managed") {
                ForEach(agentsVM.managedAgents) { agent in
                    AgentActivityRow(agent: agent)
                }
            }
        }
        .listStyle(.sidebar)
    }
}
```

### 4.2 AgentActivityRow

Per design spec with:
- Presence indicator (with spawning animation)
- Agent name with color
- Status text
- Token usage bar (background visualization)

**Deliverables:**
- [ ] `ActivityPanelView.swift`
- [ ] `AgentActivityRow.swift` with token bar
- [ ] Spawning pulse animation
- [ ] Click-to-navigate to agent thread

---

## Phase 5: Rich Features

### 5.1 Thread Hierarchy Navigation

- Drill-in/out with h/l keys
- Breadcrumb navigation
- Parent thread indicator
- Nested indentation in list

### 5.2 Command Palette (⌘K)

```swift
struct CommandPalette: View {
    @State private var query: String = ""
    @State private var results: [CommandResult] = []

    var body: some View {
        VStack {
            TextField("Search threads, agents, commands...", text: $query)
                .textFieldStyle(.plain)
                .font(.title2)

            List(results) { result in
                CommandResultRow(result: result)
            }
        }
        .frame(width: 600, height: 400)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 16))
    }
}
```

### 5.3 Keyboard Navigation

| Shortcut | Action |
|----------|--------|
| ⌘N | Focus input |
| ⌘K | Command palette |
| ⌘0 | Toggle sidebar |
| ⌘I | Toggle activity |
| j/k | Navigate messages |
| h/l | Thread drill in/out |
| ⌘R | Reply to selected |
| Esc | Clear/close |

### 5.4 Code Syntax Highlighting

Use [Splash](https://github.com/JohnSundell/Splash) or [Highlightr](https://github.com/raspu/Highlightr) for code blocks.

**Deliverables:**
- [ ] Thread drill-in/out navigation
- [ ] Command palette (⌘K)
- [ ] Full keyboard shortcut support
- [ ] Code syntax highlighting integration

---

## Phase 6: Menu Bar App

Separate target for lightweight menu bar presence.

**Features:**
- Unread badge count
- Active agent list
- Quick compose popover
- Open main app

**Implementation:**
- Separate `FrayMenuBar` target
- Shares `FrayBridge` and types
- Uses `MenuBarExtra` (macOS 13+)

**Deliverables:**
- [ ] Menu bar target in Xcode project
- [ ] Unread count polling
- [ ] Quick compose sheet
- [ ] Deep link to main app

---

## Phase 7: Polish & Platform Integration

### 7.1 Notifications

```swift
class NotificationService {
    func requestPermission() async -> Bool { /* ... */ }

    func showMentionNotification(message: FrayMessage) {
        let content = UNMutableNotificationContent()
        content.title = "@\(message.fromAgent)"
        content.body = message.body.prefix(100)
        content.sound = .default

        let request = UNNotificationRequest(
            identifier: message.guid,
            content: content,
            trigger: nil
        )

        UNUserNotificationCenter.current().add(request)
    }
}
```

### 7.2 Window State Persistence

```swift
@SceneStorage("windowFrame") private var windowFrame: Data?
@SceneStorage("sidebarWidth") private var sidebarWidth: CGFloat = 280
@SceneStorage("selectedThread") private var selectedThreadId: String?
```

### 7.3 Light Mode Support

All colors should use semantic system colors or have light mode variants.

### 7.4 Accessibility

- VoiceOver labels for all interactive elements
- Reduced motion support (disable spawning animations)
- Reduced transparency fallbacks
- High contrast mode support

**Deliverables:**
- [ ] macOS notification integration
- [ ] Window state persistence
- [ ] Light mode theme
- [ ] Full accessibility audit
- [ ] Reduced motion support

---

## Testing Strategy

### Unit Tests
- FFI bridge (mock C responses)
- ViewModels (mock bridge)
- Color hash consistency
- Markdown parsing

### Integration Tests
- Real FFI calls with test database
- Message posting and retrieval
- Thread operations

### UI Tests
- Navigation flows
- Keyboard shortcuts
- Input area behavior

---

## Build & Distribution

### Development
```bash
# Build Go library
make dylib

# Open Xcode
open FrayApp/FrayApp.xcodeproj
```

### Release
1. Archive in Xcode
2. Notarize with Apple
3. Create DMG installer
4. Optionally submit to App Store (requires removing dylib embedding)

**Makefile targets:**
```makefile
.PHONY: dylib clean

dylib:
	CGO_ENABLED=1 go build -buildmode=c-shared -o build/libfray.dylib ./internal/ffi

clean:
	rm -rf build/
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| cgo complexity | Start with minimal exports, expand incrementally |
| Memory leaks | Strict handle management, `FrayFreeString` for all allocations |
| Liquid Glass API changes | Abstract behind protocols, fallback to solid backgrounds |
| Performance (polling) | Debouncing, incremental updates only |
| Code signing | Test on multiple machines early |

---

## Success Criteria

Phase 1 complete when:
- [ ] `libfray.dylib` builds successfully
- [ ] Swift can call FFI functions
- [ ] Basic types serialize correctly

Phase 2 complete when:
- [ ] 3-column layout renders
- [ ] Messages display correctly
- [ ] Can post messages

Full MVP when:
- [ ] Real-time message updates
- [ ] Thread navigation works
- [ ] Agent presence displays
- [ ] Keyboard shortcuts functional

---

## Timeline Recommendation

This plan is structured for incremental delivery. Phase 1 (Foundation) is the riskiest and should be completed first to validate the FFI approach before investing in UI. Each subsequent phase builds on the previous and can be delivered independently.
