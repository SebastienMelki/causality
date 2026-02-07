---
phase: "02"
plan: "10"
subsystem: "sdk-android-examples"
tags: ["android", "views", "viewbinding", "material-design", "example-app"]
dependency-graph:
  requires: ["02-08"]
  provides: ["android-views-example"]
  affects: []
tech-stack:
  added: ["androidx.appcompat", "com.google.android.material", "androidx.constraintlayout"]
  patterns: ["view-binding", "application-init", "lifecyclescope-coroutine"]
key-files:
  created:
    - "sdk/android/examples/views-example/build.gradle.kts"
    - "sdk/android/examples/views-example/src/main/AndroidManifest.xml"
    - "sdk/android/examples/views-example/src/main/kotlin/io/causality/example/views/ExampleApplication.kt"
    - "sdk/android/examples/views-example/src/main/kotlin/io/causality/example/views/MainActivity.kt"
    - "sdk/android/examples/views-example/src/main/res/layout/activity_main.xml"
    - "sdk/android/examples/views-example/src/main/res/values/strings.xml"
    - "sdk/android/examples/views-example/README.md"
  modified: []
decisions: []
metrics:
  duration: "4m"
  completed: "2026-02-06"
---

# Phase 02 Plan 10: Android Views Example Summary

Android Views example app demonstrating full Causality SDK integration with View Binding, Material Design components, and Application-level initialization.

## What Was Built

### Views Example Application

A complete Android application module at `sdk/android/examples/views-example/` that demonstrates all Causality SDK features using the traditional Android Views system:

**ExampleApplication.kt** -- Application class that initializes the SDK in `onCreate()` using the Kotlin DSL configuration builder. Uses `10.0.2.2` as the emulator-to-host localhost alias.

**MainActivity.kt** -- Single activity with View Binding (`ActivityMainBinding`) demonstrating six SDK operations:
- `track("button_tap") { ... }` -- DSL-based event tracking with properties
- `track(event("purchase_complete") { ... })` -- Event DSL for custom events
- `trackScreenView("settings_screen", ...)` -- Convenience screen view method
- `identify(userId, traits)` -- User identification with trait map
- `flush()` -- Suspend function called from `lifecycleScope.launch`
- `reset()` -- Identity reset back to anonymous

**activity_main.xml** -- Material Design layout using ConstraintLayout with:
- Info card showing device ID and current user
- Three sections: Event Tracking, Identity, Queue Management
- MaterialButton variants (filled, tonal, outlined)
- Status card at bottom for feedback

**build.gradle.kts** -- Application module (not library) with `viewBinding = true`, Material components, ConstraintLayout, and coroutines dependencies.

## Task Completion

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 | Create Android Views example application | `531b7a4` | Complete |

## Deviations from Plan

None -- plan executed exactly as written.

Note: `settings.gradle.kts` and root `build.gradle.kts` already contained the required `include(":examples:views-example")` and `com.android.application` plugin from a prior commit, so no modification was needed.

## Verification

- [x] Views example demonstrates Application initialization (`ExampleApplication.kt`)
- [x] Views example uses View Binding (`ActivityMainBinding`) and Material components
- [x] Example shows event tracking, identification, flush, reset (6 buttons)
- [x] README has setup instructions (prerequisites, setup, architecture)
- [x] settings.gradle.kts includes `:examples:views-example` module

## Next Phase Readiness

No blockers. The views example is a standalone demonstration module that depends only on the `:causality` SDK module from 02-08.
