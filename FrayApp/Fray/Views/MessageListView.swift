import SwiftUI

struct MessageListView: View {
    let thread: FrayThread?
    let currentAgentId: String?
    @Environment(FrayBridge.self) private var bridge

    @State private var messages: [FrayMessage] = []
    @State private var cursor: MessageCursor?
    @State private var scrollPosition: String?
    @State private var inputText: String = ""
    @State private var replyTo: FrayMessage?
    @State private var isLoading = false
    @State private var pollTimer: Timer?

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: FraySpacing.messageSpacing) {
                    ForEach(messages) { message in
                        MessageBubble(message: message, onReply: { replyTo = message })
                            .id(message.id)
                    }
                }
                .padding()
            }
            .scrollPosition(id: $scrollPosition, anchor: .bottom)
            .onChange(of: messages.count) {
                if let last = messages.last {
                    withAnimation {
                        proxy.scrollTo(last.id, anchor: .bottom)
                    }
                }
            }
        }
        .safeAreaInset(edge: .bottom) {
            MessageInputArea(
                text: $inputText,
                replyTo: $replyTo,
                onSubmit: handleSubmit
            )
            .padding()
        }
        .navigationTitle(thread?.name ?? "Room")
        .task(id: thread?.guid) {
            await loadMessages()
            startPolling()
        }
        .onDisappear {
            stopPolling()
        }
    }

    private func startPolling() {
        stopPolling()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { _ in
            Task { @MainActor in
                await pollNewMessages()
            }
        }
    }

    private func stopPolling() {
        pollTimer?.invalidate()
        pollTimer = nil
    }

    private func pollNewMessages() async {
        guard !isLoading else { return }

        do {
            let result: MessagePage
            if let thread = thread {
                result = try bridge.getThreadMessages(threadGuid: thread.guid, limit: 50, since: cursor)
            } else {
                result = try bridge.getMessages(home: nil, limit: 50, since: cursor)
            }

            if !result.messages.isEmpty {
                let existingIds = Set(messages.map { $0.id })
                let newMessages = result.messages.filter { !existingIds.contains($0.id) }
                if !newMessages.isEmpty {
                    messages.append(contentsOf: newMessages)
                }
            }

            if let newCursor = result.cursor {
                cursor = newCursor
            }
        } catch {
            // Silent failure for polling
        }
    }

    private func loadMessages() async {
        isLoading = true
        defer { isLoading = false }

        do {
            let page: MessagePage
            if let thread = thread {
                page = try bridge.getThreadMessages(threadGuid: thread.guid)
            } else {
                page = try bridge.getMessages(home: nil)
            }
            messages = page.messages
            cursor = page.cursor
        } catch {
            print("Failed to load messages: \(error)")
        }
    }

    private func handleSubmit(_ body: String) {
        guard let agentId = currentAgentId else {
            print("No agent ID set, cannot post")
            return
        }

        Task {
            do {
                let message = try bridge.postMessage(
                    body: body,
                    from: agentId,
                    in: thread?.name,
                    replyTo: replyTo?.id
                )
                messages.append(message)
                inputText = ""
                replyTo = nil
            } catch {
                print("Failed to post message: \(error)")
            }
        }
    }
}

struct RoomView: View {
    let currentAgentId: String?

    var body: some View {
        MessageListView(thread: nil, currentAgentId: currentAgentId)
    }
}

#Preview {
    MessageListView(thread: nil, currentAgentId: "preview-user")
        .environment(FrayBridge())
}
