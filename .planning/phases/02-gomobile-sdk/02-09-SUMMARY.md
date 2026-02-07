---
phase: 02-gomobile-sdk
plan: 09
subsystem: sdk
tags: [ios, swift, uikit, swiftui, spm, example-app]

# Dependency graph
requires:
  - phase: 02-gomobile-sdk
    provides: iOS Swift wrapper (Causality SPM package with async/await API)
provides:
  - UIKit example application with AppDelegate SDK initialization
  - SwiftUI example application with App struct SDK initialization
  - Developer reference for all SDK features (track, identify, flush, reset)
affects: [02-10-android-examples, 03-web-app]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - UIKit AppDelegate + SceneDelegate initialization pattern
    - SwiftUI App struct init() initialization pattern
    - EventBuilder fluent API usage
    - Async flush with Task and MainActor dispatch

key-files:
  created:
    - sdk/ios/Examples/UIKitExample/Package.swift
    - sdk/ios/Examples/UIKitExample/UIKitExample/AppDelegate.swift
    - sdk/ios/Examples/UIKitExample/UIKitExample/SceneDelegate.swift
    - sdk/ios/Examples/UIKitExample/UIKitExample/ViewController.swift
    - sdk/ios/Examples/UIKitExample/UIKitExample/Info.plist
    - sdk/ios/Examples/UIKitExample/README.md
    - sdk/ios/Examples/SwiftUIExample/Package.swift
    - sdk/ios/Examples/SwiftUIExample/SwiftUIExample/SwiftUIExampleApp.swift
    - sdk/ios/Examples/SwiftUIExample/SwiftUIExample/ContentView.swift
    - sdk/ios/Examples/SwiftUIExample/SwiftUIExample/Info.plist
    - sdk/ios/Examples/SwiftUIExample/README.md
  modified: []

key-decisions:
  - "SPM local package dependency (path: ../..) for development, no Xcode project files"
  - "UIKit uses scene-based lifecycle (UISceneSession) for modern iOS"
  - "SwiftUI uses @main App struct with init() for SDK initialization"
  - "Info.plist NSAllowsLocalNetworking for localhost development testing"

patterns-established:
  - "UIKit initialization: Causality.shared.initialize in didFinishLaunchingWithOptions"
  - "SwiftUI initialization: Causality.shared.initialize in App init()"
  - "Screen tracking: .onAppear for SwiftUI, viewDidLoad for UIKit"
  - "Async flush: Task { try await Causality.shared.flush() } with MainActor UI updates"

# Metrics
duration: 3min
completed: 2026-02-06
---

# Phase 02 Plan 09: iOS Example Applications Summary

**UIKit and SwiftUI example apps demonstrating full Causality SDK integration with EventBuilder, identify, async flush, and reset**

## Performance

- **Duration:** 3 min
- **Started:** 2026-02-06T10:36:57Z
- **Completed:** 2026-02-06T10:40:03Z
- **Tasks:** 2
- **Files created:** 11

## Accomplishments
- UIKit example with AppDelegate initialization, SceneDelegate window setup, and ViewController with four SDK action buttons
- SwiftUI example with App struct initialization, onAppear screen tracking, purchase event tracking, and toast overlay feedback
- Both examples demonstrate all SDK features: track, trackScreenView, EventBuilder, identify, flush, reset
- READMEs with setup instructions and platform-idiomatic code patterns

## Task Commits

Each task was committed atomically:

1. **Task 1: Create UIKit example application** - `8e84955` (feat)
2. **Task 2: Create SwiftUI example application** - `6dc1e89` (feat)

## Files Created/Modified
- `sdk/ios/Examples/UIKitExample/Package.swift` - SPM manifest with Causality dependency
- `sdk/ios/Examples/UIKitExample/UIKitExample/AppDelegate.swift` - SDK init in didFinishLaunchingWithOptions
- `sdk/ios/Examples/UIKitExample/UIKitExample/SceneDelegate.swift` - Window setup with navigation controller
- `sdk/ios/Examples/UIKitExample/UIKitExample/ViewController.swift` - UI with track, identify, flush, reset buttons
- `sdk/ios/Examples/UIKitExample/UIKitExample/Info.plist` - Scene manifest + local networking
- `sdk/ios/Examples/UIKitExample/README.md` - Setup instructions and UIKit code patterns
- `sdk/ios/Examples/SwiftUIExample/Package.swift` - SPM manifest with Causality dependency
- `sdk/ios/Examples/SwiftUIExample/SwiftUIExample/SwiftUIExampleApp.swift` - SDK init in App init()
- `sdk/ios/Examples/SwiftUIExample/SwiftUIExample/ContentView.swift` - SwiftUI view with all SDK features
- `sdk/ios/Examples/SwiftUIExample/SwiftUIExample/Info.plist` - Local networking ATS exception
- `sdk/ios/Examples/SwiftUIExample/README.md` - Setup instructions and SwiftUI code patterns

## Decisions Made
- Used SPM executable targets with local package dependency (`path: "../.."`) instead of Xcode project files for simplicity and source control friendliness
- UIKit example uses scene-based lifecycle (UISceneSession/SceneDelegate) as the modern standard since iOS 13
- SwiftUI example uses `@main` App struct with synchronous `init()` for SDK initialization
- Both Info.plist files include `NSAllowsLocalNetworking: true` for development against localhost server
- Toast feedback via UIAlertController (UIKit) and custom overlay (SwiftUI) for visual confirmation of SDK actions

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- iOS examples complete, covering both UIKit and SwiftUI paradigms
- Android examples (02-10) can follow the same pattern with Kotlin/Jetpack Compose
- Full build verification requires xcframework from `make mobile-ios` (gomobile build infrastructure from 02-06)

---
*Phase: 02-gomobile-sdk*
*Completed: 2026-02-06*
