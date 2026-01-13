import SwiftUI

struct SidebarView: View {
    @Binding var selectedThread: FrayThread?
    @Environment(FrayBridge.self) private var bridge

    @State private var threads: [FrayThread] = []
    @AppStorage("pinnedThreadIds") private var pinnedThreadIdsData: Data = Data()

    var pinnedThreadIds: Set<String> {
        get {
            (try? JSONDecoder().decode(Set<String>.self, from: pinnedThreadIdsData)) ?? []
        }
        nonmutating set {
            pinnedThreadIdsData = (try? JSONEncoder().encode(newValue)) ?? Data()
        }
    }

    var pinnedThreads: [FrayThread] {
        threads.filter { pinnedThreadIds.contains($0.guid) }
    }

    var unpinnedRootThreads: [FrayThread] {
        threads.filter { $0.parentThread == nil && !pinnedThreadIds.contains($0.guid) }
    }

    var body: some View {
        List(selection: $selectedThread) {
            Section("Room") {
                Label("Main", systemImage: "bubble.left.and.bubble.right")
                    .tag(nil as FrayThread?)
            }

            if !pinnedThreads.isEmpty {
                Section("Pinned") {
                    ForEach(pinnedThreads) { thread in
                        PinnedThreadRow(
                            thread: thread,
                            selectedThread: $selectedThread,
                            onUnpin: { unpinThread(thread.guid) }
                        )
                    }
                }
            }

            Section("Threads") {
                ThreadListSection(
                    threads: threads,
                    pinnedIds: pinnedThreadIds,
                    selectedThread: $selectedThread,
                    onPin: { pinThread($0) }
                )
            }
        }
        .listStyle(.sidebar)
        .navigationTitle("Fray")
        .task {
            await loadData()
        }
    }

    private func loadData() async {
        do {
            threads = try bridge.getThreads()
        } catch {
            print("Failed to load sidebar data: \(error)")
        }
    }

    private func pinThread(_ guid: String) {
        var ids = pinnedThreadIds
        ids.insert(guid)
        pinnedThreadIds = ids
    }

    private func unpinThread(_ guid: String) {
        var ids = pinnedThreadIds
        ids.remove(guid)
        pinnedThreadIds = ids
    }
}

struct PinnedThreadRow: View {
    let thread: FrayThread
    @Binding var selectedThread: FrayThread?
    let onUnpin: () -> Void

    @State private var isHovering = false

    var body: some View {
        HStack(spacing: FraySpacing.md) {
            Image(systemName: "star.fill")
                .foregroundStyle(.yellow)
                .font(.caption)

            Text(thread.name)
                .lineLimit(1)

            Spacer()

            if isHovering {
                Button(action: onUnpin) {
                    Image(systemName: "star.slash")
                        .font(.caption)
                }
                .buttonStyle(.borderless)
                .help("Unpin")
            }
        }
        .contentShape(Rectangle())
        .onTapGesture {
            selectedThread = thread
        }
        .onHover { isHovering = $0 }
        .tag(thread)
    }
}

struct AgentListSection: View {
    let agents: [FrayAgent]

    var body: some View {
        ForEach(agents) { agent in
            AgentListItem(agent: agent)
        }
    }
}

struct AgentListItem: View {
    let agent: FrayAgent

    var body: some View {
        HStack(spacing: FraySpacing.sm) {
            AgentAvatar(agentId: agent.agentId, size: 20)

            Text("@\(agent.agentId)")
                .font(FrayTypography.agentName)

            Spacer()

            if let presence = agent.presence {
                PresenceIndicator(presence: presence)
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(agentAccessibilityLabel)
    }

    private var agentAccessibilityLabel: String {
        var label = "Agent \(agent.agentId)"
        if let presence = agent.presence {
            label += ", \(presenceText(presence))"
        }
        return label
    }

    private func presenceText(_ presence: FrayAgent.AgentPresence) -> String {
        switch presence {
        case .active: return "active"
        case .spawning: return "spawning"
        case .prompting, .prompted: return "prompting"
        case .idle: return "idle"
        case .error: return "error"
        case .offline: return "offline"
        case .brb: return "will be right back"
        }
    }
}

struct AgentAvatar: View {
    let agentId: String
    var size: CGFloat = FraySpacing.avatarSize

    var body: some View {
        Circle()
            .fill(FrayColors.colorForAgent(agentId))
            .frame(width: size, height: size)
            .overlay {
                Text(String(agentId.prefix(1)).uppercased())
                    .font(.system(size: size * 0.5, weight: .semibold))
                    .foregroundStyle(.white)
            }
            .accessibilityHidden(true)
    }
}

struct PresenceIndicator: View {
    let presence: FrayAgent.AgentPresence

    var body: some View {
        Circle()
            .fill(FrayColors.presence[presence] ?? .gray)
            .frame(width: 8, height: 8)
    }
}

struct ThreadListSection: View {
    let threads: [FrayThread]
    let pinnedIds: Set<String>
    @Binding var selectedThread: FrayThread?
    let onPin: (String) -> Void

    var rootThreads: [FrayThread] {
        threads.filter { $0.parentThread == nil && !pinnedIds.contains($0.guid) }
    }

    var body: some View {
        ForEach(rootThreads) { thread in
            ThreadListItem(
                thread: thread,
                allThreads: threads,
                pinnedIds: pinnedIds,
                selectedThread: $selectedThread,
                onPin: onPin
            )
        }
    }
}

struct ThreadListItem: View {
    let thread: FrayThread
    let allThreads: [FrayThread]
    let pinnedIds: Set<String>
    @Binding var selectedThread: FrayThread?
    let onPin: (String) -> Void

    @State private var isExpanded = false
    @State private var isHovering = false

    var childThreads: [FrayThread] {
        allThreads.filter { $0.parentThread == thread.guid && !pinnedIds.contains($0.guid) }
    }

    var hasChildren: Bool {
        !childThreads.isEmpty
    }

    var body: some View {
        if hasChildren {
            DisclosureGroup(isExpanded: $isExpanded) {
                ForEach(childThreads) { child in
                    ThreadListItem(
                        thread: child,
                        allThreads: allThreads,
                        pinnedIds: pinnedIds,
                        selectedThread: $selectedThread,
                        onPin: onPin
                    )
                }
            } label: {
                threadLabel
            }
            .tag(thread)
        } else {
            threadLabel
                .tag(thread)
        }
    }

    private var threadLabel: some View {
        HStack(spacing: FraySpacing.md) {
            Text(thread.name)
                .lineLimit(1)
                .foregroundStyle(thread.status == .archived ? .secondary : .primary)

            Spacer()

            if isHovering {
                Button(action: { onPin(thread.guid) }) {
                    Image(systemName: "star")
                        .font(.caption)
                }
                .buttonStyle(.borderless)
                .help("Pin thread")
            }

            if thread.type == .knowledge {
                Image(systemName: "brain")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .contentShape(Rectangle())
        .onTapGesture {
            selectedThread = thread
        }
        .onHover { isHovering = $0 }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(threadAccessibilityLabel)
        .accessibilityAddTraits(.isButton)
    }

    private var threadAccessibilityLabel: String {
        var parts = ["Thread", thread.name]
        if thread.status == .archived {
            parts.append("archived")
        }
        if thread.type == .knowledge {
            parts.append("knowledge type")
        }
        if hasChildren {
            parts.append("\(childThreads.count) sub-threads")
        }
        return parts.joined(separator: ", ")
    }
}

#Preview {
    SidebarView(selectedThread: .constant(nil))
        .environment(FrayBridge())
        .frame(width: 280)
}
