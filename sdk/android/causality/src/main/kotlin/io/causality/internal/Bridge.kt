package io.causality.internal

import io.causality.*
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import mobile.Mobile

internal object Bridge {
    private val json = Json {
        encodeDefaults = false
        ignoreUnknownKeys = true
    }

    fun initSDK(config: Config) {
        val configJson = json.encodeToString(config)
        val result = Mobile.init(configJson)
        if (result.isNotEmpty()) {
            throw CausalityException.Initialization(result)
        }
    }

    fun track(event: Event) {
        val eventJson = json.encodeToString(event)
        val result = Mobile.track(eventJson)
        if (result.isNotEmpty()) {
            throw CausalityException.Tracking(result)
        }
    }

    @Serializable
    private data class UserPayload(
        @kotlinx.serialization.SerialName("user_id") val userId: String,
        val traits: Map<String, String>? = null,
        val aliases: List<String>? = null
    )

    fun setUser(userId: String, traits: Map<String, String>?, aliases: List<String>?) {
        val payload = UserPayload(userId, traits, aliases)
        val payloadJson = json.encodeToString(payload)
        val result = Mobile.setUser(payloadJson)
        if (result.isNotEmpty()) {
            throw CausalityException.Identification(result)
        }
    }

    fun reset() {
        val result = Mobile.reset()
        if (result.isNotEmpty()) {
            throw CausalityException.Reset(result)
        }
    }

    fun resetAll() {
        val result = Mobile.resetAll()
        if (result.isNotEmpty()) {
            throw CausalityException.Reset(result)
        }
    }

    fun flush() {
        val result = Mobile.flush()
        if (result.isNotEmpty()) {
            throw CausalityException.Flush(result)
        }
    }

    fun getDeviceId(): String = Mobile.getDeviceId()

    fun isInitialized(): Boolean = Mobile.isInitialized()

    fun appDidEnterBackground() {
        Mobile.appDidEnterBackground()
    }

    fun appWillEnterForeground() {
        Mobile.appWillEnterForeground()
    }
}
