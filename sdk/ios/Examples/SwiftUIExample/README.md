# Causality SwiftUI Example

This example demonstrates integrating the Causality SDK with a SwiftUI-based iOS app.

## Setup

1. Build the XCFramework:
   ```bash
   make mobile-ios
   ```

2. Open in Xcode or build with Swift Package Manager:
   ```bash
   cd sdk/ios/Examples/SwiftUIExample
   swift build
   ```

3. Update the API key and endpoint in `SwiftUIExampleApp.swift`

## Features Demonstrated

- SDK initialization in App struct init
- Screen view tracking on appear
- Custom event tracking with EventBuilder
- E-commerce purchase tracking with typed properties
- User identification with traits
- Async flush with loading indicator
- User reset

## SwiftUI Patterns

### Initialization in App Struct

SDK initialization belongs in the App struct's `init()`:

```swift
@main
struct MyApp: App {
    init() {
        try? Causality.shared.initialize(config: Config(
            apiKey: "your-api-key",
            endpoint: "https://your-server.com",
            appId: "your-app-id",
            debugMode: true
        ))
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
```

### Track on View Appear

Use `.onAppear` to track screen views:

```swift
.onAppear {
    Causality.shared.trackScreenView(name: "home")
}
```

### Event Tracking with Builder

```swift
let event = EventBuilder(type: "purchase")
    .property("product_id", "pro-subscription")
    .property("price", 9.99)
    .property("currency", "USD")
    .build()
Causality.shared.track(event)
```

### Async Flush with Task

```swift
Task {
    try await Causality.shared.flush()
}
```

### User Identification

```swift
try Causality.shared.identify(
    userId: "user-123",
    traits: [
        "name": AnyCodable("Jane Smith"),
        "premium": AnyCodable(true)
    ]
)
```
