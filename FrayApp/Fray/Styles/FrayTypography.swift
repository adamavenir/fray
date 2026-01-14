import SwiftUI

enum FrayTypography {
    // Base sizes (+2px from system defaults for better readability)
    static let body = Font.system(size: 15)
    static let bodyMono = Font.system(size: 15, design: .monospaced)

    static let caption = Font.system(size: 13)
    static let captionMono = Font.system(size: 13, design: .monospaced)

    static let headline = Font.system(size: 15, weight: .semibold)
    static let subheadline = Font.system(size: 14)

    static let title = Font.title3
    static let title2 = Font.title2

    // Sidebar items
    static let sidebarChannel = Font.system(size: 14, weight: .semibold)  // Bold for channel name
    static let sidebarThread = Font.system(size: 14, weight: .medium)     // Medium for thread names

    static let agentName = Font.system(size: 15, weight: .medium, design: .monospaced)
    static let timestamp = Font.system(size: 13).monospacedDigit()

    static let messageBody = Font.system(size: 15)
    static let codeBlock = Font.system(size: 15, design: .monospaced)
    static let inlineCode = Font.system(size: 15, design: .monospaced)

    static let reactionCount = Font.system(size: 13).monospacedDigit()
    static let badge = Font.system(size: 11, weight: .semibold)

    // Message line height
    static let messageLineSpacing: CGFloat = 4
}
