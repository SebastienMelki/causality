package io.causality

import android.app.Application
import android.content.Context
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
import io.causality.internal.Bridge
import io.causality.internal.Platform
import kotlinx.coroutines.*

/**
 * Main entry point for the Causality analytics SDK.
 *
 * Usage:
 * ```kotlin
 * // Initialize in Application.onCreate()
 * Causality.initialize(this) {
 *     apiKey = "your-api-key"
 *     endpoint = "https://your-server.com"
 *     appId = "your-app-id"
 * }
 *
 * // Track events
 * Causality.track("button_tap") {
 *     property("button_name", "checkout")
 * }
 *
 * // Or using the Event DSL
 * Causality.track(event("purchase") {
 *     property("product_id", "abc123")
 *     property("price", 29.99)
 * })
 * ```
 */
object Causality : DefaultLifecycleObserver {
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private var initialized = false

    /**
     * Initialize the SDK with configuration using DSL builder.
     *
     * @param context Application context
     * @param block Configuration builder
     */
    fun initialize(context: Context, block: ConfigBuilder.() -> Unit) {
        initialize(context, config(block))
    }

    /**
     * Initialize the SDK with configuration.
     *
     * @param context Application context
     * @param config SDK configuration
     */
    fun initialize(context: Context, config: Config) {
        // Set platform context first
        Platform.collectAndSetContext(context.applicationContext)

        // Initialize Go core
        Bridge.initSDK(config)
        initialized = true

        // Register lifecycle observer
        ProcessLifecycleOwner.get().lifecycle.addObserver(this)
    }

    /**
     * Track an event using DSL builder.
     *
     * @param type Event type
     * @param block Event builder
     */
    fun track(type: String, block: EventBuilder.() -> Unit = {}) {
        track(event(type, block))
    }

    /**
     * Track an event.
     *
     * @param event The event to track
     */
    fun track(event: Event) {
        if (!initialized) {
            if (BuildConfig.DEBUG) {
                android.util.Log.w("Causality", "SDK not initialized, event dropped")
            }
            return
        }

        scope.launch {
            try {
                Bridge.track(event)
            } catch (e: Exception) {
                if (BuildConfig.DEBUG) {
                    android.util.Log.e("Causality", "Track error", e)
                }
            }
        }
    }

    /**
     * Set user identity.
     *
     * @param userId Unique user identifier
     * @param traits Optional user properties
     * @param aliases Optional alias identifiers
     */
    fun identify(userId: String, traits: Map<String, String>? = null, aliases: List<String>? = null) {
        if (!initialized) throw CausalityException.NotInitialized()
        Bridge.setUser(userId, traits, aliases)
    }

    /**
     * Clear user identity (soft reset - keeps device ID).
     */
    fun reset() {
        if (!initialized) throw CausalityException.NotInitialized()
        Bridge.reset()
    }

    /**
     * Full reset - clears user identity and regenerates device ID.
     */
    fun resetAll() {
        if (!initialized) throw CausalityException.NotInitialized()
        Bridge.resetAll()
    }

    /**
     * Force flush all queued events.
     */
    suspend fun flush() {
        if (!initialized) throw CausalityException.NotInitialized()
        withContext(Dispatchers.IO) {
            Bridge.flush()
        }
    }

    /**
     * Get the device identifier.
     */
    val deviceId: String
        get() = Bridge.getDeviceId()

    /**
     * Check if SDK is initialized.
     */
    val isInitialized: Boolean
        get() = Bridge.isInitialized()

    // Lifecycle callbacks
    override fun onStop(owner: LifecycleOwner) {
        Bridge.appDidEnterBackground()
    }

    override fun onStart(owner: LifecycleOwner) {
        Bridge.appWillEnterForeground()
    }

    // Convenience methods

    /**
     * Track a screen view.
     */
    fun trackScreenView(name: String, properties: Map<String, Any>? = null) {
        track("screen_view") {
            property("screen_name", name)
            properties?.forEach { (k, v) ->
                when (v) {
                    is String -> property(k, v)
                    is Int -> property(k, v)
                    is Long -> property(k, v)
                    is Double -> property(k, v)
                    is Boolean -> property(k, v)
                }
            }
        }
    }

    /**
     * Track a button tap.
     */
    fun trackButtonTap(name: String, properties: Map<String, Any>? = null) {
        track("button_tap") {
            property("button_name", name)
            properties?.forEach { (k, v) ->
                when (v) {
                    is String -> property(k, v)
                    is Int -> property(k, v)
                    is Long -> property(k, v)
                    is Double -> property(k, v)
                    is Boolean -> property(k, v)
                }
            }
        }
    }
}
