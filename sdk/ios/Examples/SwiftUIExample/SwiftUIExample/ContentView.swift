import SwiftUI
import Causality

struct ContentView: View {
    @State private var showToast = false
    @State private var toastMessage = ""
    @State private var isLoading = false

    var body: some View {
        NavigationView {
            ScrollView {
                VStack(spacing: 16) {
                    // Device ID
                    Text("Device ID:")
                        .font(.caption)
                        .foregroundColor(.secondary)
                    Text(Causality.shared.deviceId)
                        .font(.system(.caption, design: .monospaced))
                        .foregroundColor(.secondary)
                        .padding(.bottom, 8)

                    // Track Event Button
                    Button(action: trackEvent) {
                        Label("Track Event", systemImage: "bolt.fill")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)

                    // Track Purchase Button
                    Button(action: trackPurchase) {
                        Label("Track Purchase", systemImage: "cart.fill")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.green)

                    // Identify User Button
                    Button(action: identifyUser) {
                        Label("Identify User", systemImage: "person.fill")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.purple)

                    // Flush Button
                    Button(action: flushEvents) {
                        HStack {
                            if isLoading {
                                ProgressView()
                                    .progressViewStyle(CircularProgressViewStyle(tint: .white))
                            } else {
                                Image(systemName: "arrow.up.circle.fill")
                            }
                            Text("Flush Events")
                        }
                        .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.orange)
                    .disabled(isLoading)

                    // Reset Button
                    Button(action: resetUser) {
                        Label("Reset User", systemImage: "arrow.counterclockwise")
                            .frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(.red)
                }
                .padding()
            }
            .navigationTitle("Causality SwiftUI")
            .onAppear {
                trackScreenView()
            }
            .overlay(toastOverlay)
        }
    }

    // MARK: - Toast Overlay

    private var toastOverlay: some View {
        Group {
            if showToast {
                VStack {
                    Spacer()
                    Text(toastMessage)
                        .padding()
                        .background(Color.black.opacity(0.8))
                        .foregroundColor(.white)
                        .cornerRadius(10)
                        .padding(.bottom, 50)
                }
                .transition(.opacity)
                .animation(.easeInOut, value: showToast)
            }
        }
    }

    // MARK: - SDK Integration

    private func showToast(_ message: String) {
        toastMessage = message
        withAnimation {
            showToast = true
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            withAnimation {
                showToast = false
            }
        }
    }

    private func trackScreenView() {
        Causality.shared.track(ScreenView(screenName: "content_view"))
        print("[SwiftUIExample] Screen view tracked: content_view")
    }

    private func trackEvent() {
        Causality.shared.track(ButtonTap(buttonId: "track_event", screenName: "content_view"))
        print("[SwiftUIExample] Event tracked: button_tap")
        showToast("Event tracked!")
    }

    private func trackPurchase() {
        Causality.shared.track(PurchaseComplete(
            orderId: "order-\(UUID().uuidString.prefix(8))",
            totalCents: 999,
            currency: "USD"
        ))
        print("[SwiftUIExample] Event tracked: purchase")
        showToast("Purchase tracked!")
    }

    private func identifyUser() {
        do {
            try Causality.shared.identify(
                userId: "swiftui-user-456",
                traits: [
                    "name": AnyCodable("Jane Smith"),
                    "email": AnyCodable("jane@example.com"),
                    "premium": AnyCodable(true)
                ]
            )
            print("[SwiftUIExample] User identified")
            showToast("User identified!")
        } catch {
            print("[SwiftUIExample] Identify error: \(error)")
            showToast("Error: \(error.localizedDescription)")
        }
    }

    private func flushEvents() {
        isLoading = true
        Task {
            do {
                try await Causality.shared.flush()
                print("[SwiftUIExample] Events flushed")
                await MainActor.run {
                    isLoading = false
                    showToast("Events flushed!")
                }
            } catch {
                print("[SwiftUIExample] Flush error: \(error)")
                await MainActor.run {
                    isLoading = false
                    showToast("Error: \(error.localizedDescription)")
                }
            }
        }
    }

    private func resetUser() {
        do {
            try Causality.shared.reset()
            print("[SwiftUIExample] User reset")
            showToast("User reset!")
        } catch {
            print("[SwiftUIExample] Reset error: \(error)")
            showToast("Error: \(error.localizedDescription)")
        }
    }
}

#Preview {
    ContentView()
}
