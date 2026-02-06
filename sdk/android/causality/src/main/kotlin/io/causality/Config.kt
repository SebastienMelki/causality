package io.causality

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class Config(
    @SerialName("api_key") val apiKey: String,
    val endpoint: String,
    @SerialName("app_id") val appId: String,
    @SerialName("batch_size") val batchSize: Int? = null,
    @SerialName("flush_interval_ms") val flushIntervalMs: Int? = null,
    @SerialName("max_queue_size") val maxQueueSize: Int? = null,
    @SerialName("session_timeout_ms") val sessionTimeoutMs: Int? = null,
    @SerialName("debug_mode") val debugMode: Boolean? = null,
    @SerialName("enable_session_tracking") val enableSessionTracking: Boolean? = null,
    @SerialName("persistent_device_id") val persistentDeviceId: Boolean? = null
)

class ConfigBuilder {
    var apiKey: String = ""
    var endpoint: String = ""
    var appId: String = ""
    var batchSize: Int? = null
    var flushIntervalMs: Int? = null
    var maxQueueSize: Int? = null
    var sessionTimeoutMs: Int? = null
    var debugMode: Boolean? = null
    var enableSessionTracking: Boolean? = null
    var persistentDeviceId: Boolean? = null

    fun build(): Config {
        require(apiKey.isNotBlank()) { "apiKey is required" }
        require(endpoint.isNotBlank()) { "endpoint is required" }
        require(appId.isNotBlank()) { "appId is required" }

        return Config(
            apiKey = apiKey,
            endpoint = endpoint,
            appId = appId,
            batchSize = batchSize,
            flushIntervalMs = flushIntervalMs,
            maxQueueSize = maxQueueSize,
            sessionTimeoutMs = sessionTimeoutMs,
            debugMode = debugMode,
            enableSessionTracking = enableSessionTracking,
            persistentDeviceId = persistentDeviceId
        )
    }
}

inline fun config(block: ConfigBuilder.() -> Unit): Config {
    return ConfigBuilder().apply(block).build()
}
