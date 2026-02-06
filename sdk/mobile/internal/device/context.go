// Package device provides device context collection and device ID management
// for the Causality mobile SDK.
//
// Device context captures platform information (OS, model, locale, etc.) that
// enriches every tracked event. The context is populated by native wrappers
// (Swift/Kotlin) via SetPlatformContext during SDK initialization.
//
// Device ID provides anonymous user tracking. The ID is generated on first launch
// and persisted to SQLite. ResetAll regenerates the device ID for privacy resets.
package device

import (
	"sync"
)

// SDKVersion is the current version of the Causality mobile SDK.
const SDKVersion = "1.0.0"

// DeviceContext holds device and platform information that enriches every event.
// Platform-specific fields are populated by native wrappers via SetPlatformContext.
type DeviceContext struct {
	// DeviceID is the anonymous device identifier.
	DeviceID string `json:"device_id"`

	// Platform is the operating system ("ios" or "android"), set by native wrapper.
	Platform string `json:"platform"`

	// OSVersion is the OS version string (e.g., "17.2"), set by native wrapper.
	OSVersion string `json:"os_version"`

	// DeviceModel is the device model (e.g., "iPhone 15"), set by native wrapper.
	DeviceModel string `json:"device_model"`

	// DeviceManufacturer is the device maker (e.g., "Apple"), set by native wrapper.
	DeviceManufacturer string `json:"device_manufacturer"`

	// AppVersion is the app version string (e.g., "2.1.0"), set by native wrapper.
	AppVersion string `json:"app_version"`

	// AppBuildNumber is the app build number (e.g., "142"), set by native wrapper.
	AppBuildNumber string `json:"app_build_number"`

	// ScreenWidth is the screen width in pixels, set by native wrapper.
	ScreenWidth int `json:"screen_width"`

	// ScreenHeight is the screen height in pixels, set by native wrapper.
	ScreenHeight int `json:"screen_height"`

	// Locale is the device locale (e.g., "en_US"), set by native wrapper.
	Locale string `json:"locale"`

	// Timezone is the device timezone (e.g., "America/New_York"), set by native wrapper.
	Timezone string `json:"timezone"`

	// SDKVersion is the Causality SDK version.
	SDKVersion string `json:"sdk_version"`

	// Carrier is the mobile carrier name (optional), set by native wrapper.
	Carrier string `json:"carrier,omitempty"`

	// NetworkType is the network type ("wifi", "cellular", "offline"), set by native wrapper.
	NetworkType string `json:"network_type,omitempty"`
}

// platformContext holds the current platform-specific context.
// Protected by platformMu for concurrent access.
var (
	platformMu  sync.RWMutex
	platformCtx platformInfo
)

// platformInfo stores values set by native wrappers.
type platformInfo struct {
	platform           string
	osVersion          string
	model              string
	manufacturer       string
	appVersion         string
	buildNumber        string
	screenWidth        int
	screenHeight       int
	locale             string
	timezone           string
	carrier            string
	networkType        string
}

// SetPlatformContext is called by native wrappers (Swift/Kotlin) during SDK initialization
// to populate platform-specific device information. This function is thread-safe.
func SetPlatformContext(platform, osVersion, model, manufacturer, appVersion, buildNumber string, screenW, screenH int, locale, timezone string) {
	platformMu.Lock()
	defer platformMu.Unlock()

	platformCtx = platformInfo{
		platform:     platform,
		osVersion:    osVersion,
		model:        model,
		manufacturer: manufacturer,
		appVersion:   appVersion,
		buildNumber:  buildNumber,
		screenWidth:  screenW,
		screenHeight: screenH,
		locale:       locale,
		timezone:     timezone,
	}
}

// SetNetworkInfo updates the carrier and network type. Called by native wrappers
// when network conditions change.
func SetNetworkInfo(carrier, networkType string) {
	platformMu.Lock()
	defer platformMu.Unlock()

	platformCtx.carrier = carrier
	platformCtx.networkType = networkType
}

// CollectContext returns a DeviceContext populated with Go-available fields.
// Platform-specific fields come from the last SetPlatformContext call.
// DeviceID must be set separately via GetContext.
func CollectContext() *DeviceContext {
	platformMu.RLock()
	defer platformMu.RUnlock()

	return &DeviceContext{
		Platform:           platformCtx.platform,
		OSVersion:          platformCtx.osVersion,
		DeviceModel:        platformCtx.model,
		DeviceManufacturer: platformCtx.manufacturer,
		AppVersion:         platformCtx.appVersion,
		AppBuildNumber:     platformCtx.buildNumber,
		ScreenWidth:        platformCtx.screenWidth,
		ScreenHeight:       platformCtx.screenHeight,
		Locale:             platformCtx.locale,
		Timezone:           platformCtx.timezone,
		SDKVersion:         SDKVersion,
		Carrier:            platformCtx.carrier,
		NetworkType:        platformCtx.networkType,
	}
}

// GetContext returns the current device context including the device ID from the IDManager.
// This is the primary method for enriching events with device information.
func GetContext(idMgr *IDManager) *DeviceContext {
	ctx := CollectContext()
	if idMgr != nil {
		ctx.DeviceID = idMgr.GetOrCreateDeviceID()
	}
	return ctx
}

// resetPlatformContextForTesting clears the platform context for test isolation.
func resetPlatformContextForTesting() {
	platformMu.Lock()
	defer platformMu.Unlock()
	platformCtx = platformInfo{}
}
