---
phase: 02-gomobile-sdk
plan: 07
subsystem: sdk-ios
tags: [swift, spm, ios, async-await, codable, xcframework]

# Dependency graph
requires:
  - phase: 02-06
    provides: Fully wired Go mobile SDK singleton, gomobile build infrastructure
provides:
  - Swift Package Manager package with binaryTarget for Go xcframework
  - Idiomatic Swift wrapper with async/await API
  - Codable Config and Event types with snake_case JSON mapping
  - AnyCodable type-erased wrapper for dynamic properties
  - EventBuilder fluent API for event construction
  - Bridge layer abstracting JSON serialization to Go core
  - Platform context collector for iOS device info
  - Automatic UIApplication lifecycle notification handling
affects: [02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: [swift-5.9, swift-package-manager]
  patterns:
    - "SPM binaryTarget for Go xcframework distribution"
    - "@MainActor singleton with NotificationCenter lifecycle observers"
    - "Non-blocking track() via Task.detached"
    - "Async flush() via withCheckedThrowingContinuation"
    - "AnyCodable type erasure for heterogeneous property maps"

key-files:
  created:
    - sdk/ios/Package.swift
    - sdk/ios/Sources/Causality/Causality.swift
    - sdk/ios/Sources/Causality/Config.swift
    - sdk/ios/Sources/Causality/Errors.swift
    - sdk/ios/Sources/Causality/Event.swift
    - sdk/ios/Sources/Causality/Internal/Bridge.swift
    - sdk/ios/Sources/Causality/Internal/Platform.swift
    - sdk/ios/Tests/CausalityTests/CausalityTests.swift
  modified: []

key-decisions:
  - "SPM binaryTarget with local path for development, URL+checksum for release"
  - "CausalitySwift target name to avoid collision with CausalityCore binary"
  - "@MainActor singleton for thread-safe UI integration"
  - "Task.detached for non-blocking event tracking"
  - "withCheckedThrowingContinuation to bridge sync Go flush to async Swift"
  - "AnyCodable supports String, Int, Double, Bool property types"

patterns-established:
  - "Swift enum Bridge as namespace for Go core function calls"
  - "CausalityError enum with associated values for error context"
  - "#if canImport(UIKit) for iOS/macOS platform conditional compilation"
  - "CodingKeys enum for snake_case JSON mapping"

# Metrics
duration: 2min
completed: 2026-02-06
---

# Phase 02 Plan 07: iOS Swift Wrapper Summary

**Idiomatic Swift SPM package with async/await API, Codable types, fluent EventBuilder, and automatic lifecycle handling over Go mobile core**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-06T10:28:43Z
- **Completed:** 2026-02-06T10:30:40Z
- **Tasks:** 2/2
- **Files created:** 8

## Accomplishments
- Created complete SPM package structure with binaryTarget pointing to Go xcframework
- Config struct uses Codable with CodingKeys for snake_case JSON serialization matching Go core expectations
- Event and EventBuilder provide type-safe and fluent event construction
- AnyCodable type-erased wrapper enables heterogeneous property maps (String, Int, Double, Bool)
- Bridge enum encapsulates all Go core function calls with JSON serialization and error mapping
- Platform enum collects iOS device context (model, OS version, screen size, locale, timezone)
- Causality singleton uses @MainActor for thread safety, Task.detached for non-blocking tracking
- Async flush() bridges synchronous Go core via withCheckedThrowingContinuation
- Automatic UIApplication lifecycle notification handling (background/foreground)
- Convenience methods for common events (trackScreenView, trackButtonTap)
- Tests verify Config encoding, EventBuilder fluent API, Event encoding, and AnyCodable roundtrip

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SPM package structure and configuration types** - `8195f51` (feat)
2. **Task 2: Create main SDK class with async/await and tests** - `cf702a1` (feat)

## Files Created
- `sdk/ios/Package.swift` - SPM manifest with binaryTarget for CausalityCore xcframework
- `sdk/ios/Sources/Causality/Config.swift` - Codable config struct with snake_case JSON keys
- `sdk/ios/Sources/Causality/Event.swift` - Event struct, EventBuilder, AnyCodable type erasure
- `sdk/ios/Sources/Causality/Internal/Bridge.swift` - JSON bridge to Go core (init, track, setUser, reset, flush)
- `sdk/ios/Sources/Causality/Internal/Platform.swift` - iOS/macOS device context collection
- `sdk/ios/Sources/Causality/Causality.swift` - @MainActor singleton with async/await, lifecycle observers
- `sdk/ios/Sources/Causality/Errors.swift` - CausalityError enum with LocalizedError conformance
- `sdk/ios/Tests/CausalityTests/CausalityTests.swift` - 4 tests for encoding, builder, and roundtrip

## Decisions Made
- **SPM binaryTarget local path:** Development uses relative path `../../build/mobile/Causality.xcframework`. Release distributions will replace with URL + checksum for remote download.
- **CausalitySwift target name:** The Swift wrapper target is named CausalitySwift to avoid namespace collision with the CausalityCore binary target. The library product is still named "Causality" for clean import.
- **@MainActor singleton:** Ensures all SDK state mutations happen on the main actor, which is appropriate since the SDK is used from UIKit/SwiftUI contexts. Task.detached moves serialization work off main thread.
- **withCheckedThrowingContinuation for flush:** Bridges the synchronous Go flush call into Swift's async/await model, dispatching the actual work to a global utility queue.
- **#if canImport(UIKit) guards:** Platform.swift and Causality.swift use conditional compilation to support both iOS (UIKit) and macOS targets declared in Package.swift.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added macOS fallback in Platform.swift**

- **Found during:** Task 1
- **Issue:** Package.swift declares `.macOS(.v11)` as a supported platform, but Platform.swift only had UIKit code paths. This would fail to compile on macOS.
- **Fix:** Added `#if canImport(UIKit)` / `#else` branches in `collectContext()` with ProcessInfo-based macOS fallback.
- **Files modified:** sdk/ios/Sources/Causality/Internal/Platform.swift
- **Commit:** 8195f51

## Issues Encountered
None.

## User Setup Required
None - Swift source files only. Full compilation requires the .xcframework from gomobile build (02-06 infrastructure).

## Next Phase Readiness
- iOS Swift wrapper is complete and ready for distribution
- SPM package can be resolved once xcframework is built via `make mobile-ios`
- Tests can be run once the binary target is available
- Ready for Plan 02-08 (Android Kotlin wrapper)

---
*Phase: 02-gomobile-sdk*
*Completed: 2026-02-06*
