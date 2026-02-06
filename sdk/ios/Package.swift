// swift-tools-version:5.9

import PackageDescription

let package = Package(
    name: "Causality",
    platforms: [
        .iOS(.v14),
        .macOS(.v11)
    ],
    products: [
        .library(
            name: "Causality",
            targets: ["Causality"]
        )
    ],
    targets: [
        // Binary target for Go core (local path for development)
        .binaryTarget(
            name: "CausalityCore",
            path: "../../build/mobile/CausalityCore.xcframework"
        ),
        // Swift wrapper
        .target(
            name: "Causality",
            dependencies: ["CausalityCore"],
            path: "Sources/Causality"
        ),
        .testTarget(
            name: "CausalityTests",
            dependencies: ["Causality"],
            path: "Tests/CausalityTests"
        )
    ]
)
