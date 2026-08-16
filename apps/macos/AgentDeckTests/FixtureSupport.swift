import Foundation
@testable import AgentDeckShared

func desktopFixtureURL(_ name: String) -> URL {
	URL(fileURLWithPath: #filePath)
		.deletingLastPathComponent()
		.deletingLastPathComponent()
		.deletingLastPathComponent()
		.deletingLastPathComponent()
		.appendingPathComponent("desktop/fixtures/v1/\(name)")
}

func desktopFixtureData(_ name: String) throws -> Data {
    try Data(contentsOf: desktopFixtureURL(name))
}

func makeEmbeddedHelperBundle(in temporaryDirectory: URL) throws -> URL {
    let fileManager = FileManager.default
    let bundleURL = temporaryDirectory.appendingPathComponent("AgentDeck.app", isDirectory: true)
    let helperURL = bundleURL
        .appendingPathComponent("Contents", isDirectory: true)
        .appendingPathComponent("Helpers", isDirectory: true)
        .appendingPathComponent("agentdeck", isDirectory: false)
    try fileManager.createDirectory(at: helperURL.deletingLastPathComponent(), withIntermediateDirectories: true)
    guard fileManager.createFile(atPath: helperURL.path, contents: Data("#!/bin/sh\nexit 0\n".utf8)) else {
        throw CocoaError(.fileWriteUnknown)
    }
    try fileManager.setAttributes([.posixPermissions: 0o700], ofItemAtPath: helperURL.path)
    return bundleURL
}

enum TestHelperBehavior: Sendable {
    case output(HelperProcessOutput)
    case helperError(HelperExecutionError)
    case cancellation
}

struct RecordedHelperInvocation: Equatable, Sendable {
    let executableURL: URL
    let arguments: [String]
    let environment: [String: String]
    let timeout: Duration
}

actor RecordingHelperProcess: EmbeddedHelperProcess {
    private let behavior: TestHelperBehavior
    private var invocations = [RecordedHelperInvocation]()

    init(behavior: TestHelperBehavior) {
        self.behavior = behavior
    }

    func run(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        timeout: Duration
    ) async throws -> HelperProcessOutput {
        invocations.append(
            RecordedHelperInvocation(
                executableURL: executableURL,
                arguments: arguments,
                environment: environment,
                timeout: timeout
            )
        )
        switch behavior {
        case let .output(output):
            return output
        case let .helperError(error):
            throw error
        case .cancellation:
            throw CancellationError()
        }
    }

    func recordedInvocations() -> [RecordedHelperInvocation] {
        invocations
    }
}
