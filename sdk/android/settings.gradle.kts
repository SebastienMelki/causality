rootProject.name = "causality-android-sdk"

include(":causality")
include(":examples:views-example")
include(":examples:compose-example")

// Include AAR from build output
// For development, point to local build
// For release, this would be a published artifact
