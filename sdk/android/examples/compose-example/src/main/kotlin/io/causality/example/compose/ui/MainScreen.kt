package io.causality.example.compose.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.causality.Causality
import io.causality.event
import kotlinx.coroutines.launch

/**
 * Main screen demonstrating Causality SDK integration with Jetpack Compose.
 *
 * Demonstrates:
 * - Screen view tracking via LaunchedEffect
 * - Event tracking via button taps
 * - User identification
 * - Event flushing with loading state
 * - SDK reset
 * - Top-level event DSL usage
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen() {
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    var isFlushing by remember { mutableStateOf(false) }
    var eventCount by remember { mutableIntStateOf(0) }

    // Track screen view when this composable enters composition
    LaunchedEffect(Unit) {
        Causality.trackScreenView("main_screen")
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Causality Compose") },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer,
                    titleContentColor = MaterialTheme.colorScheme.onPrimaryContainer
                )
            )
        },
        snackbarHost = { SnackbarHost(hostState = snackbarHostState) }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            // Status section
            Text(
                text = "SDK Status",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface
            )

            Text(
                text = if (Causality.isInitialized) "Initialized" else "Not initialized",
                style = MaterialTheme.typography.bodyMedium,
                color = if (Causality.isInitialized) {
                    MaterialTheme.colorScheme.primary
                } else {
                    MaterialTheme.colorScheme.error
                }
            )

            Text(
                text = "Device: ${Causality.deviceId.take(8)}...",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Text(
                text = "Events tracked: $eventCount",
                style = MaterialTheme.typography.bodyMedium
            )

            Spacer(modifier = Modifier.height(8.dp))

            // Track Event button — uses DSL builder
            Button(
                onClick = {
                    Causality.track("button_tap") {
                        property("button_name", "track_event")
                        property("screen", "main_screen")
                    }
                    eventCount++
                    scope.launch {
                        snackbarHostState.showSnackbar("Event tracked: button_tap")
                    }
                },
                modifier = Modifier.fillMaxWidth()
            ) {
                Text("Track Event")
            }

            // Track Purchase button — uses top-level event() DSL
            Button(
                onClick = {
                    Causality.track(event("purchase") {
                        property("product_id", "PROD-001")
                        property("price", 29.99)
                        property("currency", "USD")
                    })
                    eventCount++
                    scope.launch {
                        snackbarHostState.showSnackbar("Event tracked: purchase")
                    }
                },
                modifier = Modifier.fillMaxWidth()
            ) {
                Text("Track Purchase")
            }

            // Identify User button
            OutlinedButton(
                onClick = {
                    Causality.identify(
                        userId = "user-compose-123",
                        traits = mapOf("plan" to "premium", "source" to "compose-example")
                    )
                    scope.launch {
                        snackbarHostState.showSnackbar("User identified: user-compose-123")
                    }
                },
                modifier = Modifier.fillMaxWidth()
            ) {
                Text("Identify User")
            }

            // Flush Events button — with loading indicator
            Button(
                onClick = {
                    scope.launch {
                        isFlushing = true
                        try {
                            Causality.flush()
                            snackbarHostState.showSnackbar("Events flushed successfully")
                        } catch (e: Exception) {
                            snackbarHostState.showSnackbar("Flush failed: ${e.message}")
                        } finally {
                            isFlushing = false
                        }
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = !isFlushing,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.secondary,
                    contentColor = MaterialTheme.colorScheme.onSecondary
                )
            ) {
                if (isFlushing) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = MaterialTheme.colorScheme.onSecondary,
                        strokeWidth = 2.dp
                    )
                } else {
                    Text("Flush Events")
                }
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Reset button
            OutlinedButton(
                onClick = {
                    try {
                        Causality.reset()
                        scope.launch {
                            snackbarHostState.showSnackbar("User identity cleared")
                        }
                    } catch (e: Exception) {
                        scope.launch {
                            snackbarHostState.showSnackbar("Reset failed: ${e.message}")
                        }
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                colors = ButtonDefaults.outlinedButtonColors(
                    contentColor = MaterialTheme.colorScheme.error
                )
            ) {
                Text("Reset Identity")
            }
        }
    }
}
