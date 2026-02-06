package io.causality.example.compose

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import io.causality.example.compose.ui.MainScreen
import io.causality.example.compose.ui.theme.CausalityTheme

/**
 * Main activity using Jetpack Compose.
 *
 * Uses ComponentActivity with setContent to host Compose UI.
 * All UI is defined in composable functions.
 */
class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            CausalityTheme {
                MainScreen()
            }
        }
    }
}
