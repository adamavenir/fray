import SwiftUI

struct AgentActivityRow: View {
    let agent: FrayAgent
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        HStack(spacing: FraySpacing.sm) {
            PresenceIndicatorAnimated(presence: agent.presence ?? .offline)

            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: FraySpacing.xs) {
                    Text("@\(agent.agentId)")
                        .font(FrayTypography.agentName)
                        .foregroundStyle(FrayColors.colorForAgent(agent.agentId))

                    if agent.managed == true {
                        Image(systemName: "gearshape.fill")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }

                if let status = agent.status, !status.isEmpty {
                    Text(status)
                        .font(FrayTypography.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }

                if let lastActivity = formattedLastActivity {
                    Text(lastActivity)
                        .font(FrayTypography.timestamp)
                        .foregroundStyle(.tertiary)
                }
            }

            Spacer()

            if let presence = agent.presence {
                Text(presence.rawValue)
                    .font(.caption2)
                    .foregroundStyle(presenceTextColor(presence))
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(presenceBackgroundColor(presence))
                    .clipShape(Capsule())
            }
        }
        .padding(.vertical, 4)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(accessibilityLabel)
    }

    private var accessibilityLabel: String {
        var parts = ["Agent \(agent.agentId)"]
        if let presence = agent.presence {
            parts.append(presenceDescription(presence))
        }
        if let status = agent.status, !status.isEmpty {
            parts.append(status)
        }
        if let lastActivity = formattedLastActivity {
            parts.append("Last active \(lastActivity)")
        }
        return parts.joined(separator: ", ")
    }

    private func presenceDescription(_ presence: FrayAgent.AgentPresence) -> String {
        switch presence {
        case .active: return "active"
        case .spawning: return "spawning"
        case .prompting: return "prompting"
        case .prompted: return "prompted"
        case .idle: return "idle"
        case .error: return "error"
        case .offline: return "offline"
        case .brb: return "will be right back"
        }
    }

    private var formattedLastActivity: String? {
        let ts = agent.lastHeartbeat ?? agent.lastSeen
        guard ts > 0 else { return nil }

        let date = Date(timeIntervalSince1970: Double(ts))
        let now = Date()
        let interval = now.timeIntervalSince(date)

        if interval < 60 {
            return "just now"
        } else if interval < 3600 {
            let mins = Int(interval / 60)
            return "\(mins)m ago"
        } else if interval < 86400 {
            let hours = Int(interval / 3600)
            return "\(hours)h ago"
        } else {
            let days = Int(interval / 86400)
            return "\(days)d ago"
        }
    }

    private func presenceTextColor(_ presence: FrayAgent.AgentPresence) -> Color {
        switch presence {
        case .active: return .green
        case .spawning: return .yellow
        case .prompting, .prompted: return .orange
        case .idle: return .gray
        case .error: return .red
        case .offline: return .gray
        case .brb: return .purple
        }
    }

    private func presenceBackgroundColor(_ presence: FrayAgent.AgentPresence) -> Color {
        presenceTextColor(presence).opacity(0.15)
    }
}

struct PresenceIndicatorAnimated: View {
    let presence: FrayAgent.AgentPresence
    @State private var isAnimating = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        Circle()
            .fill(FrayColors.presence[presence] ?? .gray)
            .frame(width: 10, height: 10)
            .overlay {
                if presence == .spawning && !reduceMotion {
                    Circle()
                        .stroke(FrayColors.presence[presence] ?? .yellow, lineWidth: 2)
                        .scaleEffect(isAnimating ? 2.0 : 1.0)
                        .opacity(isAnimating ? 0 : 0.5)
                        .animation(
                            .easeOut(duration: 1.0).repeatForever(autoreverses: false),
                            value: isAnimating
                        )
                }
            }
            .onAppear {
                if presence == .spawning && !reduceMotion {
                    isAnimating = true
                }
            }
            .onChange(of: presence) { _, newValue in
                isAnimating = newValue == .spawning && !reduceMotion
            }
            .accessibilityLabel(presenceAccessibilityLabel)
    }

    private var presenceAccessibilityLabel: String {
        switch presence {
        case .active: return "Active"
        case .spawning: return "Spawning"
        case .prompting: return "Prompting"
        case .prompted: return "Prompted"
        case .idle: return "Idle"
        case .error: return "Error"
        case .offline: return "Offline"
        case .brb: return "Will be right back"
        }
    }
}

#Preview {
    VStack(spacing: 16) {
        AgentActivityRow(agent: FrayAgent(
            guid: "usr-12345678",
            agentId: "opus",
            status: "Working on macOS client",
            purpose: nil,
            avatar: nil,
            registeredAt: 0,
            lastSeen: Int64(Date().timeIntervalSince1970 * 1000) - 300000,
            leftAt: nil,
            managed: true,
            invoke: nil,
            presence: .active,
            mentionWatermark: nil,
            reactionWatermark: nil,
            lastHeartbeat: Int64(Date().timeIntervalSince1970 * 1000) - 60000,
            lastSessionId: nil,
            sessionMode: nil,
            jobId: nil,
            jobIdx: nil,
            isEphemeral: nil
        ))

        AgentActivityRow(agent: FrayAgent(
            guid: "usr-23456789",
            agentId: "designer",
            status: nil,
            purpose: nil,
            avatar: nil,
            registeredAt: 0,
            lastSeen: Int64(Date().timeIntervalSince1970 * 1000) - 7200000,
            leftAt: nil,
            managed: true,
            invoke: nil,
            presence: .spawning,
            mentionWatermark: nil,
            reactionWatermark: nil,
            lastHeartbeat: nil,
            lastSessionId: nil,
            sessionMode: nil,
            jobId: nil,
            jobIdx: nil,
            isEphemeral: nil
        ))

        AgentActivityRow(agent: FrayAgent(
            guid: "usr-34567890",
            agentId: "reviewer",
            status: nil,
            purpose: nil,
            avatar: nil,
            registeredAt: 0,
            lastSeen: Int64(Date().timeIntervalSince1970 * 1000) - 86400000,
            leftAt: nil,
            managed: false,
            invoke: nil,
            presence: .offline,
            mentionWatermark: nil,
            reactionWatermark: nil,
            lastHeartbeat: nil,
            lastSessionId: nil,
            sessionMode: nil,
            jobId: nil,
            jobIdx: nil,
            isEphemeral: nil
        ))
    }
    .padding()
    .frame(width: 250)
}
