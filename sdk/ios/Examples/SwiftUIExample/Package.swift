// swift-tools-version:5.9

import PackageDescription

let package = Package(
    name: "SwiftUIExample",
    platforms: [.iOS(.v14)],
    dependencies: [
        .package(path: "../..") // Points to sdk/ios
    ],
    targets: [
        .executableTarget(
            name: "SwiftUIExample",
            dependencies: [
                .product(name: "Causality", package: "Causality")
            ],
            path: "SwiftUIExample"
        )
    ]
)
