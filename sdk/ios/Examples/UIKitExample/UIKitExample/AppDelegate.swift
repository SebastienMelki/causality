import UIKit
import Causality

@main
class AppDelegate: UIResponder, UIApplicationDelegate {

    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {

        // Initialize Causality SDK
        do {
            try Causality.shared.initialize(config: Config(
                apiKey: "your-api-key", // Replace with actual key
                endpoint: "http://localhost:8080", // Replace with server URL
                appId: "uikit-example",
                debugMode: true
            ))
            print("[UIKitExample] Causality SDK initialized")
            print("[UIKitExample] Device ID: \(Causality.shared.deviceId)")
        } catch {
            print("[UIKitExample] Failed to initialize: \(error)")
        }

        return true
    }

    // MARK: UISceneSession Lifecycle

    func application(
        _ application: UIApplication,
        configurationForConnecting connectingSceneSession: UISceneSession,
        options: UIScene.ConnectionOptions
    ) -> UISceneConfiguration {
        return UISceneConfiguration(
            name: "Default Configuration",
            sessionRole: connectingSceneSession.role
        )
    }
}
