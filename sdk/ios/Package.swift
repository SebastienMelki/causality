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
            targets: ["CausalitySwift"]
        )
    ],
    targets: [
        // Binary target for Go core (local path for development)
        .binaryTarget(
            name: "CausalityCore",
            path: "../../build/mobile/Causality.xcframework"
        ),
        // Swift wrapper
        .target(
            name: "CausalitySwift",
            dependencies: ["CausalityCore"],
            path: "Sources/Causality"
        ),
        .testTarget(
            name: "CausalityTests",
            dependencies: ["CausalitySwift"],
            path: "Tests/CausalityTests"
        )
    ]
)
