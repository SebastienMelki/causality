# Causality Android Views Example

Demonstrates Causality SDK integration with a traditional Android Views application using View Binding and Material Design components.

## Features Demonstrated

- **SDK Initialization** -- Application-level setup in `ExampleApplication.kt`
- **Event Tracking** -- Button tap events using the DSL builder
- **Custom Events** -- Purchase event using the `event()` DSL function
- **Screen Views** -- `trackScreenView()` convenience method
- **User Identification** -- `identify()` with user traits
- **Manual Flush** -- Coroutine-based `flush()` call
- **Identity Reset** -- `reset()` to clear user identity

## Prerequisites

1. Build the Go mobile core AAR:
   ```bash
   # From the project root
   make build-mobile-android
   ```

2. Start the Causality server:
   ```bash
   make docker-up
   ```

## Setup

1. Open the `sdk/android` directory in Android Studio.

2. The project includes the views-example module via `settings.gradle.kts`:
   ```kotlin
   include(":examples:views-example")
   ```

3. Update the API key in `ExampleApplication.kt`:
   ```kotlin
   Causality.initialize(this) {
       apiKey = "your-actual-api-key"
       endpoint = "http://10.0.2.2:8080"  // localhost from emulator
       appId = "views-example"
   }
   ```

4. Run the `views-example` run configuration on an emulator or device.

## Project Structure

```
views-example/
  src/main/
    AndroidManifest.xml           -- App manifest with INTERNET permission
    kotlin/.../ExampleApplication.kt  -- SDK initialization
    kotlin/.../MainActivity.kt        -- UI with tracking demos
    res/layout/activity_main.xml      -- Material Design layout
    res/values/strings.xml            -- String resources
```

## Architecture

- **ExampleApplication** -- Initializes SDK in `onCreate()`, before any Activity starts
- **MainActivity** -- Uses View Binding (`ActivityMainBinding`) for type-safe view access
- **Layout** -- ConstraintLayout with Material cards and buttons organized by feature section

## Notes

- The endpoint `10.0.2.2` is the Android emulator alias for the host machine's `localhost`
- Session tracking is enabled automatically via `ProcessLifecycleOwner`
- Events are batched (default: 10 events or 30s interval) and sent in the background
- The `flush()` function is a suspend function and must be called from a coroutine scope
