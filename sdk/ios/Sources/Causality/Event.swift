import Foundation

/// An analytics event
public struct Event: Codable {
    /// Event type (e.g., "button_tap", "screen_view")
    public var type: String

    /// Event properties as key-value pairs
    public var properties: [String: AnyCodable]?

    /// Timestamp (ISO8601, auto-populated if not provided)
    public var timestamp: String?

    public init(type: String, properties: [String: AnyCodable]? = nil, timestamp: String? = nil) {
        self.type = type
        self.properties = properties
        self.timestamp = timestamp
    }
}

/// Event builder for fluent API
public class EventBuilder {
    private var type: String
    private var properties: [String: AnyCodable] = [:]

    public init(type: String) {
        self.type = type
    }

    @discardableResult
    public func property(_ key: String, _ value: String) -> EventBuilder {
        properties[key] = AnyCodable(value)
        return self
    }

    @discardableResult
    public func property(_ key: String, _ value: Int) -> EventBuilder {
        properties[key] = AnyCodable(value)
        return self
    }

    @discardableResult
    public func property(_ key: String, _ value: Double) -> EventBuilder {
        properties[key] = AnyCodable(value)
        return self
    }

    @discardableResult
    public func property(_ key: String, _ value: Bool) -> EventBuilder {
        properties[key] = AnyCodable(value)
        return self
    }

    public func build() -> Event {
        Event(type: type, properties: properties.isEmpty ? nil : properties)
    }
}

/// Type-erased Codable wrapper for property values
public struct AnyCodable: Codable {
    public let value: Any

    public init(_ value: Any) {
        self.value = value
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let string = try? container.decode(String.self) {
            value = string
        } else if let int = try? container.decode(Int.self) {
            value = int
        } else if let double = try? container.decode(Double.self) {
            value = double
        } else if let bool = try? container.decode(Bool.self) {
            value = bool
        } else {
            throw DecodingError.typeMismatch(AnyCodable.self, DecodingError.Context(codingPath: decoder.codingPath, debugDescription: "Unsupported type"))
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch value {
        case let string as String:
            try container.encode(string)
        case let int as Int:
            try container.encode(int)
        case let double as Double:
            try container.encode(double)
        case let bool as Bool:
            try container.encode(bool)
        default:
            throw EncodingError.invalidValue(value, EncodingError.Context(codingPath: encoder.codingPath, debugDescription: "Unsupported type"))
        }
    }
}
