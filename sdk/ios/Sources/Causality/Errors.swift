import Foundation

/// Errors thrown by the Causality SDK
public enum CausalityError: Error, LocalizedError {
    case notInitialized
    case encoding(String)
    case initialization(String)
    case tracking(String)
    case identification(String)
    case reset(String)
    case flush(String)

    public var errorDescription: String? {
        switch self {
        case .notInitialized:
            return "Causality SDK is not initialized. Call initialize() first."
        case .encoding(let message):
            return "Encoding error: \(message)"
        case .initialization(let message):
            return "Initialization error: \(message)"
        case .tracking(let message):
            return "Tracking error: \(message)"
        case .identification(let message):
            return "Identification error: \(message)"
        case .reset(let message):
            return "Reset error: \(message)"
        case .flush(let message):
            return "Flush error: \(message)"
        }
    }
}
