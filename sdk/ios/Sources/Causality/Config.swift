import Foundation

/// Configuration for the Causality SDK
public struct Config: Codable {
    /// API key for authentication (required)
    public var apiKey: String

    /// Server endpoint URL (required)
    public var endpoint: String

    /// Application identifier (required)
    public var appId: String

    /// Maximum events per batch (optional, default: 30)
    public var batchSize: Int?

    /// Flush interval in milliseconds (optional, default: 30000)
    public var flushIntervalMs: Int?

    /// Maximum queue size (optional, default: 10000)
    public var maxQueueSize: Int?

    /// Session timeout in milliseconds (optional, default: 30000)
    public var sessionTimeoutMs: Int?

    /// Enable debug logging (optional, default: false)
    public var debugMode: Bool?

    /// Enable automatic session tracking (optional, default: true)
    public var enableSessionTracking: Bool?

    /// Use persistent device ID across reinstalls (optional, default: false)
    public var persistentDeviceId: Bool?

    public init(
        apiKey: String,
        endpoint: String,
        appId: String,
        batchSize: Int? = nil,
        flushIntervalMs: Int? = nil,
        maxQueueSize: Int? = nil,
        sessionTimeoutMs: Int? = nil,
        debugMode: Bool? = nil,
        enableSessionTracking: Bool? = nil,
        persistentDeviceId: Bool? = nil
    ) {
        self.apiKey = apiKey
        self.endpoint = endpoint
        self.appId = appId
        self.batchSize = batchSize
        self.flushIntervalMs = flushIntervalMs
        self.maxQueueSize = maxQueueSize
        self.sessionTimeoutMs = sessionTimeoutMs
        self.debugMode = debugMode
        self.enableSessionTracking = enableSessionTracking
        self.persistentDeviceId = persistentDeviceId
    }

    private enum CodingKeys: String, CodingKey {
        case apiKey = "api_key"
        case endpoint
        case appId = "app_id"
        case batchSize = "batch_size"
        case flushIntervalMs = "flush_interval_ms"
        case maxQueueSize = "max_queue_size"
        case sessionTimeoutMs = "session_timeout_ms"
        case debugMode = "debug_mode"
        case enableSessionTracking = "enable_session_tracking"
        case persistentDeviceId = "persistent_device_id"
    }
}
