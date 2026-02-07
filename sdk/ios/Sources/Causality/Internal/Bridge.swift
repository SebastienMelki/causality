import Foundation
import CausalityCore

/// Internal bridge to Go core via JSON
enum Bridge {
    static func initSDK(config: Config) throws {
        let jsonData = try JSONEncoder().encode(config)
        guard let jsonString = String(data: jsonData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode config")
        }
        print("[Causality:Bridge] Init JSON: \(jsonString)")
        let result = CAUMobileInit(jsonString)
        print("[Causality:Bridge] Init result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.initialization(result)
        }
    }

    static func track(event: Event) throws {
        let jsonData = try JSONEncoder().encode(event)
        guard let jsonString = String(data: jsonData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode event")
        }
        print("[Causality:Bridge] Track JSON: \(jsonString)")
        let result = CAUMobileTrack(jsonString)
        print("[Causality:Bridge] Track result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.tracking(result)
        }
    }

    static func trackTyped<E: CausalityEvent>(event: E) throws {
        let propsData = try JSONEncoder().encode(event)
        guard let propsJSON = String(data: propsData, encoding: .utf8) else {
            throw CausalityError.encoding("Failed to encode event properties")
        }
        let jsonString = "{\"type\":\"\(E.eventType)\",\"properties\":\(propsJSON)}"
        print("[Causality:Bridge] Track JSON: \(jsonString)")
        let result = CAUMobileTrack(jsonString)
        print("[Causality:Bridge] Track result: '\(result)'")
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
        print("[Causality:Bridge] SetUser JSON: \(jsonString)")
        let result = CAUMobileSetUser(jsonString)
        print("[Causality:Bridge] SetUser result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.identification(result)
        }
    }

    static func reset() throws {
        print("[Causality:Bridge] Reset called")
        let result = CAUMobileReset()
        print("[Causality:Bridge] Reset result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.reset(result)
        }
    }

    static func resetAll() throws {
        print("[Causality:Bridge] ResetAll called")
        let result = CAUMobileResetAll()
        print("[Causality:Bridge] ResetAll result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.reset(result)
        }
    }

    static func flush() throws {
        print("[Causality:Bridge] Flush called")
        let result = CAUMobileFlush()
        print("[Causality:Bridge] Flush result: '\(result)'")
        if !result.isEmpty {
            throw CausalityError.flush(result)
        }
    }

    static func getDeviceId() -> String {
        let result = CAUMobileGetDeviceId()
        print("[Causality:Bridge] GetDeviceId: '\(result)'")
        return result
    }

    static func isInitialized() -> Bool {
        let result = CAUMobileIsInitialized()
        print("[Causality:Bridge] IsInitialized: \(result)")
        return result
    }

    static func appDidEnterBackground() {
        print("[Causality:Bridge] AppDidEnterBackground called")
        _ = CAUMobileAppDidEnterBackground()
    }

    static func appWillEnterForeground() {
        print("[Causality:Bridge] AppWillEnterForeground called")
        _ = CAUMobileAppWillEnterForeground()
    }
}
