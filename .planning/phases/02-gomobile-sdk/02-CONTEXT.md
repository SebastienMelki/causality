# Phase 2: gomobile SDK (iOS + Android) - Context

**Gathered:** 2026-02-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Ship a cross-platform mobile SDK from a single Go codebase. One implementation, two platforms. Delivers: gomobile bind library (.xcframework + .aar), JSON bridge API, SQLite persistence, batching, offline retry, device context auto-collection, session tracking, and idiomatic Swift/Kotlin wrappers with example apps.

</domain>

<decisions>
## Implementation Decisions

### API Design
- **Init pattern**: Functional options in Go (`WithOption()` variadic), idiomatic equivalents per platform (trailing closures Swift, DSL Kotlin)
- **Event properties**: Typed event structs from proto schemas — compile-time safety via sebuf code generation
- **Schema source**: Proto files via sebuf are source of truth — generates Go + Swift + Kotlin types
- **Error handling**: Silent logging for minor issues, callbacks/delegates for critical failures (auth, disk full). Severity levels.

### Batching & Flush
- **Trigger**: Both event count AND time interval — whichever comes first
- **Defaults**: Fully configurable, no opinionated defaults
- **Minimums**: Enforce sane minimums to prevent 1-event-per-request abuse
- **Background behavior**: Immediate flush + request background time for maximum reliability

### Session Tracking
- **Definition**: Claude's discretion — research best practices, implement robust hybrid (timeout + lifecycle)
- **Auto events**: Automatic session_start and session_end events (industry standard)
- **Default**: Enabled by default, can disable in config

### Device Context
- **Collection scope**: Maximum — everything available without extra permissions
- **Location**: Precise if app has location permission, IP-based fallback otherwise
- **Device ID**: App-scoped by default (privacy-first), opt-in persistent option for cross-reinstall needs
- **Opt-out**: Immediate kill switch that stops collection and purges queue (GDPR requirement)

### User Identity
- **Model**: Multi-identity — device_id (auto) + user_id (optional) + aliases (advanced)
- **Merge strategy**: Server-side decision — SDK sends both IDs, warehouse handles merge
- **Reset options**: `ResetUser()` for soft reset (clear user, keep device), `ResetAll()` for full reset

### Offline & Retry
- **Retention**: Configurable duration for offline event storage
- **Retry strategy**: Pluggable presets — exponential backoff, linear with jitter, connectivity-triggered
- **Payload size**: Configurable with sensible default

### Wrapper APIs (Swift & Kotlin)
- **Native feel**: Idiomatic full native facade — Swift feels Swifty (async/await, Codable), Kotlin feels Kotliny (coroutines, DSL). Go is hidden implementation detail.
- **iOS distribution**: SPM only (Swift Package Manager)
- **Android distribution**: Maven Central only
- **Example apps**: Multiple — barebones UIKit/Views AND modern SwiftUI/Jetpack Compose

### Debugging & Logging
- **Debug mode**: Full — log levels + visual event inspector + local server mode for development
- **Log integration**: Native platform logging (os_log iOS, Logcat Android)

### Testing Support
- **Utilities**: Mock SDK + test mode flag + spy for intercepting real calls
- **Distribution**: Separate test package (CausalityTesting) — keeps production builds clean

### Claude's Discretion
- Session timeout duration and exact lifecycle handling
- Specific retry backoff intervals and caps
- Visual event inspector implementation details
- SQLite schema and migration strategy
- gomobile type bridge design (JSON serialization details)

</decisions>

<specifics>
## Specific Ideas

- "I want to destroy Amplitude" — aim to be better in every dimension: DX, data accuracy, privacy, flexibility
- Proto schemas via sebuf as single source of truth — eventually a UI for schema management that translates to event-driven + warehousing
- Pluggable retry strategies (presets + custom) for maximum flexibility
- Privacy-first device ID with opt-in persistence for advanced use cases
- Server-side identity merge to keep SDK simple and allow flexible warehouse-side policies

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-gomobile-sdk*
*Context gathered: 2026-02-06*
