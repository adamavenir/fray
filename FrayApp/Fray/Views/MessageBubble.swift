import SwiftUI
import AppKit

struct MessageBubble: View {
    let message: FrayMessage
    var onReply: (() -> Void)?
    var showHeader: Bool = true

    @State private var isHovering = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        HStack(alignment: .top, spacing: FraySpacing.sm) {
            AgentAvatar(agentId: message.fromAgent)

            VStack(alignment: .leading, spacing: FraySpacing.xs) {
                if showHeader {
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
                }

                MessageContent(content: message.body)

                if !message.reactions.isEmpty {
                    ReactionBar(reactions: message.reactions) { emoji in
                        print("React with \(emoji)")
                    }
                }

                MessageFooter(message: message)
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
                withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
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
        let interval = Date().timeIntervalSince(date)

        if interval < 60 { return "just now" }
        if interval < 3600 { return "\(Int(interval / 60))m ago" }
        if interval < 86400 { return "\(Int(interval / 3600))h ago" }
        if interval < 604800 { return "\(Int(interval / 86400))d ago" }

        let formatter = DateFormatter()
        formatter.dateStyle = .short
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
            .accessibilityLabel("Reply to message")

            Button(action: { print("React") }) {
                Image(systemName: "face.smiling")
            }
            .buttonStyle(.borderless)
            .help("Add reaction")
            .accessibilityLabel("Add reaction")

            Button(action: { print("More") }) {
                Image(systemName: "ellipsis")
            }
            .buttonStyle(.borderless)
            .help("More actions")
            .accessibilityLabel("More actions")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
    }
}

struct MessageContent: View {
    let content: String

    var body: some View {
        Text(parseContent(content))
            .font(FrayTypography.messageBody)
            .lineSpacing(FrayTypography.messageLineSpacing)
            .textSelection(.enabled)
            .environment(\.openURL, OpenURLAction { url in
                if url.scheme == "frayid" {
                    let id = "#\(url.host ?? "")"
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(id, forType: .string)
                    return .handled
                }
                return .systemAction
            })
    }

    private func parseContent(_ text: String) -> AttributedString {
        var result: AttributedString
        do {
            var options = AttributedString.MarkdownParsingOptions()
            options.interpretedSyntax = .inlineOnlyPreservingWhitespace
            result = try AttributedString(markdown: text, options: options)
        } catch {
            result = AttributedString(text)
        }

        // Find and style #fray-id patterns (including .n suffix like #fray-abc.1)
        // Pattern: #prefix-id or #prefix-id.n (e.g. #fray-abc123 or #fray-abc123.1)
        let idPattern = try? Regex("#[a-z]+-[a-z0-9]+(?:\\.[a-z0-9]+)?")
        guard let pattern = idPattern else { return result }

        let plainText = String(result.characters)
        for match in plainText.matches(of: pattern) {
            let matchStr = String(plainText[match.range])
            if let attrRange = result.range(of: matchStr) {
                result[attrRange].font = Font.system(size: 15, weight: .bold)
                if let url = URL(string: "frayid://\(matchStr.dropFirst())") {
                    result[attrRange].link = url
                }
            }
        }

        return result
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

struct MessageFooter: View {
    let message: FrayMessage

    var body: some View {
        HStack(spacing: FraySpacing.sm) {
            CopyableIdText(id: message.id)

            if let sessionId = message.sessionId {
                Text("•")
                    .font(.caption2)
                    .foregroundStyle(.quaternary)
                // Display @agent#sessid (first 5 chars), copy full @agent#sessionId
                let shortSess = String(sessionId.prefix(5))
                let displayText = "@\(message.fromAgent)#\(shortSess)"
                let copyText = "@\(message.fromAgent)#\(sessionId)"
                CopyableIdText(id: displayText, copyValue: copyText)
            }

            Spacer()
        }
        .padding(.top, FraySpacing.xs)
    }
}

struct CopyableIdText: View {
    let id: String
    var copyValue: String?
    @State private var isCopied = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var valueToCopy: String {
        copyValue ?? id
    }

    var body: some View {
        Text(id)
            .font(.caption2.monospaced())
            .foregroundStyle(isCopied ? AnyShapeStyle(Color.accentColor) : AnyShapeStyle(.quaternary))
            .contentShape(Rectangle())
            .onTapGesture {
                NSPasteboard.general.clearContents()
                NSPasteboard.general.setString(valueToCopy, forType: .string)
                if reduceMotion {
                    isCopied = true
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                        isCopied = false
                    }
                } else {
                    withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                        isCopied = true
                    }
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {
                        withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                            isCopied = false
                        }
                    }
                }
            }
            .help("Click to copy: \(valueToCopy)")
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
