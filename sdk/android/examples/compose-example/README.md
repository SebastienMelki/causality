# Causality Compose Example

A Jetpack Compose example application demonstrating integration with the Causality Android SDK.

## Features

- **SDK Initialization** in `Application.onCreate()` using the DSL builder
- **Screen View Tracking** via `LaunchedEffect(Unit)` for automatic tracking on composition
- **Event Tracking** with the `track()` DSL and top-level `event()` builder
- **User Identification** with traits
- **Event Flushing** with `CircularProgressIndicator` loading state
- **Identity Reset** for logout flows
- **Material 3** theming with dynamic colors (Material You on Android 12+)

## Project Structure

```
compose-example/
├── build.gradle.kts                    # App module with Compose dependencies
└── src/main/
    ├── AndroidManifest.xml             # App manifest
    └── kotlin/io/causality/example/compose/
        ├── ExampleApplication.kt       # SDK initialization
        ├── MainActivity.kt             # ComponentActivity with setContent
        └── ui/
            ├── MainScreen.kt           # Main composable with SDK integration
            └── theme/
                └── Theme.kt            # Material 3 theme with dynamic colors
```

## Setup

### Prerequisites

- Android Studio Hedgehog (2023.1.1) or later
- JDK 17
- Android SDK with API 34

### Running

1. Open the `sdk/android/` directory in Android Studio
2. Select the `examples:compose-example` run configuration
3. Run on an emulator (API 24+) or physical device

### Configuration

The SDK is initialized in `ExampleApplication.kt` with:

```kotlin
Causality.initialize(this) {
    apiKey = "example-api-key-compose"
    endpoint = "http://10.0.2.2:8080"  // Emulator localhost
    appId = "compose-example"
    debugMode = true
    enableSessionTracking = true
}
```

To connect to a running Causality server:

1. Start the server: `make dev` (from project root)
2. The emulator address `10.0.2.2` maps to the host machine's `localhost`

## SDK Usage Patterns

### Screen Tracking with LaunchedEffect

```kotlin
@Composable
fun MyScreen() {
    LaunchedEffect(Unit) {
        Causality.trackScreenView("my_screen")
    }
    // Screen content...
}
```

### Event Tracking with DSL

```kotlin
Causality.track("button_tap") {
    property("button_name", "checkout")
    property("screen", "cart")
}
```

### Top-level Event Builder

```kotlin
Causality.track(event("purchase") {
    property("product_id", "PROD-001")
    property("price", 29.99)
})
```

### Async Flush in Coroutine Scope

```kotlin
val scope = rememberCoroutineScope()
scope.launch {
    try {
        Causality.flush()
    } catch (e: Exception) {
        // Handle flush error
    }
}
```

### User Identification

```kotlin
Causality.identify(
    userId = "user-123",
    traits = mapOf("plan" to "premium")
)
```
