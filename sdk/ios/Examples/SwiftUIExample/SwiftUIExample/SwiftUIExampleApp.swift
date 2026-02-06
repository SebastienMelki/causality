import SwiftUI
import Causality

@main
struct SwiftUIExampleApp: App {

    init() {
        // Initialize Causality SDK
        do {
            try Causality.shared.initialize(config: Config(
                apiKey: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
                endpoint: "http://localhost:8080",
                appId: "dev-app",
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
