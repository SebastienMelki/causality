---
phase: 02-gomobile-sdk
plan: 08
subsystem: sdk-android
tags: [kotlin, android, coroutines, dsl-builder, kotlinx-serialization, lifecycle]

# Dependency graph
requires:
  - phase: 02-06
    provides: Go mobile core exports (Init, Track, SetUser, Reset, Flush, etc.)
provides:
  - Idiomatic Kotlin wrapper with coroutines API
  - DSL builders for Config and Event
  - Automatic Android lifecycle handling via ProcessLifecycleOwner
  - JSON bridge to Go core via kotlinx.serialization
  - Android device context collection (model, OS, screen size)
affects:
  - 02-09: iOS Swift wrapper (parallel platform wrapper)
  - 02-10: Integration testing (Android SDK testing)
  - 02-12: Distribution packaging (Maven/Gradle publishing)

# Tech tracking
tech-stack:
  added:
    - kotlinx-coroutines-android: 1.7.3
    - kotlinx-serialization-json: 1.6.2
    - androidx-lifecycle-runtime-ktx: 2.7.0
    - androidx-lifecycle-process: 2.7.0
  patterns:
    - Kotlin DSL builders (config {} and event {} top-level functions)
    - Sealed exception hierarchy (CausalityException subclasses)
    - JSON bridge pattern (Kotlin serialization -> Go mobile JSON API)
    - Coroutine-backed non-blocking track()
    - ProcessLifecycleOwner for automatic background/foreground handling

# File tracking
key-files:
  created:
    - sdk/android/settings.gradle.kts
    - sdk/android/build.gradle.kts
    - sdk/android/causality/build.gradle.kts
    - sdk/android/causality/src/main/AndroidManifest.xml
    - sdk/android/causality/consumer-rules.pro
    - sdk/android/causality/src/main/kotlin/io/causality/Config.kt
    - sdk/android/causality/src/main/kotlin/io/causality/Event.kt
    - sdk/android/causality/src/main/kotlin/io/causality/Causality.kt
    - sdk/android/causality/src/main/kotlin/io/causality/Errors.kt
    - sdk/android/causality/src/main/kotlin/io/causality/internal/Bridge.kt
    - sdk/android/causality/src/main/kotlin/io/causality/internal/Platform.kt
    - sdk/android/causality/src/test/kotlin/io/causality/ConfigTest.kt
    - sdk/android/causality/src/test/kotlin/io/causality/EventTest.kt
  modified:
    - .gitignore (scoped android/ pattern to root-only)

# Decisions
decisions:
  - id: android-min-sdk
    decision: "minSdk 21, compileSdk 34, JVM 17"
    reason: "minSdk 21 covers 99%+ of active Android devices; compileSdk 34 for latest APIs"
  - id: kotlin-serialization
    decision: "kotlinx.serialization for JSON bridge (not Gson/Moshi)"
    reason: "Compile-time codegen, no reflection, smaller footprint, native Kotlin multiplatform support"
  - id: coroutine-backed-track
    decision: "track() launches coroutine on Dispatchers.IO, never blocks caller"
    reason: "Industry standard for analytics SDKs - tracking must never block UI thread"
  - id: lifecycle-observer
    decision: "ProcessLifecycleOwner for automatic background/foreground detection"
    reason: "No manual Application/Activity callbacks needed; AndroidX standard pattern"
  - id: sealed-exceptions
    decision: "Sealed CausalityException hierarchy for typed error handling"
    reason: "Kotlin idiom for exhaustive when-matching; each error category as distinct subclass"

# Metrics
metrics:
  duration: "3 minutes"
  completed: "2026-02-06"
  tasks: 3/3
  files-created: 13
  files-modified: 1
  test-count: 9
---

# Phase 02 Plan 08: Android Kotlin Wrapper Summary

Idiomatic Kotlin wrapper with DSL builders, coroutines-backed tracking, and automatic lifecycle handling over Go mobile core via JSON bridge.

## What Was Built

### Android Library Project Structure (Task 1)
- **Gradle configuration**: Root `settings.gradle.kts` and `build.gradle.kts` with Kotlin 1.9.22 and serialization plugin
- **Library module**: `causality/build.gradle.kts` with dependencies on kotlinx-coroutines, kotlinx-serialization, and AndroidX lifecycle
- **AndroidManifest**: INTERNET and ACCESS_NETWORK_STATE permissions
- **ProGuard rules**: Consumer rules preserving Go mobile classes and kotlinx.serialization annotations

### Kotlin SDK Classes (Task 2)
- **Config.kt**: `@Serializable` data class with snake_case JSON mapping via `@SerialName`; `ConfigBuilder` with validation (`require` for apiKey, endpoint, appId); top-level `config {}` DSL function
- **Event.kt**: `@Serializable` data class with `JsonObject` properties; `EventBuilder` with fluent `property()` overloads for String/Int/Long/Double/Boolean; top-level `event {}` DSL function
- **Causality.kt**: Singleton object implementing `DefaultLifecycleObserver`; DSL-based `initialize(context) {}` and `track(type) {}`; non-blocking `track()` via `CoroutineScope(Dispatchers.IO + SupervisorJob())`; suspend `flush()` with `withContext(Dispatchers.IO)`; convenience methods `trackScreenView()` and `trackButtonTap()`
- **Errors.kt**: Sealed `CausalityException` hierarchy with NotInitialized, Initialization, Tracking, Identification, Reset, Flush subclasses
- **Bridge.kt**: Internal JSON bridge using `kotlinx.serialization.Json` to serialize Config/Event/UserPayload, calling `Mobile.*` Go exports, mapping non-empty return strings to typed exceptions
- **Platform.kt**: Internal Android device context collector using `Build.*`, `PackageManager`, `DisplayMetrics`, `Locale`, `TimeZone` -- calls `Mobile.setPlatformContext()`

### Unit Tests (Task 3)
- **ConfigTest.kt**: 5 tests -- DSL builder creates valid config, serializes with snake_case, requires apiKey, requires endpoint, requires appId
- **EventTest.kt**: 4 tests -- DSL creates valid event, serializes correctly, null properties when empty, all 5 property types supported

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] .gitignore `android/` pattern blocked sdk/android/ commits**
- **Found during:** Task 1
- **Issue:** `.gitignore` line 37 had `android/` which matches any directory named `android` anywhere in the tree, including `sdk/android/`
- **Fix:** Changed `android/` to `/android/` (root-only) and `ios/` to `/ios/` (root-only); added comment explaining that `sdk/ios/` and `sdk/android/` are source code
- **Files modified:** `.gitignore`
- **Commit:** `758d70f`

## Verification

- All 13 files created in correct directory structure
- Kotlin source files have valid syntax (cannot compile without Android SDK + AAR, as expected)
- Test files have correct package declarations and JUnit annotations
- Build verification deferred to integration testing (requires Android SDK and gomobile-generated AAR)

## Next Phase Readiness

No blockers. Plan 02-09 (iOS Swift wrapper) can proceed independently. Plan 02-10 (integration testing) will need the gomobile AAR and Android SDK to compile and run the Kotlin tests.
