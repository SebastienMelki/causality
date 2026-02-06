# Causality UIKit Example

This example demonstrates integrating the Causality SDK with a UIKit-based iOS app.

## Setup

1. Build the XCFramework:
   ```bash
   make mobile-ios
   ```

2. Open in Xcode or build with Swift Package Manager:
   ```bash
   cd sdk/ios/Examples/UIKitExample
   swift build
   ```

3. Update the API key and endpoint in `AppDelegate.swift`

## Features Demonstrated

- SDK initialization in AppDelegate
- Screen view tracking on viewDidLoad
- Custom event tracking with EventBuilder
- User identification with traits
- Async event flush
- User reset

## Code Patterns

### Initialization

SDK initialization belongs in `AppDelegate.application(_:didFinishLaunchingWithOptions:)`:

```swift
try Causality.shared.initialize(config: Config(
    apiKey: "your-api-key",
    endpoint: "https://your-server.com",
    appId: "your-app-id",
    debugMode: true
))
```

### Event Tracking

Track events using the builder pattern:

```swift
let event = EventBuilder(type: "button_tap")
    .property("button_name", "checkout")
    .property("screen", "cart")
    .build()
Causality.shared.track(event)
```

Or use convenience methods:

```swift
Causality.shared.trackScreenView(name: "home")
Causality.shared.trackButtonTap(name: "checkout")
```

### User Identification

```swift
try Causality.shared.identify(
    userId: "user-123",
    traits: [
        "email": AnyCodable("user@example.com"),
        "plan": AnyCodable("premium")
    ]
)
```

### Flush and Reset

```swift
// Async flush
Task {
    try await Causality.shared.flush()
}

// Reset user identity
try Causality.shared.reset()
```
