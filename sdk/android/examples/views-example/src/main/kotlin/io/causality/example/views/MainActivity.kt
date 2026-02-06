package io.causality.example.views

import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import io.causality.Causality
import io.causality.event
import io.causality.example.views.databinding.ActivityMainBinding
import kotlinx.coroutines.launch

/**
 * Main activity demonstrating the Causality SDK features.
 *
 * Shows how to:
 * - Track events using the DSL builder
 * - Track events using the Event DSL
 * - Track screen views
 * - Identify users with traits
 * - Flush queued events
 * - Reset user identity
 */
class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private var eventCount = 0

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        setupUI()

        // Track this screen view
        Causality.trackScreenView("main_activity")
    }

    private fun setupUI() {
        // Display device ID
        binding.textDeviceId.text = getString(R.string.device_id_format, Causality.deviceId)

        // Track Event button -- demonstrates basic event tracking with DSL
        binding.buttonTrackEvent.setOnClickListener {
            eventCount++
            Causality.track("button_tap") {
                property("button_name", "track_event")
                property("tap_count", eventCount)
                property("screen", "main_activity")
            }
            showStatus("Event tracked (#$eventCount)")
        }

        // Track Custom Event button -- demonstrates the Event DSL
        binding.buttonTrackCustom.setOnClickListener {
            val customEvent = event("purchase_complete") {
                property("product_id", "SKU-12345")
                property("price", 29.99)
                property("currency", "USD")
                property("quantity", 1)
            }
            Causality.track(customEvent)
            showStatus("Custom event tracked: purchase_complete")
        }

        // Identify User button -- demonstrates user identification
        binding.buttonIdentify.setOnClickListener {
            Causality.identify(
                userId = "user-42",
                traits = mapOf(
                    "name" to "Jane Doe",
                    "email" to "jane@example.com",
                    "plan" to "premium"
                )
            )
            binding.textUserId.text = getString(R.string.user_id_format, "user-42")
            showStatus("User identified: user-42")
        }

        // Flush Events button -- demonstrates manual flush (suspend function)
        binding.buttonFlush.setOnClickListener {
            lifecycleScope.launch {
                try {
                    Causality.flush()
                    showStatus("Events flushed successfully")
                } catch (e: Exception) {
                    showStatus("Flush error: ${e.message}")
                }
            }
        }

        // Reset button -- demonstrates identity reset
        binding.buttonReset.setOnClickListener {
            Causality.reset()
            binding.textUserId.text = getString(R.string.user_id_format, "anonymous")
            eventCount = 0
            showStatus("Identity reset")
        }

        // Track Screen View button -- demonstrates convenience method
        binding.buttonTrackScreen.setOnClickListener {
            Causality.trackScreenView(
                "settings_screen",
                properties = mapOf(
                    "source" to "main_activity",
                    "tab_index" to 2
                )
            )
            showStatus("Screen view tracked: settings_screen")
        }
    }

    private fun showStatus(message: String) {
        binding.textStatus.text = message
        Toast.makeText(this, message, Toast.LENGTH_SHORT).show()
    }
}
