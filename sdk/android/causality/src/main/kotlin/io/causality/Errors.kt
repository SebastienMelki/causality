package io.causality

sealed class CausalityException(message: String, cause: Throwable? = null) : Exception(message, cause) {
    class NotInitialized : CausalityException("Causality SDK is not initialized. Call initialize() first.")
    class Initialization(message: String) : CausalityException("Initialization error: $message")
    class Tracking(message: String) : CausalityException("Tracking error: $message")
    class Identification(message: String) : CausalityException("Identification error: $message")
    class Reset(message: String) : CausalityException("Reset error: $message")
    class Flush(message: String) : CausalityException("Flush error: $message")
}
