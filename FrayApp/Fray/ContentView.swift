import SwiftUI

struct ContentView: View {
    @Environment(FrayBridge.self) private var bridge
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.colorScheme) private var colorScheme
    @State private var selectedThread: FrayThread?
    @State private var columnVisibility: NavigationSplitViewVisibility = .all
    @State private var showActivityPanel = false
    @State private var showCommandPalette = false
    @State private var agentsVM: AgentsViewModel?
    @State private var allThreads: [FrayThread] = []
    @FocusState private var isInputFocused: Bool

    @SceneStorage("sidebarWidth") private var sidebarWidth: Double = 280
    @SceneStorage("selectedThreadId") private var selectedThreadId: String = ""
    @SceneStorage("activityPanelVisible") private var activityPanelVisible: Bool = false

    var body: some View {
        ZStack {
            NavigationSplitView(columnVisibility: $columnVisibility) {
                SidebarView(selectedThread: $selectedThread)
                    .navigationSplitViewColumnWidth(min: 200, ideal: FraySpacing.sidebarWidth)
            } content: {
                VStack(spacing: 0) {
                    if selectedThread != nil {
                        ThreadBreadcrumb(
                            thread: selectedThread,
                            allThreads: allThreads,
                            onNavigate: { selectedThread = $0 }
                        )
                    }

                    if let thread = selectedThread {
                        MessageListView(thread: thread)
                    } else {
                        RoomView()
                    }
                }
            } detail: {
                if showActivityPanel, let vm = agentsVM {
                    ActivityPanelView()
                        .environment(vm)
                        .navigationSplitViewColumnWidth(
                            min: 150,
                            ideal: FraySpacing.activityPanelWidth
                        )
                }
            }
            .task {
                await connectToProject()
                agentsVM = AgentsViewModel(bridge: bridge)
                await loadThreads()
                restoreState()
            }
            .onChange(of: selectedThread?.guid) { _, newValue in
                selectedThreadId = newValue ?? ""
            }
            .onChange(of: showActivityPanel) { _, newValue in
                activityPanelVisible = newValue
            }
            .toolbar {
                ToolbarItem(placement: .navigation) {
                    Button(action: {
                        withAnimation {
                            columnVisibility = columnVisibility == .all ? .detailOnly : .all
                        }
                    }) {
                        Image(systemName: "sidebar.left")
                    }
                    .help("Toggle Sidebar (⌘0)")
                    .keyboardShortcut("0", modifiers: [.command])
                }

                ToolbarItem(placement: .navigation) {
                    Button(action: { showCommandPalette = true }) {
                        Image(systemName: "magnifyingglass")
                    }
                    .help("Command Palette (⌘K)")
                    .keyboardShortcut("k", modifiers: [.command])
                }

                ToolbarItem(placement: .primaryAction) {
                    Button(action: { showActivityPanel.toggle() }) {
                        Image(systemName: "sidebar.right")
                    }
                    .help("Toggle Activity Panel (⌘I)")
                    .keyboardShortcut("i", modifiers: [.command])
                }
            }

            if showCommandPalette {
                FrayColors.modalOverlay.resolve(for: colorScheme)
                    .ignoresSafeArea()
                    .onTapGesture { showCommandPalette = false }

                CommandPalette { result in
                    handleCommandResult(result)
                }
            }
        }
    }

    private func handleCommandResult(_ result: CommandResult) {
        showCommandPalette = false

        switch result.action {
        case .openThread(let guid):
            if let thread = allThreads.first(where: { $0.guid == guid }) {
                selectedThread = thread
            }
        case .viewAgent:
            break
        case .focusInput:
            isInputFocused = true
        case .toggleSidebar:
            withAnimation {
                columnVisibility = columnVisibility == .all ? .detailOnly : .all
            }
        case .toggleActivity:
            showActivityPanel.toggle()
        case .openRoom:
            selectedThread = nil
        }
    }

    private func loadThreads() async {
        do {
            allThreads = try bridge.getThreads()
        } catch {
            print("Failed to load threads: \(error)")
        }
    }

    private func connectToProject() async {
        // Try multiple discovery paths since app launch directory varies
        let searchPaths = [
            FileManager.default.currentDirectoryPath,
            ProcessInfo.processInfo.environment["FRAY_PROJECT_PATH"],
            FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("dev/fray").path,
            "/Users/adam/dev/fray"  // Fallback for development
        ].compactMap { $0 }

        for startPath in searchPaths {
            if let projectPath = FrayBridge.discoverProject(from: startPath) {
                do {
                    try bridge.connect(projectPath: projectPath)
                    print("Connected to fray project at: \(projectPath)")
                    return
                } catch {
                    print("Failed to connect to \(projectPath): \(error)")
                }
            }
        }

        print("No fray project found. Searched: \(searchPaths)")
    }

    private func restoreState() {
        showActivityPanel = activityPanelVisible

        if !selectedThreadId.isEmpty,
           let thread = allThreads.first(where: { $0.guid == selectedThreadId }) {
            selectedThread = thread
        }
    }
}

#Preview {
    ContentView()
        .environment(FrayBridge())
        .frame(width: 1000, height: 600)
}
