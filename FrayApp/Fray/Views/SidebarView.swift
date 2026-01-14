import SwiftUI

enum SidebarConstants {
    static let iconColumnWidth: CGFloat = 20
}

struct SidebarView: View {
    @Binding var selectedThread: FrayThread?
    @Binding var currentChannel: FrayChannel?
    let currentAgentId: String?

    @Environment(FrayBridge.self) private var bridge

    @State private var threads: [FrayThread] = []
    @State private var channels: [FrayChannel] = []
    @State private var favedThreadGuids: Set<String> = []
    @State private var pollTimer: Timer?

    var favedThreads: [FrayThread] {
        threads.filter { favedThreadGuids.contains($0.guid) }
    }

    var unfavedRootThreads: [FrayThread] {
        threads.filter { $0.parentThread == nil && !favedThreadGuids.contains($0.guid) }
    }

    var body: some View {
        List(selection: $selectedThread) {
            channelPickerSection
            roomSection
            favedThreadsSection
            unfavedThreadsSection
        }
        .listStyle(.sidebar)
        .navigationTitle(currentChannel?.name ?? "Fray")
        .task {
            await loadData()
            startPolling()
        }
        .onDisappear {
            stopPolling()
        }
        .onChange(of: bridge.projectPath) { _, _ in
            // Reload when bridge connects to a different project
            Task { await loadThreadsAndFaves() }
        }
    }

    private func startPolling() {
        stopPolling()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 3.0, repeats: true) { _ in
            Task { @MainActor in
                await pollThreads()
            }
        }
    }

    private func stopPolling() {
        pollTimer?.invalidate()
        pollTimer = nil
    }

    private func pollThreads() async {
        do {
            let newThreads = try bridge.getThreads()
            // Only update if there's a change
            if newThreads.count != threads.count ||
               newThreads.map({ $0.guid }).sorted() != threads.map({ $0.guid }).sorted() {
                threads = newThreads
            }
        } catch {
            // Silent failure for polling
        }
    }

    @ViewBuilder
    private var channelPickerSection: some View {
        if channels.count > 1 {
            Picker("Channel", selection: $currentChannel) {
                ForEach(channels) { channel in
                    Text(channel.name)
                        .tag(channel as FrayChannel?)
                }
            }
            .pickerStyle(.menu)
            .labelsHidden()
            .padding(.bottom, FraySpacing.sm)
        }
    }

    @ViewBuilder
    private var roomSection: some View {
        SidebarRow(
            icon: "❖",
            title: currentChannel?.name ?? "Room",
            isChannel: true,
            isSelected: selectedThread == nil,
            isFaved: false,
            onSelect: { selectedThread = nil },
            onFaveToggle: nil
        )
    }

    @ViewBuilder
    private var favedThreadsSection: some View {
        if !favedThreads.isEmpty {
            ForEach(favedThreads) { thread in
                SidebarRow(
                    icon: nil,
                    title: thread.name,
                    isChannel: false,
                    isSelected: selectedThread?.guid == thread.guid,
                    isFaved: true,
                    onSelect: { selectedThread = thread },
                    onFaveToggle: { unfaveThread(thread.guid) }
                )
            }
            // Divider under faved section
            Divider()
                .padding(.vertical, FraySpacing.xs)
        }
    }

    @ViewBuilder
    private var unfavedThreadsSection: some View {
        ForEach(unfavedRootThreads) { thread in
            ThreadListItem(
                thread: thread,
                allThreads: threads,
                favedIds: favedThreadGuids,
                selectedThread: $selectedThread,
                onFave: { faveThread($0) }
            )
        }
    }

    private func loadData() async {
        // Load channels
        do {
            channels = try FrayBridge.listChannels()
            // If no current channel but we have channels, select first
            if currentChannel == nil, let first = channels.first {
                currentChannel = first
            }
        } catch {
            print("Failed to load channels: \(error)")
        }

        // Load threads
        do {
            threads = try bridge.getThreads()
        } catch {
            print("Failed to load threads: \(error)")
        }

        // Load faves from fray
        await loadFaves()
    }

    private func loadThreadsAndFaves() async {
        // Load threads for current channel
        do {
            threads = try bridge.getThreads()
        } catch {
            print("Failed to load threads: \(error)")
            threads = []
        }

        // Load faves
        await loadFaves()
    }

    private func loadFaves() async {
        guard let agentId = currentAgentId else {
            favedThreadGuids = []
            return
        }

        do {
            let faves = try bridge.getFaves(agentId: agentId, itemType: "thread")
            favedThreadGuids = Set(faves.map { $0.itemGuid })
        } catch {
            print("Failed to load faves: \(error)")
            favedThreadGuids = []
        }
    }

    private func faveThread(_ guid: String) {
        guard let agentId = currentAgentId else { return }
        Task {
            do {
                try bridge.faveItem(itemGuid: guid, agentId: agentId)
                favedThreadGuids.insert(guid)
            } catch {
                print("Failed to fave thread: \(error)")
            }
        }
    }

    private func unfaveThread(_ guid: String) {
        guard let agentId = currentAgentId else { return }
        Task {
            do {
                try bridge.unfaveItem(itemGuid: guid, agentId: agentId)
                favedThreadGuids.remove(guid)
            } catch {
                print("Failed to unfave thread: \(error)")
            }
        }
    }
}

// Unified sidebar row with consistent hover/select styling
struct SidebarRow: View {
    let icon: String?
    let title: String
    let isChannel: Bool
    let isSelected: Bool
    let isFaved: Bool
    let onSelect: () -> Void
    var onFaveToggle: (() -> Void)?

    @State private var isHovering = false
    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        HStack(spacing: FraySpacing.sm) {
            // Icon column (optional - for channel)
            if let icon = icon {
                Text(icon)
                    .font(.system(size: 12))
                    .frame(width: SidebarConstants.iconColumnWidth, alignment: .center)
            }

            Text(title)
                .font(isChannel ? FrayTypography.sidebarChannel : FrayTypography.sidebarThread)
                .lineLimit(1)
                .foregroundStyle(isSelected ? .white : .primary)

            Spacer()

            // Star on the right (only show on hover for non-faved, always show for faved)
            if onFaveToggle != nil {
                Button(action: { onFaveToggle?() }) {
                    Image(systemName: isFaved ? "star.fill" : "star")
                        .font(.caption)
                        .foregroundStyle(isSelected ? .white : (isFaved ? .yellow : .secondary))
                }
                .buttonStyle(.borderless)
                .opacity(isFaved || isHovering ? 1 : 0)
                .accessibilityLabel(isFaved ? "Remove from favorites" : "Add to favorites")
            }
        }
        .padding(.horizontal, FraySpacing.sm)
        .padding(.vertical, FraySpacing.xs)
        .background {
            RoundedRectangle(cornerRadius: FraySpacing.smallCornerRadius)
                .fill(isSelected ? Color.accentColor : (isHovering ? FrayColors.hoverFill.resolve(for: colorScheme) : Color.clear))
        }
        .contentShape(Rectangle())
        .onTapGesture { onSelect() }
        .onHover { isHovering = $0 }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(isChannel ? "Channel: \(title)" : (isFaved ? "Favorited thread: \(title)" : "Thread: \(title)"))
        .accessibilityAddTraits(.isButton)
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

struct ThreadListItem: View {
    let thread: FrayThread
    let allThreads: [FrayThread]
    let favedIds: Set<String>
    @Binding var selectedThread: FrayThread?
    let onFave: (String) -> Void

    @State private var isExpanded = false
    @State private var isHovering = false
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var childThreads: [FrayThread] {
        allThreads.filter { $0.parentThread == thread.guid && !favedIds.contains($0.guid) }
    }

    var hasChildren: Bool {
        !childThreads.isEmpty
    }

    var isSelected: Bool {
        selectedThread?.guid == thread.guid
    }

    var body: some View {
        VStack(spacing: 0) {
            threadRow

            if hasChildren && isExpanded {
                ForEach(childThreads) { child in
                    ThreadListItem(
                        thread: child,
                        allThreads: allThreads,
                        favedIds: favedIds,
                        selectedThread: $selectedThread,
                        onFave: onFave
                    )
                    .padding(.leading, FraySpacing.md)
                }
            }
        }
    }

    private var threadRow: some View {
        HStack(spacing: FraySpacing.sm) {
            // Expand/collapse chevron for threads with children
            if hasChildren {
                Button(action: {
                    if reduceMotion {
                        isExpanded.toggle()
                    } else {
                        withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                            isExpanded.toggle()
                        }
                    }
                }) {
                    Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                        .font(.caption2)
                        .foregroundStyle(isSelected ? AnyShapeStyle(.white) : AnyShapeStyle(.tertiary))
                }
                .buttonStyle(.borderless)
                .frame(width: 16)
            }

            Text(thread.name)
                .font(FrayTypography.sidebarThread)
                .lineLimit(1)
                .foregroundStyle(isSelected ? .white : (thread.status == .archived ? .secondary : .primary))

            Spacer()

            if thread.type == .knowledge {
                Image(systemName: "brain")
                    .font(.caption)
                    .foregroundStyle(isSelected ? .white.opacity(0.7) : .secondary)
            }

            // Star on the right
            Button(action: { onFave(thread.guid) }) {
                Image(systemName: "star")
                    .font(.caption)
                    .foregroundStyle(isSelected ? .white : .secondary)
            }
            .buttonStyle(.borderless)
            .opacity(isHovering ? 1 : 0)
        }
        .padding(.horizontal, FraySpacing.sm)
        .padding(.vertical, FraySpacing.xs)
        .background {
            RoundedRectangle(cornerRadius: FraySpacing.smallCornerRadius)
                .fill(isSelected ? Color.accentColor : (isHovering ? FrayColors.hoverFill.resolve(for: colorScheme) : Color.clear))
        }
        .contentShape(Rectangle())
        .onTapGesture { selectedThread = thread }
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
    SidebarView(
        selectedThread: .constant(nil),
        currentChannel: .constant(nil),
        currentAgentId: "preview-user"
    )
    .environment(FrayBridge())
    .frame(width: 280)
}
