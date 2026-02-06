import SwiftUI
import Causality

@main
struct SwiftUIExampleApp: App {

    init() {
        // Initialize Causality SDK
        do {
            try Causality.shared.initialize(config: Config(
                apiKey: "your-api-key", // Replace with actual key
                endpoint: "http://localhost:8080", // Replace with server URL
                appId: "swiftui-example",
                debugMode: true
            ))
            print("[SwiftUIExample] Causality SDK initialized")
            print("[SwiftUIExample] Device ID: \(Causality.shared.deviceId)")
        } catch {
            print("[SwiftUIExample] Failed to initialize: \(error)")
        }
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
