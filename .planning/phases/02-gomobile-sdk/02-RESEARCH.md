# Phase 2: gomobile SDK (iOS + Android) - Research

**Researched:** 2026-02-06
**Domain:** gomobile cross-platform mobile SDK development
**Confidence:** HIGH (core gomobile), MEDIUM (native wrappers), HIGH (SQLite)

## Summary

This phase implements a cross-platform mobile analytics SDK using gomobile to generate shared Go libraries for iOS (.xcframework) and Android (.aar). The core Go SDK handles event batching, SQLite persistence, offline retry, and session tracking. Native Swift and Kotlin wrappers provide idiomatic platform APIs.

The gomobile toolchain has strict type limitations: only primitives (string, int, bool, float64), []byte, error, structs with exported fields of supported types, and interfaces are exportable. Complex types like maps, slices of strings, and unsigned integers cannot cross the bridge. The standard solution is a JSON bridge API where all complex data passes as JSON strings.

**Primary recommendation:** Use modernc.org/sqlite (CGO-free) for persistence, JSON string bridge for all complex types, functional options pattern in Go core, and thin native wrappers (Swift/Kotlin) that handle JSON serialization and provide idiomatic async APIs.

## Standard Stack

The established libraries/tools for this domain:

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/mobile/cmd/gomobile | latest | Cross-compile Go to iOS/Android | Official Go mobile toolchain |
| modernc.org/sqlite | v1.44.3 | CGO-free SQLite | Required for gomobile (no CGO), ARM support verified |
| encoding/json | stdlib | JSON serialization | Bridge between Go core and native APIs |
| github.com/google/uuid | v1.6.0 | UUID generation | Already in project, device/session/event IDs |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| apple/swift-protobuf | 1.28+ | Proto code gen for Swift | Typed event structs from proto schemas |
| streem/pbandk | 0.15+ | Proto code gen for Kotlin | Typed event structs from proto schemas |
| zombiezen/go-sqlite | alt | SQLite wrapper | Alternative if modernc issues arise |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| modernc.org/sqlite | mattn/go-sqlite3 | CGO required, breaks gomobile |
| JSON bridge | Protobuf binary | More complex, minimal benefit for mobile |
| gomobile bind | pure native SDKs | Duplicated implementation, maintenance burden |

**Installation:**
```bash
# gomobile toolchain
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# Android NDK (via sdkmanager)
sdkmanager "ndk-bundle"

# iOS requires Xcode on macOS

# SQLite dependency
go get modernc.org/sqlite@v1.44.3
```

## Architecture Patterns

### Recommended Project Structure
```
sdk/
├── mobile/                    # gomobile-bound core (JSON bridge)
│   ├── causality.go          # Init, Track, SetUser, Reset, Flush, GetDeviceId
│   ├── config.go             # Config struct with JSON tags
│   ├── bridge.go             # JSON marshal/unmarshal helpers
│   └── internal/
│       ├── storage/          # SQLite event queue
│       │   ├── db.go         # Database operations
│       │   ├── queue.go      # FIFO queue implementation
│       │   └── migrations.go # Schema migrations
│       ├── batch/            # Event batching logic
│       │   ├── batcher.go    # Size + time triggers
│       │   └── flush.go      # Background flush loop
│       ├── transport/        # HTTP client with retry
│       │   ├── client.go     # HTTP transport
│       │   └── retry.go      # Exponential backoff
│       ├── session/          # Session management
│       │   ├── tracker.go    # Timeout + lifecycle hybrid
│       │   └── events.go     # session_start/end events
│       ├── device/           # Device context collection
│       │   ├── context.go    # Platform-agnostic interface
│       │   └── id.go         # Device ID generation/storage
│       └── identity/         # User identity management
│           └── user.go       # SetUser, Reset, aliases
├── ios/                       # Swift wrapper (SPM package)
│   ├── Package.swift         # SPM manifest with binaryTarget
│   ├── Sources/Causality/
│   │   ├── Causality.swift   # Main SDK class (async/await)
│   │   ├── Config.swift      # Configuration with builder
│   │   ├── Event.swift       # Typed event builders
│   │   └── Internal/
│   │       ├── Bridge.swift  # JSON bridge to Go core
│   │       └── Platform.swift# iOS device context
│   └── Examples/
│       ├── UIKitExample/     # Basic UIKit integration
│       └── SwiftUIExample/   # Modern SwiftUI app
├── android/                   # Kotlin wrapper (Maven)
│   ├── causality/
│   │   ├── build.gradle.kts  # Library module
│   │   └── src/main/kotlin/
│   │       ├── Causality.kt  # Main SDK class (coroutines)
│   │       ├── Config.kt     # Configuration DSL
│   │       ├── Event.kt      # Typed event builders
│   │       └── internal/
│   │           ├── Bridge.kt # JSON bridge to Go core
│   │           └── Platform.kt# Android device context
│   └── examples/
│       ├── views-example/    # Basic Views integration
│       └── compose-example/  # Modern Jetpack Compose
└── build/                     # Build outputs
    ├── Causality.xcframework # iOS artifact
    └── causality.aar         # Android artifact
```

### Pattern 1: JSON Bridge API

**What:** All exported Go functions accept/return only gomobile-compatible types. Complex data passes as JSON strings.

**When to use:** Always for gomobile bind - required by type limitations.

**Example:**
```go
// Source: https://pliutau.com/gomobile-bind-types/
package mobile

import (
    "encoding/json"
    "github.com/SebastienMelki/causality/sdk/mobile/internal/batch"
)

// InitConfig is the JSON structure for SDK initialization
type InitConfig struct {
    APIKey        string `json:"api_key"`
    Endpoint      string `json:"endpoint"`
    AppID         string `json:"app_id"`
    BatchSize     int    `json:"batch_size,omitempty"`
    FlushInterval int    `json:"flush_interval_ms,omitempty"`
    MaxQueueSize  int    `json:"max_queue_size,omitempty"`
    DebugMode     bool   `json:"debug_mode,omitempty"`
}

// Init initializes the SDK with JSON configuration
// Returns empty string on success, error message on failure
func Init(configJSON string) string {
    var cfg InitConfig
    if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
        return "invalid config JSON: " + err.Error()
    }
    // ... initialization logic
    return ""
}

// Track enqueues an event for async sending
// Returns empty string on success, error message on failure
func Track(eventJSON string) string {
    // Parse and enqueue
    return ""
}
```

### Pattern 2: SQLite Event Queue (FIFO)

**What:** Persistent SQLite queue for events with automatic eviction.

**When to use:** All event storage to survive app crashes/kills.

**Example:**
```go
// Source: https://sqlite.org/forum/info/b047f5ef5b76edff
package storage

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    retry_count INTEGER DEFAULT 0,
    last_retry_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_retry ON events(retry_count, last_retry_at);
`

type Queue struct {
    db       *sql.DB
    maxSize  int
}

func NewQueue(dbPath string, maxSize int) (*Queue, error) {
    db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
    if err != nil {
        return nil, err
    }
    if _, err := db.Exec(schema); err != nil {
        return nil, err
    }
    return &Queue{db: db, maxSize: maxSize}, nil
}

// Enqueue adds event, evicting oldest if full
func (q *Queue) Enqueue(eventJSON string) error {
    tx, _ := q.db.Begin()
    defer tx.Rollback()

    // Check queue size and evict oldest if needed
    var count int
    tx.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
    if count >= q.maxSize {
        evictCount := count - q.maxSize + 1
        tx.Exec("DELETE FROM events WHERE id IN (SELECT id FROM events ORDER BY created_at ASC LIMIT ?)", evictCount)
    }

    _, err := tx.Exec("INSERT INTO events (event_json, created_at) VALUES (?, ?)", eventJSON, time.Now().UnixMilli())
    if err != nil {
        return err
    }
    return tx.Commit()
}

// DequeueBatch returns up to n events in FIFO order
func (q *Queue) DequeueBatch(n int) ([]Event, error) {
    rows, _ := q.db.Query("SELECT id, event_json FROM events ORDER BY created_at ASC LIMIT ?", n)
    // ... parse rows
}
```

### Pattern 3: Hybrid Session Tracking

**What:** Combine timeout-based (30s) and lifecycle events for robust session detection.

**When to use:** Default session tracking implementation.

**Example:**
```go
// Source: https://www.braze.com/docs/developer_guide/analytics/tracking_sessions
package session

import (
    "sync"
    "time"
    "github.com/google/uuid"
)

const DefaultSessionTimeout = 30 * time.Second

type Tracker struct {
    mu              sync.Mutex
    sessionID       string
    lastActivity    time.Time
    timeout         time.Duration
    onSessionStart  func(sessionID string)
    onSessionEnd    func(sessionID string, durationMs int64)
}

func NewTracker(timeout time.Duration) *Tracker {
    return &Tracker{
        timeout: timeout,
    }
}

// RecordActivity checks if session is still valid or starts new one
func (t *Tracker) RecordActivity() string {
    t.mu.Lock()
    defer t.mu.Unlock()

    now := time.Now()

    // Check if session expired (timeout elapsed)
    if t.sessionID != "" && now.Sub(t.lastActivity) > t.timeout {
        // End previous session
        duration := t.lastActivity.Sub(t.sessionStart).Milliseconds()
        if t.onSessionEnd != nil {
            t.onSessionEnd(t.sessionID, duration)
        }
        t.sessionID = ""
    }

    // Start new session if needed
    if t.sessionID == "" {
        t.sessionID = uuid.New().String()
        t.sessionStart = now
        if t.onSessionStart != nil {
            t.onSessionStart(t.sessionID)
        }
    }

    t.lastActivity = now
    return t.sessionID
}

// AppDidEnterBackground - flush and mark for potential session end
func (t *Tracker) AppDidEnterBackground() {
    t.mu.Lock()
    t.backgroundedAt = time.Now()
    t.mu.Unlock()
}

// AppWillEnterForeground - check if session should end
func (t *Tracker) AppWillEnterForeground() {
    t.mu.Lock()
    defer t.mu.Unlock()

    if t.sessionID != "" && time.Since(t.backgroundedAt) > t.timeout {
        // Session expired while backgrounded
        // End old, start new
    }
}
```

### Pattern 4: Exponential Backoff with Jitter

**What:** Retry failed requests with randomized exponential delays to prevent thundering herd.

**When to use:** All HTTP requests to server.

**Example:**
```go
// Source: https://docs.mixpanel.com/docs/tracking-methods/sdks/android
package transport

import (
    "math"
    "math/rand"
    "time"
)

type RetryStrategy interface {
    NextDelay(attempt int) time.Duration
    MaxAttempts() int
}

type ExponentialBackoff struct {
    BaseDelay  time.Duration
    MaxDelay   time.Duration
    MaxRetries int
    Jitter     float64 // 0.0 to 1.0
}

func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
    if attempt >= e.MaxRetries {
        return 0 // No more retries
    }

    // Calculate exponential delay
    delay := float64(e.BaseDelay) * math.Pow(2, float64(attempt))
    if delay > float64(e.MaxDelay) {
        delay = float64(e.MaxDelay)
    }

    // Apply jitter
    jitter := delay * e.Jitter * (rand.Float64()*2 - 1)
    delay += jitter

    return time.Duration(delay)
}

// Default preset
var DefaultRetry = &ExponentialBackoff{
    BaseDelay:  1 * time.Second,
    MaxDelay:   5 * time.Minute,
    MaxRetries: 10,
    Jitter:     0.2,
}
```

### Anti-Patterns to Avoid

- **Exporting complex Go types:** Maps, string slices, unsigned ints cannot cross gomobile bridge
- **CGO dependencies:** modernc.org/sqlite is mandatory; mattn/go-sqlite3 breaks gomobile
- **Synchronous flush on Track():** Always queue and return immediately
- **Global state without mutex:** SDK must be thread-safe for concurrent mobile apps
- **Ignoring background task limits:** iOS gives ~30s background time; flush must complete
- **Hardcoded retry intervals:** Use configurable presets for different network conditions

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQLite for mobile | CGO bindings | modernc.org/sqlite | CGO breaks cross-compilation |
| UUID generation | Custom ID logic | github.com/google/uuid | UUIDv7 is time-sortable |
| JSON serialization | Manual parsing | encoding/json | Standard, well-tested |
| Exponential backoff | Simple retry loop | Preset strategies | Jitter prevents thundering herd |
| Session timeout | Manual timers | Hybrid tracker pattern | Lifecycle + timeout for accuracy |
| Device ID persistence | SharedPrefs/UserDefaults | Keychain/EncryptedPrefs | Survives reinstall if needed |

**Key insight:** The gomobile type system is the biggest constraint. Design APIs around JSON strings from the start rather than fighting the bridge.

## Common Pitfalls

### Pitfall 1: Unsupported Types in Exported API

**What goes wrong:** Build fails with "unsupported seqType" or methods silently skipped
**Why it happens:** gomobile only supports: string, int, int32, int64, float32, float64, bool, []byte, error, structs (with supported fields), interfaces (with supported methods)
**How to avoid:** Use JSON string bridge for all complex data; test `gomobile bind` early
**Warning signs:** Methods missing from generated bindings, compile errors mentioning seqType

### Pitfall 2: CGO Dependency Breaks Build

**What goes wrong:** gomobile bind fails on iOS or Android targets
**Why it happens:** Libraries like mattn/go-sqlite3 require CGO which isn't available for mobile cross-compilation
**How to avoid:** Only use pure Go libraries; modernc.org/sqlite is the standard CGO-free SQLite
**Warning signs:** "cgo is not supported" errors, missing symbols in generated framework

### Pitfall 3: NDK Version Incompatibility

**What goes wrong:** Android build fails with missing APIs or ABI errors
**Why it happens:** gomobile has specific NDK version requirements; NDK 24+ dropped API 16 support
**How to avoid:** Use `-androidapi 21` flag; pin NDK version in CI; test on multiple API levels
**Warning signs:** "abi16 not supported", "missing __aeabi_*" linker errors

### Pitfall 4: Background Task Timeout on iOS

**What goes wrong:** Events lost when app backgrounded
**Why it happens:** iOS only grants ~30 seconds background execution
**How to avoid:** Flush immediately on background; use `beginBackgroundTaskWithExpirationHandler`; keep batches small
**Warning signs:** Events missing when app killed, "background task expired" logs

### Pitfall 5: SQLite modernc.org/libc Version Mismatch

**What goes wrong:** Runtime crashes or undefined behavior
**Why it happens:** modernc.org/sqlite has strict dependency on exact libc version
**How to avoid:** Use `go mod tidy` to ensure consistent versions; pin in go.mod
**Warning signs:** Panic in SQLite operations, inconsistent query results

### Pitfall 6: Device ID Reset on Reinstall

**What goes wrong:** Analytics treats reinstall as new device, breaking funnels
**Why it happens:** Standard storage (UserDefaults/SharedPreferences) cleared on uninstall
**How to avoid:** Store device ID in iOS Keychain / Android EncryptedSharedPreferences with opt-in
**Warning signs:** Sudden spikes in "new users" after app updates

## Code Examples

Verified patterns from official sources:

### gomobile Build Commands

```bash
# Source: https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile

# iOS XCFramework (device + simulator)
gomobile bind -target ios -o build/Causality.xcframework ./sdk/mobile

# iOS XCFramework (specific architectures)
gomobile bind -target ios/arm64,iossimulator/arm64,iossimulator/amd64 \
    -o build/Causality.xcframework ./sdk/mobile

# Android AAR (all architectures)
gomobile bind -target android -androidapi 21 -o build/causality.aar ./sdk/mobile

# Android AAR (specific architectures)
gomobile bind -target android/arm64,android/amd64 -androidapi 21 \
    -o build/causality.aar ./sdk/mobile

# With Java package prefix
gomobile bind -target android -javapkg io.causality -o build/causality.aar ./sdk/mobile

# With Objective-C prefix
gomobile bind -target ios -prefix CAU -o build/Causality.xcframework ./sdk/mobile
```

### Swift Wrapper (SPM Package.swift)

```swift
// Source: https://www.avanderlee.com/swift/binary-targets-swift-package-manager/
// swift-tools-version:5.9

import PackageDescription

let package = Package(
    name: "Causality",
    platforms: [
        .iOS(.v14),
        .macOS(.v11)
    ],
    products: [
        .library(name: "Causality", targets: ["CausalitySwift"])
    ],
    targets: [
        .binaryTarget(
            name: "CausalityCore",
            url: "https://github.com/org/causality-ios/releases/download/v1.0.0/Causality.xcframework.zip",
            checksum: "839F9F30DC13C30795666DD8F6FB77DD0E097B83D06954073E34FE5154481F7A"
        ),
        .target(
            name: "CausalitySwift",
            dependencies: ["CausalityCore"],
            path: "Sources/Causality"
        )
    ]
)
```

### Swift Async Wrapper

```swift
// Source: https://sheldonwangrjt.github.io/posts/2025/08/ios-async-await-migration-guide/

import CausalityCore
import Foundation

@MainActor
public final class Causality {
    public static let shared = Causality()

    private init() {}

    public func initialize(config: Config) throws {
        let json = try JSONEncoder().encode(config)
        let result = Mobile.init(String(data: json, encoding: .utf8)!)
        if !result.isEmpty {
            throw CausalityError.initialization(result)
        }
    }

    public func track(_ event: Event) {
        Task.detached(priority: .utility) {
            let json = try? JSONEncoder().encode(event)
            if let jsonString = json.flatMap({ String(data: $0, encoding: .utf8) }) {
                _ = Mobile.track(jsonString)
            }
        }
    }

    public func flush() async throws {
        try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global(qos: .utility).async {
                let result = Mobile.flush()
                if result.isEmpty {
                    continuation.resume()
                } else {
                    continuation.resume(throwing: CausalityError.flush(result))
                }
            }
        }
    }
}

public struct Config: Codable {
    public var apiKey: String
    public var endpoint: String
    public var appId: String
    public var batchSize: Int?
    public var flushIntervalMs: Int?
    public var debugMode: Bool?

    public init(apiKey: String, endpoint: String, appId: String) {
        self.apiKey = apiKey
        self.endpoint = endpoint
        self.appId = appId
    }
}
```

### Kotlin Coroutines Wrapper

```kotlin
// Source: https://kotlinlang.org/docs/type-safe-builders.html

package io.causality

import kotlinx.coroutines.*
import kotlinx.serialization.*
import kotlinx.serialization.json.*
import mobile.Mobile

object Causality {
    private val json = Json { ignoreUnknownKeys = true }
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    fun initialize(block: ConfigBuilder.() -> Unit) {
        val config = ConfigBuilder().apply(block).build()
        val configJson = json.encodeToString(config)
        val result = Mobile.init(configJson)
        if (result.isNotEmpty()) {
            throw CausalityException.Initialization(result)
        }
    }

    fun track(event: Event) {
        scope.launch {
            val eventJson = json.encodeToString(event)
            Mobile.track(eventJson)
        }
    }

    suspend fun flush(): Unit = withContext(Dispatchers.IO) {
        val result = Mobile.flush()
        if (result.isNotEmpty()) {
            throw CausalityException.Flush(result)
        }
    }
}

@Serializable
data class Config(
    @SerialName("api_key") val apiKey: String,
    val endpoint: String,
    @SerialName("app_id") val appId: String,
    @SerialName("batch_size") val batchSize: Int? = null,
    @SerialName("flush_interval_ms") val flushIntervalMs: Int? = null,
    @SerialName("debug_mode") val debugMode: Boolean? = null
)

class ConfigBuilder {
    var apiKey: String = ""
    var endpoint: String = ""
    var appId: String = ""
    var batchSize: Int? = null
    var flushIntervalMs: Int? = null
    var debugMode: Boolean? = null

    fun build() = Config(apiKey, endpoint, appId, batchSize, flushIntervalMs, debugMode)
}
```

### Android Background Flush with WorkManager

```kotlin
// Source: https://developer.android.com/topic/libraries/architecture/workmanager

package io.causality.internal

import android.content.Context
import androidx.work.*
import java.util.concurrent.TimeUnit

class FlushWorker(
    context: Context,
    params: WorkerParameters
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        return try {
            Causality.flush()
            Result.success()
        } catch (e: Exception) {
            if (runAttemptCount < 3) {
                Result.retry()
            } else {
                Result.failure()
            }
        }
    }

    companion object {
        fun schedulePeriodicFlush(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<FlushWorker>(
                15, TimeUnit.MINUTES
            )
                .setConstraints(constraints)
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 1, TimeUnit.MINUTES)
                .build()

            WorkManager.getInstance(context)
                .enqueueUniquePeriodicWork(
                    "causality_flush",
                    ExistingPeriodicWorkPolicy.KEEP,
                    request
                )
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| mattn/go-sqlite3 (CGO) | modernc.org/sqlite (pure Go) | 2022 | Enables gomobile builds |
| .framework bundles | .xcframework bundles | Xcode 11+ (2019) | Required for simulator + device |
| CocoaPods distribution | SPM binaryTarget | Swift 5.3+ (2020) | Modern iOS dependency mgmt |
| JCenter for Android | Maven Central | 2021 (JCenter sunset) | Required for AAR publishing |
| Completion handlers (iOS) | async/await | Swift 5.5+ (2021) | Idiomatic Swift APIs |
| Callbacks (Kotlin) | Coroutines/Flow | Kotlin 1.3+ (2018) | Idiomatic Kotlin APIs |
| API 16 minimum | API 21+ minimum | NDK 24+ (2022) | NDK dropped older support |
| SharedPreferences device ID | EncryptedSharedPreferences | Android 6+ | Privacy-compliant persistence |

**Deprecated/outdated:**
- JCenter/Bintray: Shut down 2021, use Maven Central
- Universal frameworks: Use XCFramework instead
- NDK API 16 support: Dropped in NDK 24+
- UIWebView analytics: Deprecated, use WKWebView

## Open Questions

Things that couldn't be fully resolved:

1. **sebuf Swift/Kotlin Generation**
   - What we know: sebuf generates Go + TypeScript from protos, not Swift/Kotlin
   - What's unclear: User decision says "sebuf generates Swift + Kotlin" but tool doesn't support this
   - Recommendation: Use apple/swift-protobuf + streem/pbandk separately for native proto types, OR generate JSON schemas from proto and use native JSON Codable/kotlinx.serialization

2. **Visual Event Inspector Implementation**
   - What we know: Amplitude/Mixpanel have visual debuggers; decision says "full debug mode with visual event inspector"
   - What's unclear: Exact implementation approach (SwiftUI overlay? Debug server? Platform-specific?)
   - Recommendation: Start with structured logging to platform loggers (os_log, Logcat), add visual inspector in later iteration

3. **Go 1.24 + Latest NDK Compatibility**
   - What we know: Go 1.22/1.23 had issues with older NDKs; project uses Go 1.24
   - What's unclear: Specific Go 1.24 + NDK r27/r28 compatibility
   - Recommendation: Pin NDK r26 initially, test thoroughly, document working combination

4. **Proto-to-Native Event Types**
   - What we know: User wants typed events from proto schemas
   - What's unclear: How to maintain sync between Go proto types and native wrapper types
   - Recommendation: Generate proto types separately per platform; consider shared JSON schema as intermediate format

## Sources

### Primary (HIGH confidence)
- [golang.org/x/mobile/cmd/gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile) - Official gomobile documentation, build flags, type support
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - v1.44.3, CGO-free, ARM64 support verified
- [Apple Developer Documentation](https://developer.apple.com/documentation/xcode/distributing-binary-frameworks-as-swift-packages) - XCFramework + SPM distribution

### Secondary (MEDIUM confidence)
- [pliutau.com/gomobile-bind-types](https://pliutau.com/gomobile-bind-types/) - Detailed gomobile type limitations
- [Braze Session Tracking Docs](https://www.braze.com/docs/developer_guide/analytics/tracking_sessions) - Industry session timeout standards (10-30s)
- [SwiftLee Binary Targets](https://www.avanderlee.com/swift/binary-targets-swift-package-manager/) - SPM binaryTarget configuration
- [Kotlin Type-Safe Builders](https://kotlinlang.org/docs/type-safe-builders.html) - DSL builder pattern

### Tertiary (LOW confidence - needs validation)
- gomobile + Go 1.24 + NDK r27 compatibility (no official documentation found)
- Visual event inspector implementation patterns (varies by SDK)
- Exact iOS background task time (Apple docs say "varies", ~30s observed)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - gomobile and modernc.org/sqlite well-documented
- Architecture: HIGH - JSON bridge pattern is established, verified in multiple projects
- Pitfalls: HIGH - gomobile type limitations well-known, documented in issues
- Native wrappers: MEDIUM - Swift/Kotlin patterns established but integration with gomobile less documented
- CI/Build: MEDIUM - general patterns known, specific Go 1.24 + NDK combos need testing

**Research date:** 2026-02-06
**Valid until:** 2026-03-06 (30 days - gomobile is stable but native platforms evolve)
