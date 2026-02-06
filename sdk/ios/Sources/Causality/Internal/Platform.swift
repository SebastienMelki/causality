import Foundation
#if canImport(UIKit)
import UIKit
#endif

/// Collects iOS platform context
enum Platform {
    static func collectContext() -> (
        platform: String,
        osVersion: String,
        model: String,
        manufacturer: String,
        appVersion: String,
        buildNumber: String,
        screenWidth: Int,
        screenHeight: Int,
        locale: String,
        timezone: String
    ) {
        #if canImport(UIKit)
        let device = UIDevice.current
        let bundle = Bundle.main
        let screen = UIScreen.main

        return (
            platform: "ios",
            osVersion: device.systemVersion,
            model: device.model,
            manufacturer: "Apple",
            appVersion: bundle.infoDictionary?["CFBundleShortVersionString"] as? String ?? "unknown",
            buildNumber: bundle.infoDictionary?["CFBundleVersion"] as? String ?? "unknown",
            screenWidth: Int(screen.bounds.width * screen.scale),
            screenHeight: Int(screen.bounds.height * screen.scale),
            locale: Locale.current.identifier,
            timezone: TimeZone.current.identifier
        )
        #else
        let bundle = Bundle.main
        return (
            platform: "macos",
            osVersion: ProcessInfo.processInfo.operatingSystemVersionString,
            model: "Mac",
            manufacturer: "Apple",
            appVersion: bundle.infoDictionary?["CFBundleShortVersionString"] as? String ?? "unknown",
            buildNumber: bundle.infoDictionary?["CFBundleVersion"] as? String ?? "unknown",
            screenWidth: 0,
            screenHeight: 0,
            locale: Locale.current.identifier,
            timezone: TimeZone.current.identifier
        )
        #endif
    }

    static func setPlatformContext() {
        let ctx = collectContext()
        MobileSetPlatformContext(
            ctx.platform,
            ctx.osVersion,
            ctx.model,
            ctx.manufacturer,
            ctx.appVersion,
            ctx.buildNumber,
            ctx.screenWidth,
            ctx.screenHeight,
            ctx.locale,
            ctx.timezone
        )
    }
}
