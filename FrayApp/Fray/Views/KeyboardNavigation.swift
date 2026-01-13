import SwiftUI

struct KeyboardNavigationModifier: ViewModifier {
    @Binding var selectedMessageIndex: Int?
    @Binding var showCommandPalette: Bool
    let messageCount: Int
    let onReply: () -> Void
    let onEscape: () -> Void

    func body(content: Content) -> some View {
        content
            .onKeyPress(.downArrow) {
                moveSelection(down: true)
                return .handled
            }
            .onKeyPress(.upArrow) {
                moveSelection(down: false)
                return .handled
            }
            .onKeyPress(.escape) {
                onEscape()
                return .handled
            }
    }

    private func moveSelection(down: Bool) {
        if let current = selectedMessageIndex {
            if down {
                selectedMessageIndex = min(messageCount - 1, current + 1)
            } else {
                selectedMessageIndex = max(0, current - 1)
            }
        } else if messageCount > 0 {
            selectedMessageIndex = down ? 0 : messageCount - 1
        }
    }
}

extension View {
    func keyboardNavigation(
        selectedIndex: Binding<Int?>,
        showCommandPalette: Binding<Bool>,
        messageCount: Int,
        onReply: @escaping () -> Void,
        onEscape: @escaping () -> Void
    ) -> some View {
        modifier(KeyboardNavigationModifier(
            selectedMessageIndex: selectedIndex,
            showCommandPalette: showCommandPalette,
            messageCount: messageCount,
            onReply: onReply,
            onEscape: onEscape
        ))
    }
}

struct GlobalKeyboardShortcuts: ViewModifier {
    @Binding var showSidebar: Bool
    @Binding var showActivity: Bool
    @Binding var showCommandPalette: Bool
    let onFocusInput: () -> Void

    func body(content: Content) -> some View {
        content
    }
}

extension View {
    func globalKeyboardShortcuts(
        showSidebar: Binding<Bool>,
        showActivity: Binding<Bool>,
        showCommandPalette: Binding<Bool>,
        onFocusInput: @escaping () -> Void
    ) -> some View {
        modifier(GlobalKeyboardShortcuts(
            showSidebar: showSidebar,
            showActivity: showActivity,
            showCommandPalette: showCommandPalette,
            onFocusInput: onFocusInput
        ))
    }
}
