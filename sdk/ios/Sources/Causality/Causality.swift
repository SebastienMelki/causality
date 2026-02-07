import Foundation
import CausalityCore
#if canImport(UIKit)
import UIKit
#endif

/// Main entry point for the Causality analytics SDK
@MainActor
public final class Causality {
    /// Shared singleton instance
    public static let shared = Causality()

    private var isInitialized = false

    private init() {
        // Register for lifecycle notifications
        #if canImport(UIKit)
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(appDidEnterBackground),
            name: UIApplication.didEnterBackgroundNotification,
            object: nil
        )
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(appWillEnterForeground),
            name: UIApplication.willEnterForegroundNotification,
            object: nil
        )
        #endif
    }

    /// Initialize the SDK with configuration
    /// - Parameter config: SDK configuration
    /// - Throws: CausalityError if initialization fails
    public func initialize(config: Config) throws {
        // Set platform context first
        Platform.setPlatformContext()

        // Initialize Go core
        try Bridge.initSDK(config: config)
        isInitialized = true
    }

    /// Track a freeform event
    /// - Parameter event: The event to track
    /// - Note: This method is non-blocking. Events are queued for batch sending.
    public func track(_ event: Event) {
        guard isInitialized else {
            #if DEBUG
            print("[Causality] Warning: SDK not initialized, event dropped")
            #endif
            return
        }

        Task.detached(priority: .utility) {
            do {
                try Bridge.track(event: event)
            } catch {
                #if DEBUG
                print("[Causality] Track error: \(error)")
                #endif
            }
        }
    }

    /// Track a typed event with compile-time safety
    /// - Parameter event: A CausalityEvent (e.g., ScreenView, ButtonTap, PurchaseComplete)
    /// - Note: This method is non-blocking. Events are queued for batch sending.
    public func track<E: CausalityEvent>(_ event: E) {
        guard isInitialized else {
            #if DEBUG
            print("[Causality] Warning: SDK not initialized, event dropped")
            #endif
            return
        }

        Task.detached(priority: .utility) {
            do {
                try Bridge.trackTyped(event: event)
            } catch {
                #if DEBUG
                print("[Causality] Track error: \(error)")
                #endif
            }
        }
    }

    /// Track an event using a builder
    /// - Parameter type: Event type
    /// - Returns: EventBuilder for fluent property addition
    public func track(type: String) -> EventBuilder {
        EventBuilder(type: type)
    }

    /// Set user identity
    /// - Parameters:
    ///   - userId: Unique user identifier
    ///   - traits: Optional user properties
    ///   - aliases: Optional alias identifiers
    public func identify(userId: String, traits: [String: AnyCodable]? = nil, aliases: [String]? = nil) throws {
        guard isInitialized else {
            throw CausalityError.notInitialized
        }
        try Bridge.setUser(userId: userId, traits: traits, aliases: aliases)
    }

    /// Clear user identity (soft reset - keeps device ID)
    public func reset() throws {
        guard isInitialized else {
            throw CausalityError.notInitialized
        }
        try Bridge.reset()
    }

    /// Full reset - clears user identity and regenerates device ID
    public func resetAll() throws {
        guard isInitialized else {
            throw CausalityError.notInitialized
        }
        try Bridge.resetAll()
    }

    /// Force flush all queued events
    public func flush() async throws {
        guard isInitialized else {
            throw CausalityError.notInitialized
        }

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            DispatchQueue.global(qos: .utility).async {
                do {
                    try Bridge.flush()
                    continuation.resume()
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Get the device identifier
    /// - Returns: Device ID, or empty string if not initialized
    public var deviceId: String {
        Bridge.getDeviceId()
    }

    /// Check if SDK is initialized
    public var initialized: Bool {
        Bridge.isInitialized()
    }

    // MARK: - Lifecycle

    @objc private func appDidEnterBackground() {
        Bridge.appDidEnterBackground()
    }

    @objc private func appWillEnterForeground() {
        Bridge.appWillEnterForeground()
    }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }
}

// MARK: - Convenience extensions

public extension Causality {
    /// Track a screen view
    func trackScreenView(name: String, screenClass: String? = nil, previousScreen: String? = nil) {
        track(ScreenView(screenName: name, screenClass: screenClass, previousScreen: previousScreen))
    }

    /// Track a button tap
    func trackButtonTap(id: String, text: String? = nil, screenName: String? = nil) {
        track(ButtonTap(buttonId: id, buttonText: text, screenName: screenName))
    }
}
