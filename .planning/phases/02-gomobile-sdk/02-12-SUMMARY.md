---
phase: "02"
plan: "12"
subsystem: "android-sdk"
tags: ["android", "compose", "material3", "example", "kotlin"]
dependency-graph:
  requires: ["02-08"]
  provides: ["compose-example-app"]
  affects: []
tech-stack:
  added: ["compose-bom:2024.01.00", "material3", "activity-compose:1.8.2"]
  patterns: ["ComponentActivity+setContent", "LaunchedEffect", "rememberCoroutineScope", "Material3-dynamic-colors"]
key-files:
  created:
    - "sdk/android/examples/compose-example/build.gradle.kts"
    - "sdk/android/examples/compose-example/src/main/AndroidManifest.xml"
    - "sdk/android/examples/compose-example/src/main/kotlin/io/causality/example/compose/ExampleApplication.kt"
    - "sdk/android/examples/compose-example/src/main/kotlin/io/causality/example/compose/MainActivity.kt"
    - "sdk/android/examples/compose-example/src/main/kotlin/io/causality/example/compose/ui/MainScreen.kt"
    - "sdk/android/examples/compose-example/src/main/kotlin/io/causality/example/compose/ui/theme/Theme.kt"
    - "sdk/android/examples/compose-example/README.md"
  modified:
    - "sdk/android/settings.gradle.kts"
    - "sdk/android/build.gradle.kts"
decisions: []
metrics:
  duration: "~3 minutes"
  completed: "2026-02-06"
---

# Phase 02 Plan 12: Jetpack Compose Example Application Summary

Compose example app demonstrating Causality Android SDK integration with modern Compose architecture, Material 3 dynamic theming, and coroutine-based event flushing.

## What Was Done

### Task 1: Create Jetpack Compose example application

Created a complete Jetpack Compose example application that demonstrates all major SDK features:

**ExampleApplication.kt** - SDK initialization in `Application.onCreate()` using the DSL builder with emulator-compatible endpoint (`10.0.2.2:8080`), debug mode enabled, and session tracking.

**MainActivity.kt** - Minimal `ComponentActivity` using `setContent` to host Compose UI within the `CausalityTheme`.

**MainScreen.kt** - Main composable demonstrating:
- `LaunchedEffect(Unit)` for automatic screen view tracking on composition entry
- `Causality.track("button_tap") { property("button_name", "track_event") }` DSL usage
- `Causality.track(event("purchase") { ... })` top-level event builder DSL
- `Causality.identify()` with userId and traits
- `Causality.flush()` with `CircularProgressIndicator` loading state via coroutine scope
- `Causality.reset()` for identity clearing
- `SnackbarHost` for user feedback on all operations
- `Scaffold` + `TopAppBar` + `Column` layout
- Event counter state tracking with `mutableIntStateOf`

**Theme.kt** - Material 3 theming with:
- Dynamic colors (Material You) on Android 12+ (API 31+)
- Custom Causality brand color scheme fallback (blue primary, teal secondary)
- Light and dark theme support via `isSystemInDarkTheme()`

**build.gradle.kts** - Application module with:
- `minSdk = 24` (Compose requirement)
- Compose BOM 2024.01.00 for version alignment
- Compose compiler extension 1.5.8 (matching Kotlin 1.9.22)
- Dependency on `:causality` project module

**settings.gradle.kts** updated with both `:examples:views-example` and `:examples:compose-example` includes (coordinated with parallel plan 02-10).

**README.md** with setup instructions, project structure, and SDK usage patterns for Compose.

## Deviations from Plan

None - plan executed exactly as written.

## Decisions Made

No new architectural decisions required.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 0bd1818 | feat | Create Jetpack Compose example application |

## Verification

- [x] Compose example demonstrates Application initialization (ExampleApplication.kt)
- [x] Uses modern Compose patterns: LaunchedEffect, rememberCoroutineScope
- [x] Example shows event tracking (track DSL + event builder), identification, flush, reset
- [x] Material 3 theming with dynamic colors implemented (Theme.kt)
- [x] README has setup instructions and usage patterns
- [x] settings.gradle.kts includes `:examples:compose-example` module

## Next Phase Readiness

No blockers. The compose example is self-contained and ready for use as a reference application. It depends on plan 02-08 (Android Kotlin wrapper) which is already complete.
