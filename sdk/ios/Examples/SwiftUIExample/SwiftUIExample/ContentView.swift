import SwiftUI
import Causality

struct ContentView: View {
    @State private var showToast = false
    @State private var toastMessage = ""
    @State private var isLoading = false

    var body: some View {
        NavigationView {
            VStack(spacing: 20) {
                // Device ID
                Text("Device ID:")
                    .font(.caption)
                    .foregroundColor(.secondary)
                Text(Causality.shared.deviceId)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundColor(.secondary)
                    .padding(.bottom, 20)

                // Track Event Button
                Button(action: trackEvent) {
                    HStack {
                        Image(systemName: "bolt.fill")
                        Text("Track Event")
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.blue)
                    .foregroundColor(.white)
                    .cornerRadius(10)
                }

                // Track Purchase Button
                Button(action: trackPurchase) {
                    HStack {
                        Image(systemName: "cart.fill")
                        Text("Track Purchase")
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.green)
                    .foregroundColor(.white)
                    .cornerRadius(10)
                }

                // Identify User Button
                Button(action: identifyUser) {
                    HStack {
                        Image(systemName: "person.fill")
                        Text("Identify User")
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.purple)
                    .foregroundColor(.white)
                    .cornerRadius(10)
                }

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
                    .padding()
                    .background(Color.orange)
                    .foregroundColor(.white)
                    .cornerRadius(10)
                }
                .disabled(isLoading)

                // Reset Button
                Button(action: resetUser) {
                    HStack {
                        Image(systemName: "arrow.counterclockwise")
                        Text("Reset User")
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.red)
                    .foregroundColor(.white)
                    .cornerRadius(10)
                }

                Spacer()
            }
            .padding()
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
        Causality.shared.trackScreenView(name: "content_view")
        print("[SwiftUIExample] Screen view tracked: content_view")
    }

    private func trackEvent() {
        let event = EventBuilder(type: "button_tap")
            .property("button_name", "track_event")
            .property("screen", "content_view")
            .build()

        Causality.shared.track(event)
        print("[SwiftUIExample] Event tracked: button_tap")
        showToast("Event tracked!")
    }

    private func trackPurchase() {
        let event = EventBuilder(type: "purchase")
            .property("product_id", "pro-subscription")
            .property("price", 9.99)
            .property("currency", "USD")
            .property("quantity", 1)
            .build()

        Causality.shared.track(event)
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
