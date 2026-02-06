package io.causality.internal

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.util.DisplayMetrics
import android.view.WindowManager
import mobile.Mobile
import java.util.Locale
import java.util.TimeZone

internal object Platform {
    fun collectAndSetContext(context: Context) {
        val packageInfo = try {
            context.packageManager.getPackageInfo(context.packageName, 0)
        } catch (e: PackageManager.NameNotFoundException) {
            null
        }

        val windowManager = context.getSystemService(Context.WINDOW_SERVICE) as WindowManager
        val displayMetrics = DisplayMetrics()
        @Suppress("DEPRECATION")
        windowManager.defaultDisplay.getMetrics(displayMetrics)

        Mobile.setPlatformContext(
            "android",                                              // platform
            Build.VERSION.RELEASE,                                  // osVersion
            Build.MODEL,                                            // model
            Build.MANUFACTURER,                                     // manufacturer
            packageInfo?.versionName ?: "unknown",                  // appVersion
            (packageInfo?.let {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    it.longVersionCode.toString()
                } else {
                    @Suppress("DEPRECATION")
                    it.versionCode.toString()
                }
            } ?: "unknown"),                                        // buildNumber
            displayMetrics.widthPixels,                             // screenWidth
            displayMetrics.heightPixels,                            // screenHeight
            Locale.getDefault().toString(),                         // locale
            TimeZone.getDefault().id                                // timezone
        )
    }
}
