// swift-tools-version:5.9

import PackageDescription

let package = Package(
    name: "UIKitExample",
    platforms: [.iOS(.v14)],
    dependencies: [
        .package(path: "../..") // Points to sdk/ios
    ],
    targets: [
        .executableTarget(
            name: "UIKitExample",
            dependencies: [
                .product(name: "Causality", package: "Causality")
            ],
            path: "UIKitExample"
        )
    ]
)
