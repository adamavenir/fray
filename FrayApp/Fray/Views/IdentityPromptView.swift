import SwiftUI

struct IdentityPromptView: View {
    @Environment(FrayBridge.self) private var bridge
    @Binding var isPresented: Bool
    let onComplete: (String) -> Void

    @State private var agentName: String = ""
    @State private var isSubmitting: Bool = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(spacing: FraySpacing.lg) {
            Text("Welcome to Fray")
                .font(FrayTypography.title)
                .foregroundStyle(.primary)

            Text("Enter your username to get started.")
                .font(FrayTypography.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)

            VStack(alignment: .leading, spacing: FraySpacing.xs) {
                HStack {
                    Text("@")
                        .font(FrayTypography.agentName)
                        .foregroundStyle(.secondary)
                    TextField("user", text: $agentName)
                        .textFieldStyle(.plain)
                        .font(FrayTypography.agentName)
                        .autocorrectionDisabled()
                        .onSubmit(handleSubmit)
                }
                .padding(FraySpacing.sm)
                .background {
                    RoundedRectangle(cornerRadius: FraySpacing.smallCornerRadius)
                        .fill(FrayColors.tertiaryBackground)
                }

                Text("Lowercase letters, numbers, and hyphens only")
                    .font(FrayTypography.caption)
                    .foregroundStyle(.tertiary)
            }

            if let error = errorMessage {
                Text(error)
                    .font(FrayTypography.caption)
                    .foregroundStyle(.red)
            }

            HStack(spacing: FraySpacing.md) {
                Button("Cancel") {
                    isPresented = false
                }
                .buttonStyle(.bordered)

                Button("Continue") {
                    handleSubmit()
                }
                .buttonStyle(.borderedProminent)
                .disabled(agentName.isEmpty || isSubmitting)
            }
        }
        .padding(FraySpacing.xl)
        .frame(width: 320)
    }

    private func handleSubmit() {
        guard !agentName.isEmpty else { return }

        let normalizedName = agentName.lowercased().trimmingCharacters(in: .whitespaces)
        guard isValidAgentName(normalizedName) else {
            errorMessage = "Invalid name. Use lowercase letters, numbers, and hyphens."
            return
        }

        isSubmitting = true
        errorMessage = nil

        Task {
            do {
                _ = try bridge.registerAgent(agentId: normalizedName)
                try bridge.setConfig(key: "username", value: normalizedName)
                await MainActor.run {
                    isPresented = false
                    onComplete(normalizedName)
                }
            } catch {
                await MainActor.run {
                    errorMessage = "Failed to register: \(error.localizedDescription)"
                    isSubmitting = false
                }
            }
        }
    }

    private func isValidAgentName(_ name: String) -> Bool {
        guard !name.isEmpty else { return false }
        guard let first = name.first, first.isLetter else { return false }
        let allowedChars = CharacterSet.lowercaseLetters
            .union(.decimalDigits)
            .union(CharacterSet(charactersIn: "-"))
        return name.unicodeScalars.allSatisfy { allowedChars.contains($0) }
    }
}

#Preview {
    IdentityPromptView(isPresented: .constant(true)) { name in
        print("Registered as: \(name)")
    }
    .environment(FrayBridge())
}
