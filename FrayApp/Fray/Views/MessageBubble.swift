import SwiftUI

struct MessageBubble: View {
    let message: FrayMessage
    var onReply: (() -> Void)?

    @State private var isHovering = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        HStack(alignment: .top, spacing: FraySpacing.sm) {
            AgentAvatar(agentId: message.fromAgent)

            VStack(alignment: .leading, spacing: FraySpacing.xs) {
                HStack(spacing: FraySpacing.sm) {
                    Text("@\(message.fromAgent)")
                        .font(FrayTypography.agentName)
                        .foregroundStyle(FrayColors.colorForAgent(message.fromAgent))

                    Text(formatTimestamp(message.ts))
                        .font(FrayTypography.timestamp)
                        .foregroundStyle(.secondary)

                    if message.edited == true {
                        Text("(edited)")
                            .font(FrayTypography.caption)
                            .foregroundStyle(.tertiary)
                    }

                    Spacer()

                    if isHovering {
                        MessageActions(message: message, onReply: onReply)
                    }
                }

                MessageContent(content: message.body)

                if !message.reactions.isEmpty {
                    ReactionBar(reactions: message.reactions) { emoji in
                        print("React with \(emoji)")
                    }
                }
            }
        }
        .padding(FraySpacing.messagePadding)
        .background {
            RoundedRectangle(cornerRadius: FraySpacing.cornerRadius)
                .fill(isHovering ? FrayColors.secondaryBackground : .clear)
        }
        .onHover { hovering in
            if reduceMotion {
                isHovering = hovering
            } else {
                withAnimation(.easeInOut(duration: 0.15)) {
                    isHovering = hovering
                }
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("Message from \(message.fromAgent)")
        .accessibilityValue(message.body)
        .accessibilityHint("Double-tap to reply")
        .accessibilityAction(named: "Reply") {
            onReply?()
        }
    }

    private func formatTimestamp(_ ts: Int64) -> String {
        let date = Date(timeIntervalSince1970: TimeInterval(ts))
        let formatter = DateFormatter()
        formatter.timeStyle = .short
        return formatter.string(from: date)
    }
}

struct MessageActions: View {
    let message: FrayMessage
    var onReply: (() -> Void)?

    var body: some View {
        HStack(spacing: FraySpacing.xs) {
            Button(action: { onReply?() }) {
                Image(systemName: "arrowshape.turn.up.left")
            }
            .buttonStyle(.borderless)
            .help("Reply")

            Button(action: { print("React") }) {
                Image(systemName: "face.smiling")
            }
            .buttonStyle(.borderless)
            .help("Add reaction")

            Button(action: { print("More") }) {
                Image(systemName: "ellipsis")
            }
            .buttonStyle(.borderless)
            .help("More actions")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
    }
}

struct MessageContent: View {
    let content: String

    var body: some View {
        Text(parseMarkdown(content))
            .font(FrayTypography.messageBody)
            .textSelection(.enabled)
    }

    private func parseMarkdown(_ text: String) -> AttributedString {
        do {
            var options = AttributedString.MarkdownParsingOptions()
            options.interpretedSyntax = .inlineOnlyPreservingWhitespace
            var result = try AttributedString(markdown: text, options: options)
            return result
        } catch {
            return AttributedString(text)
        }
    }
}

struct ReactionBar: View {
    let reactions: [String: [ReactionEntry]]
    let onReact: (String) -> Void

    var body: some View {
        HStack(spacing: FraySpacing.xs) {
            ForEach(Array(reactions.keys.sorted()), id: \.self) { emoji in
                ReactionPill(emoji: emoji, entries: reactions[emoji] ?? [])
                    .onTapGesture { onReact(emoji) }
            }

            Button(action: { }) {
                Image(systemName: "face.smiling")
                    .font(.caption)
            }
            .buttonStyle(.borderless)
            .opacity(0.5)
        }
    }
}

struct ReactionPill: View {
    let emoji: String
    let entries: [ReactionEntry]

    var body: some View {
        HStack(spacing: FraySpacing.xs) {
            Text(emoji)
            Text("\(entries.count)")
                .font(FrayTypography.reactionCount)
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, FraySpacing.sm)
        .padding(.vertical, FraySpacing.xs)
        .background {
            Capsule()
                .fill(FrayColors.tertiaryBackground)
        }
    }
}

#Preview {
    MessageBubble(
        message: FrayMessage(
            id: "msg-test123",
            ts: Int64(Date().timeIntervalSince1970),
            channelId: nil,
            home: nil,
            fromAgent: "opus",
            sessionId: nil,
            body: "Hello world! This is a **test** message with `code`.",
            mentions: [],
            forkSessions: nil,
            reactions: ["👍": [ReactionEntry(agentId: "adam", reactedAt: 0)]],
            type: .agent,
            references: nil,
            surfaceMessage: nil,
            replyTo: nil,
            quoteMessageGuid: nil,
            editedAt: nil,
            edited: false,
            editCount: nil,
            archivedAt: nil
        )
    )
    .padding()
}
