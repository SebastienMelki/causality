// Typed Swift event structs matching proto/causality/v1/events.proto
// JSON keys use snake_case to match Go encoding/json tags in convert.go
// When sebuf gains a protoc-gen-swift-codable plugin, this file will be auto-generated.

import Foundation

// MARK: - Protocol

/// Protocol for all typed Causality events.
/// Conforming types encode to JSON with snake_case keys matching the proto field names.
public protocol CausalityEvent: Encodable, Sendable {
    /// The event type string used by the Go bridge (e.g., "screen_view", "button_tap").
    static var eventType: String { get }
}

// MARK: - Enums

public enum SwipeDirection: Int, Codable, Sendable {
    case unspecified = 0
    case left = 1
    case right = 2
    case up = 3
    case down = 4
}

public enum ScrollDirection: Int, Codable, Sendable {
    case unspecified = 0
    case up = 1
    case down = 2
}

public enum NetworkType: Int, Codable, Sendable {
    case unspecified = 0
    case wifi = 1
    case cellular2G = 2
    case cellular3G = 3
    case cellular4G = 4
    case cellular5G = 5
    case ethernet = 6
    case offline = 7
}

public enum PermissionStatus: Int, Codable, Sendable {
    case unspecified = 0
    case granted = 1
    case denied = 2
    case deniedPermanently = 3
}

public enum MemoryWarningLevel: Int, Codable, Sendable {
    case unspecified = 0
    case low = 1
    case critical = 2
}

public enum BatteryState: Int, Codable, Sendable {
    case unspecified = 0
    case charging = 1
    case discharging = 2
    case full = 3
}

// MARK: - Helper Types

public struct Coordinates: Codable, Sendable {
    public var x: Float?
    public var y: Float?

    public init(x: Float? = nil, y: Float? = nil) {
        self.x = x
        self.y = y
    }
}

public struct PurchaseItem: Codable, Sendable {
    public var productId: String?
    public var productName: String?
    public var quantity: Int32?
    public var priceCents: Int64?

    public init(
        productId: String? = nil,
        productName: String? = nil,
        quantity: Int32? = nil,
        priceCents: Int64? = nil
    ) {
        self.productId = productId
        self.productName = productName
        self.quantity = quantity
        self.priceCents = priceCents
    }

    enum CodingKeys: String, CodingKey {
        case productId = "product_id"
        case productName = "product_name"
        case quantity
        case priceCents = "price_cents"
    }
}

// MARK: - User Events

public struct UserLogin: CausalityEvent, Codable, Sendable {
    public static let eventType = "user_login"

    public var userId: String?
    public var method: String?
    public var isNewUser: Bool?

    public init(userId: String? = nil, method: String? = nil, isNewUser: Bool? = nil) {
        self.userId = userId
        self.method = method
        self.isNewUser = isNewUser
    }

    enum CodingKeys: String, CodingKey {
        case userId = "user_id"
        case method
        case isNewUser = "is_new_user"
    }
}

public struct UserLogout: CausalityEvent, Codable, Sendable {
    public static let eventType = "user_logout"

    public var userId: String?
    public var reason: String?

    public init(userId: String? = nil, reason: String? = nil) {
        self.userId = userId
        self.reason = reason
    }

    enum CodingKeys: String, CodingKey {
        case userId = "user_id"
        case reason
    }
}

public struct UserSignup: CausalityEvent, Codable, Sendable {
    public static let eventType = "user_signup"

    public var userId: String?
    public var method: String?
    public var referralSource: String?

    public init(userId: String? = nil, method: String? = nil, referralSource: String? = nil) {
        self.userId = userId
        self.method = method
        self.referralSource = referralSource
    }

    enum CodingKeys: String, CodingKey {
        case userId = "user_id"
        case method
        case referralSource = "referral_source"
    }
}

public struct UserProfileUpdate: CausalityEvent, Codable, Sendable {
    public static let eventType = "user_profile_update"

    public var userId: String?
    public var fieldsUpdated: [String]?

    public init(userId: String? = nil, fieldsUpdated: [String]? = nil) {
        self.userId = userId
        self.fieldsUpdated = fieldsUpdated
    }

    enum CodingKeys: String, CodingKey {
        case userId = "user_id"
        case fieldsUpdated = "fields_updated"
    }
}

// MARK: - Screen Events

public struct ScreenView: CausalityEvent, Codable, Sendable {
    public static let eventType = "screen_view"

    public var screenName: String
    public var screenClass: String?
    public var previousScreen: String?
    public var params: [String: String]?

    public init(
        screenName: String,
        screenClass: String? = nil,
        previousScreen: String? = nil,
        params: [String: String]? = nil
    ) {
        self.screenName = screenName
        self.screenClass = screenClass
        self.previousScreen = previousScreen
        self.params = params
    }

    enum CodingKeys: String, CodingKey {
        case screenName = "screen_name"
        case screenClass = "screen_class"
        case previousScreen = "previous_screen"
        case params
    }
}

public struct ScreenExit: CausalityEvent, Codable, Sendable {
    public static let eventType = "screen_exit"

    public var screenName: String
    public var durationMs: Int64?
    public var nextScreen: String?

    public init(screenName: String, durationMs: Int64? = nil, nextScreen: String? = nil) {
        self.screenName = screenName
        self.durationMs = durationMs
        self.nextScreen = nextScreen
    }

    enum CodingKeys: String, CodingKey {
        case screenName = "screen_name"
        case durationMs = "duration_ms"
        case nextScreen = "next_screen"
    }
}

// MARK: - Interaction Events

public struct ButtonTap: CausalityEvent, Codable, Sendable {
    public static let eventType = "button_tap"

    public var buttonId: String
    public var buttonText: String?
    public var screenName: String?
    public var coordinates: Coordinates?

    public init(
        buttonId: String,
        buttonText: String? = nil,
        screenName: String? = nil,
        coordinates: Coordinates? = nil
    ) {
        self.buttonId = buttonId
        self.buttonText = buttonText
        self.screenName = screenName
        self.coordinates = coordinates
    }

    enum CodingKeys: String, CodingKey {
        case buttonId = "button_id"
        case buttonText = "button_text"
        case screenName = "screen_name"
        case coordinates
    }
}

public struct SwipeGesture: CausalityEvent, Codable, Sendable {
    public static let eventType = "swipe_gesture"

    public var direction: SwipeDirection?
    public var screenName: String?
    public var start: Coordinates?
    public var end: Coordinates?
    public var durationMs: Int64?

    public init(
        direction: SwipeDirection? = nil,
        screenName: String? = nil,
        start: Coordinates? = nil,
        end: Coordinates? = nil,
        durationMs: Int64? = nil
    ) {
        self.direction = direction
        self.screenName = screenName
        self.start = start
        self.end = end
        self.durationMs = durationMs
    }

    enum CodingKeys: String, CodingKey {
        case direction
        case screenName = "screen_name"
        case start
        case end
        case durationMs = "duration_ms"
    }
}

public struct ScrollEvent: CausalityEvent, Codable, Sendable {
    public static let eventType = "scroll_event"

    public var screenName: String?
    public var containerId: String?
    public var scrollDepthPercent: Int32?
    public var direction: ScrollDirection?

    public init(
        screenName: String? = nil,
        containerId: String? = nil,
        scrollDepthPercent: Int32? = nil,
        direction: ScrollDirection? = nil
    ) {
        self.screenName = screenName
        self.containerId = containerId
        self.scrollDepthPercent = scrollDepthPercent
        self.direction = direction
    }

    enum CodingKeys: String, CodingKey {
        case screenName = "screen_name"
        case containerId = "container_id"
        case scrollDepthPercent = "scroll_depth_percent"
        case direction
    }
}

public struct TextInput: CausalityEvent, Codable, Sendable {
    public static let eventType = "text_input"

    public var fieldId: String
    public var fieldType: String?
    public var screenName: String?
    public var textLength: Int32?
    public var inputDurationMs: Int64?

    public init(
        fieldId: String,
        fieldType: String? = nil,
        screenName: String? = nil,
        textLength: Int32? = nil,
        inputDurationMs: Int64? = nil
    ) {
        self.fieldId = fieldId
        self.fieldType = fieldType
        self.screenName = screenName
        self.textLength = textLength
        self.inputDurationMs = inputDurationMs
    }

    enum CodingKeys: String, CodingKey {
        case fieldId = "field_id"
        case fieldType = "field_type"
        case screenName = "screen_name"
        case textLength = "text_length"
        case inputDurationMs = "input_duration_ms"
    }
}

public struct LongPress: CausalityEvent, Codable, Sendable {
    public static let eventType = "long_press"

    public var elementId: String?
    public var screenName: String?
    public var coordinates: Coordinates?
    public var durationMs: Int64?

    public init(
        elementId: String? = nil,
        screenName: String? = nil,
        coordinates: Coordinates? = nil,
        durationMs: Int64? = nil
    ) {
        self.elementId = elementId
        self.screenName = screenName
        self.coordinates = coordinates
        self.durationMs = durationMs
    }

    enum CodingKeys: String, CodingKey {
        case elementId = "element_id"
        case screenName = "screen_name"
        case coordinates
        case durationMs = "duration_ms"
    }
}

public struct DoubleTap: CausalityEvent, Codable, Sendable {
    public static let eventType = "double_tap"

    public var elementId: String?
    public var screenName: String?
    public var coordinates: Coordinates?

    public init(
        elementId: String? = nil,
        screenName: String? = nil,
        coordinates: Coordinates? = nil
    ) {
        self.elementId = elementId
        self.screenName = screenName
        self.coordinates = coordinates
    }

    enum CodingKeys: String, CodingKey {
        case elementId = "element_id"
        case screenName = "screen_name"
        case coordinates
    }
}

// MARK: - Commerce Events

public struct ProductView: CausalityEvent, Codable, Sendable {
    public static let eventType = "product_view"

    public var productId: String
    public var productName: String?
    public var category: String?
    public var priceCents: Int64?
    public var currency: String?
    public var source: String?

    public init(
        productId: String,
        productName: String? = nil,
        category: String? = nil,
        priceCents: Int64? = nil,
        currency: String? = nil,
        source: String? = nil
    ) {
        self.productId = productId
        self.productName = productName
        self.category = category
        self.priceCents = priceCents
        self.currency = currency
        self.source = source
    }

    enum CodingKeys: String, CodingKey {
        case productId = "product_id"
        case productName = "product_name"
        case category
        case priceCents = "price_cents"
        case currency
        case source
    }
}

public struct AddToCart: CausalityEvent, Codable, Sendable {
    public static let eventType = "add_to_cart"

    public var productId: String
    public var productName: String?
    public var quantity: Int32?
    public var priceCents: Int64?
    public var currency: String?
    public var cartId: String?

    public init(
        productId: String,
        productName: String? = nil,
        quantity: Int32? = nil,
        priceCents: Int64? = nil,
        currency: String? = nil,
        cartId: String? = nil
    ) {
        self.productId = productId
        self.productName = productName
        self.quantity = quantity
        self.priceCents = priceCents
        self.currency = currency
        self.cartId = cartId
    }

    enum CodingKeys: String, CodingKey {
        case productId = "product_id"
        case productName = "product_name"
        case quantity
        case priceCents = "price_cents"
        case currency
        case cartId = "cart_id"
    }
}

public struct RemoveFromCart: CausalityEvent, Codable, Sendable {
    public static let eventType = "remove_from_cart"

    public var productId: String
    public var quantity: Int32?
    public var cartId: String?
    public var reason: String?

    public init(
        productId: String,
        quantity: Int32? = nil,
        cartId: String? = nil,
        reason: String? = nil
    ) {
        self.productId = productId
        self.quantity = quantity
        self.cartId = cartId
        self.reason = reason
    }

    enum CodingKeys: String, CodingKey {
        case productId = "product_id"
        case quantity
        case cartId = "cart_id"
        case reason
    }
}

public struct CheckoutStart: CausalityEvent, Codable, Sendable {
    public static let eventType = "checkout_start"

    public var cartId: String?
    public var itemCount: Int32?
    public var totalCents: Int64?
    public var currency: String?

    public init(
        cartId: String? = nil,
        itemCount: Int32? = nil,
        totalCents: Int64? = nil,
        currency: String? = nil
    ) {
        self.cartId = cartId
        self.itemCount = itemCount
        self.totalCents = totalCents
        self.currency = currency
    }

    enum CodingKeys: String, CodingKey {
        case cartId = "cart_id"
        case itemCount = "item_count"
        case totalCents = "total_cents"
        case currency
    }
}

public struct CheckoutStep: CausalityEvent, Codable, Sendable {
    public static let eventType = "checkout_step"

    public var cartId: String?
    public var stepNumber: Int32?
    public var stepName: String?
    public var stepDurationMs: Int64?

    public init(
        cartId: String? = nil,
        stepNumber: Int32? = nil,
        stepName: String? = nil,
        stepDurationMs: Int64? = nil
    ) {
        self.cartId = cartId
        self.stepNumber = stepNumber
        self.stepName = stepName
        self.stepDurationMs = stepDurationMs
    }

    enum CodingKeys: String, CodingKey {
        case cartId = "cart_id"
        case stepNumber = "step_number"
        case stepName = "step_name"
        case stepDurationMs = "step_duration_ms"
    }
}

public struct PurchaseComplete: CausalityEvent, Codable, Sendable {
    public static let eventType = "purchase_complete"

    public var orderId: String
    public var cartId: String?
    public var itemCount: Int32?
    public var totalCents: Int64?
    public var currency: String?
    public var paymentMethod: String?
    public var items: [PurchaseItem]?

    public init(
        orderId: String,
        cartId: String? = nil,
        itemCount: Int32? = nil,
        totalCents: Int64? = nil,
        currency: String? = nil,
        paymentMethod: String? = nil,
        items: [PurchaseItem]? = nil
    ) {
        self.orderId = orderId
        self.cartId = cartId
        self.itemCount = itemCount
        self.totalCents = totalCents
        self.currency = currency
        self.paymentMethod = paymentMethod
        self.items = items
    }

    enum CodingKeys: String, CodingKey {
        case orderId = "order_id"
        case cartId = "cart_id"
        case itemCount = "item_count"
        case totalCents = "total_cents"
        case currency
        case paymentMethod = "payment_method"
        case items
    }
}

public struct PurchaseFailed: CausalityEvent, Codable, Sendable {
    public static let eventType = "purchase_failed"

    public var cartId: String?
    public var errorCode: String?
    public var errorMessage: String?
    public var paymentMethod: String?
    public var checkoutStep: Int32?

    public init(
        cartId: String? = nil,
        errorCode: String? = nil,
        errorMessage: String? = nil,
        paymentMethod: String? = nil,
        checkoutStep: Int32? = nil
    ) {
        self.cartId = cartId
        self.errorCode = errorCode
        self.errorMessage = errorMessage
        self.paymentMethod = paymentMethod
        self.checkoutStep = checkoutStep
    }

    enum CodingKeys: String, CodingKey {
        case cartId = "cart_id"
        case errorCode = "error_code"
        case errorMessage = "error_message"
        case paymentMethod = "payment_method"
        case checkoutStep = "checkout_step"
    }
}

// MARK: - System Events

public struct AppStart: CausalityEvent, Codable, Sendable {
    public static let eventType = "app_start"

    public var isColdStart: Bool?
    public var launchDurationMs: Int64?
    public var launchSource: String?
    public var deeplinkUrl: String?

    public init(
        isColdStart: Bool? = nil,
        launchDurationMs: Int64? = nil,
        launchSource: String? = nil,
        deeplinkUrl: String? = nil
    ) {
        self.isColdStart = isColdStart
        self.launchDurationMs = launchDurationMs
        self.launchSource = launchSource
        self.deeplinkUrl = deeplinkUrl
    }

    enum CodingKeys: String, CodingKey {
        case isColdStart = "is_cold_start"
        case launchDurationMs = "launch_duration_ms"
        case launchSource = "launch_source"
        case deeplinkUrl = "deeplink_url"
    }
}

public struct AppBackground: CausalityEvent, Codable, Sendable {
    public static let eventType = "app_background"

    public var foregroundDurationMs: Int64?
    public var currentScreen: String?

    public init(foregroundDurationMs: Int64? = nil, currentScreen: String? = nil) {
        self.foregroundDurationMs = foregroundDurationMs
        self.currentScreen = currentScreen
    }

    enum CodingKeys: String, CodingKey {
        case foregroundDurationMs = "foreground_duration_ms"
        case currentScreen = "current_screen"
    }
}

public struct AppForeground: CausalityEvent, Codable, Sendable {
    public static let eventType = "app_foreground"

    public var backgroundDurationMs: Int64?
    public var resumeScreen: String?

    public init(backgroundDurationMs: Int64? = nil, resumeScreen: String? = nil) {
        self.backgroundDurationMs = backgroundDurationMs
        self.resumeScreen = resumeScreen
    }

    enum CodingKeys: String, CodingKey {
        case backgroundDurationMs = "background_duration_ms"
        case resumeScreen = "resume_screen"
    }
}

public struct AppCrash: CausalityEvent, Codable, Sendable {
    public static let eventType = "app_crash"

    public var crashType: String?
    public var crashMessage: String?
    public var stackTrace: String?
    public var currentScreen: String?

    public init(
        crashType: String? = nil,
        crashMessage: String? = nil,
        stackTrace: String? = nil,
        currentScreen: String? = nil
    ) {
        self.crashType = crashType
        self.crashMessage = crashMessage
        self.stackTrace = stackTrace
        self.currentScreen = currentScreen
    }

    enum CodingKeys: String, CodingKey {
        case crashType = "crash_type"
        case crashMessage = "crash_message"
        case stackTrace = "stack_trace"
        case currentScreen = "current_screen"
    }
}

public struct NetworkChange: CausalityEvent, Codable, Sendable {
    public static let eventType = "network_change"

    public var previousType: NetworkType?
    public var currentType: NetworkType?

    public init(previousType: NetworkType? = nil, currentType: NetworkType? = nil) {
        self.previousType = previousType
        self.currentType = currentType
    }

    enum CodingKeys: String, CodingKey {
        case previousType = "previous_type"
        case currentType = "current_type"
    }
}

public struct PermissionRequest: CausalityEvent, Codable, Sendable {
    public static let eventType = "permission_request"

    public var permissionType: String?
    public var triggerScreen: String?

    public init(permissionType: String? = nil, triggerScreen: String? = nil) {
        self.permissionType = permissionType
        self.triggerScreen = triggerScreen
    }

    enum CodingKeys: String, CodingKey {
        case permissionType = "permission_type"
        case triggerScreen = "trigger_screen"
    }
}

public struct PermissionResult: CausalityEvent, Codable, Sendable {
    public static let eventType = "permission_result"

    public var permissionType: String?
    public var status: PermissionStatus?

    public init(permissionType: String? = nil, status: PermissionStatus? = nil) {
        self.permissionType = permissionType
        self.status = status
    }

    enum CodingKeys: String, CodingKey {
        case permissionType = "permission_type"
        case status
    }
}

public struct MemoryWarning: CausalityEvent, Codable, Sendable {
    public static let eventType = "memory_warning"

    public var availableMemoryBytes: Int64?
    public var usedMemoryBytes: Int64?
    public var level: MemoryWarningLevel?

    public init(
        availableMemoryBytes: Int64? = nil,
        usedMemoryBytes: Int64? = nil,
        level: MemoryWarningLevel? = nil
    ) {
        self.availableMemoryBytes = availableMemoryBytes
        self.usedMemoryBytes = usedMemoryBytes
        self.level = level
    }

    enum CodingKeys: String, CodingKey {
        case availableMemoryBytes = "available_memory_bytes"
        case usedMemoryBytes = "used_memory_bytes"
        case level
    }
}

public struct BatteryChange: CausalityEvent, Codable, Sendable {
    public static let eventType = "battery_change"

    public var batteryLevel: Int32?
    public var state: BatteryState?

    public init(batteryLevel: Int32? = nil, state: BatteryState? = nil) {
        self.batteryLevel = batteryLevel
        self.state = state
    }

    enum CodingKeys: String, CodingKey {
        case batteryLevel = "battery_level"
        case state
    }
}

// MARK: - Custom Event

public struct CustomEvent: CausalityEvent, Codable, Sendable {
    public static let eventType = "custom"

    public var eventName: String
    public var stringParams: [String: String]?
    public var intParams: [String: Int64]?
    public var floatParams: [String: Double]?
    public var boolParams: [String: Bool]?

    public init(
        eventName: String,
        stringParams: [String: String]? = nil,
        intParams: [String: Int64]? = nil,
        floatParams: [String: Double]? = nil,
        boolParams: [String: Bool]? = nil
    ) {
        self.eventName = eventName
        self.stringParams = stringParams
        self.intParams = intParams
        self.floatParams = floatParams
        self.boolParams = boolParams
    }

    enum CodingKeys: String, CodingKey {
        case eventName = "event_name"
        case stringParams = "string_params"
        case intParams = "int_params"
        case floatParams = "float_params"
        case boolParams = "bool_params"
    }
}
