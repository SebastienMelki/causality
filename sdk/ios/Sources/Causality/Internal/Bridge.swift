import Foundation
import CausalityCore

/// Internal bridge to Go core via JSON
enum Bridge {
    static func initSDK(config: Config) throws {
        let jsonData = try JSONEncoder().encode(config)
        guard let jsonString = String(data: jsonData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode config")
        }
        let result = MobileInit(jsonString)
        if !result.isEmpty {
            throw CausalityError.initialization(result)
        }
    }

    static func track(event: Event) throws {
        let jsonData = try JSONEncoder().encode(event)
        guard let jsonString = String(data: jsonData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode event")
        }
        let result = MobileTrack(jsonString)
        if !result.isEmpty {
            throw CausalityError.tracking(result)
        }
    }

    static func setUser(userId: String, traits: [String: AnyCodable]?, aliases: [String]?) throws {
        struct UserPayload: Codable {
            let userId: String
            let traits: [String: AnyCodable]?
            let aliases: [String]?

            enum CodingKeys: String, CodingKey {
                case userId = "user_id"
                case traits
                case aliases
            }
        }

        let payload = UserPayload(userId: userId, traits: traits, aliases: aliases)
        let jsonData = try JSONEncoder().encode(payload)
        guard let jsonString = String(data: jsonData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode user")
        }
        let result = MobileSetUser(jsonString)
        if !result.isEmpty {
            throw CausalityError.identification(result)
        }
    }

    static func reset() throws {
        let result = MobileReset()
        if !result.isEmpty {
            throw CausalityError.reset(result)
        }
    }

    static func resetAll() throws {
        let result = MobileResetAll()
        if !result.isEmpty {
            throw CausalityError.reset(result)
        }
    }

    static func flush() throws {
        let result = MobileFlush()
        if !result.isEmpty {
            throw CausalityError.flush(result)
        }
    }

    static func getDeviceId() -> String {
        MobileGetDeviceId()
    }

    static func isInitialized() -> Bool {
        MobileIsInitialized()
    }

    static func appDidEnterBackground() {
        _ = MobileAppDidEnterBackground()
    }

    static func appWillEnterForeground() {
        _ = MobileAppWillEnterForeground()
    }
}
