package io.causality.example.compose

import android.app.Application
import android.util.Log
import io.causality.Causality

/**
 * Application class that initializes the Causality SDK.
 *
 * SDK initialization must happen in Application.onCreate() before
 * any Activity starts tracking events.
 */
class ExampleApplication : Application() {

    override fun onCreate() {
        super.onCreate()

        // Initialize Causality SDK using the DSL builder.
        // In production, these values should come from BuildConfig or a config file.
        Causality.initialize(this) {
            apiKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
            endpoint = "http://10.0.2.2:8080" // Android emulator localhost
            appId = "dev-app"
            debugMode = true
            enableSessionTracking = true
        }

        Log.d("CausalityExample", "Causality SDK initialized, device ID: ${Causality.deviceId}")
    }
}
