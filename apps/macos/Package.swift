// swift-tools-version: 6.2
import PackageDescription

// This package exists only to exercise AgentDeckShared without Xcode. The
// Xcode project remains the canonical application project and bundle build.
let package = Package(
    name: "AgentDeckMacOSFoundation",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "AgentDeckShared", targets: ["AgentDeckShared"]),
        .executable(name: "AgentDeckFoundationVerifier", targets: ["AgentDeckFoundationVerifier"]),
    ],
    targets: [
        .target(
            name: "AgentDeckShared",
            path: "AgentDeckShared"
        ),
        .executableTarget(
            name: "AgentDeckFoundationVerifier",
            dependencies: ["AgentDeckShared"],
            path: "AgentDeckVerification"
        ),
        .testTarget(
            name: "AgentDeckSharedTests",
            dependencies: ["AgentDeckShared"],
            path: "AgentDeckTests"
        ),
    ]
)
