package io.causality.example.views

import android.app.Application
import android.util.Log
import io.causality.Causality

/**
 * Application class demonstrating Causality SDK initialization.
 *
 * The SDK should be initialized once in Application.onCreate() before
 * any tracking calls are made. This ensures the SDK is ready as soon
 * as the first Activity starts.
 */
class ExampleApplication : Application() {

    override fun onCreate() {
        super.onCreate()

        // Initialize Causality SDK with DSL configuration.
        // Use 10.0.2.2 to reach host localhost from Android emulator.
        Causality.initialize(this) {
            apiKey = "example-api-key-replace-me"
            endpoint = "http://10.0.2.2:8080"
            appId = "views-example"

            // Optional configuration
            batchSize = 10
            flushIntervalMs = 30_000
            debugMode = true
            enableSessionTracking = true
        }

        Log.d(TAG, "Causality SDK initialized, deviceId=${Causality.deviceId}")
    }

    companion object {
        private const val TAG = "CausalityExample"
    }
}
