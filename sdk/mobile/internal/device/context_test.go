package device

import (
	"path/filepath"
	"testing"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
)

func TestCollectContext_ReturnsSDKVersion(t *testing.T) {
	resetPlatformContextForTesting()

	ctx := CollectContext()
	if ctx.SDKVersion != SDKVersion {
		t.Errorf("expected SDK version %q, got %q", SDKVersion, ctx.SDKVersion)
	}
	if ctx.SDKVersion == "" {
		t.Error("SDK version must not be empty")
	}
}

func TestCollectContext_EmptyWithoutPlatformContext(t *testing.T) {
	resetPlatformContextForTesting()

	ctx := CollectContext()
	if ctx.Platform != "" {
		t.Errorf("expected empty platform, got %q", ctx.Platform)
	}
	if ctx.OSVersion != "" {
		t.Errorf("expected empty OS version, got %q", ctx.OSVersion)
	}
	if ctx.DeviceModel != "" {
		t.Errorf("expected empty device model, got %q", ctx.DeviceModel)
	}
}

func TestSetPlatformContext_StoresValues(t *testing.T) {
	resetPlatformContextForTesting()

	SetPlatformContext(
		"ios", "17.2", "iPhone 15 Pro", "Apple",
		"2.1.0", "142",
		1179, 2556,
		"en_US", "America/New_York",
	)

	ctx := CollectContext()

	if ctx.Platform != "ios" {
		t.Errorf("expected platform 'ios', got %q", ctx.Platform)
	}
	if ctx.OSVersion != "17.2" {
		t.Errorf("expected OS version '17.2', got %q", ctx.OSVersion)
	}
	if ctx.DeviceModel != "iPhone 15 Pro" {
		t.Errorf("expected model 'iPhone 15 Pro', got %q", ctx.DeviceModel)
	}
	if ctx.DeviceManufacturer != "Apple" {
		t.Errorf("expected manufacturer 'Apple', got %q", ctx.DeviceManufacturer)
	}
	if ctx.AppVersion != "2.1.0" {
		t.Errorf("expected app version '2.1.0', got %q", ctx.AppVersion)
	}
	if ctx.AppBuildNumber != "142" {
		t.Errorf("expected build number '142', got %q", ctx.AppBuildNumber)
	}
	if ctx.ScreenWidth != 1179 {
		t.Errorf("expected screen width 1179, got %d", ctx.ScreenWidth)
	}
	if ctx.ScreenHeight != 2556 {
		t.Errorf("expected screen height 2556, got %d", ctx.ScreenHeight)
	}
	if ctx.Locale != "en_US" {
		t.Errorf("expected locale 'en_US', got %q", ctx.Locale)
	}
	if ctx.Timezone != "America/New_York" {
		t.Errorf("expected timezone 'America/New_York', got %q", ctx.Timezone)
	}
	if ctx.SDKVersion != SDKVersion {
		t.Errorf("expected SDK version %q, got %q", SDKVersion, ctx.SDKVersion)
	}
}

func TestSetNetworkInfo_StoresValues(t *testing.T) {
	resetPlatformContextForTesting()

	SetNetworkInfo("Verizon", "cellular")

	ctx := CollectContext()
	if ctx.Carrier != "Verizon" {
		t.Errorf("expected carrier 'Verizon', got %q", ctx.Carrier)
	}
	if ctx.NetworkType != "cellular" {
		t.Errorf("expected network type 'cellular', got %q", ctx.NetworkType)
	}
}

func TestSetPlatformContext_Overwrite(t *testing.T) {
	resetPlatformContextForTesting()

	SetPlatformContext("ios", "16.0", "iPhone 14", "Apple", "1.0.0", "100", 1170, 2532, "en_US", "UTC")
	SetPlatformContext("android", "14.0", "Pixel 8", "Google", "2.0.0", "200", 1080, 2400, "fr_FR", "Europe/Paris")

	ctx := CollectContext()
	if ctx.Platform != "android" {
		t.Errorf("expected platform 'android' after overwrite, got %q", ctx.Platform)
	}
	if ctx.DeviceModel != "Pixel 8" {
		t.Errorf("expected model 'Pixel 8' after overwrite, got %q", ctx.DeviceModel)
	}
}

func TestGetContext_IncludesDeviceID(t *testing.T) {
	resetPlatformContextForTesting()

	dir := t.TempDir()
	db, err := storage.NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	idMgr := NewIDManager(db, false)

	SetPlatformContext("ios", "17.0", "iPhone 15", "Apple", "1.0.0", "1", 1179, 2556, "en_US", "UTC")

	ctx := GetContext(idMgr)
	if ctx.DeviceID == "" {
		t.Error("expected non-empty device ID in context")
	}
	if ctx.Platform != "ios" {
		t.Errorf("expected platform 'ios', got %q", ctx.Platform)
	}
	if ctx.SDKVersion != SDKVersion {
		t.Errorf("expected SDK version %q, got %q", SDKVersion, ctx.SDKVersion)
	}
}

func TestGetContext_NilIDManager(t *testing.T) {
	resetPlatformContextForTesting()

	ctx := GetContext(nil)
	if ctx.DeviceID != "" {
		t.Errorf("expected empty device ID with nil IDManager, got %q", ctx.DeviceID)
	}
	if ctx.SDKVersion != SDKVersion {
		t.Errorf("expected SDK version %q, got %q", SDKVersion, ctx.SDKVersion)
	}
}
