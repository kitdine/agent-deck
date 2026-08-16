import Foundation
import XCTest
@testable import AgentDeckShared

final class AppGroupSnapshotStoreTests: XCTestCase {
	func testAtomicStorePersistsOnlyWidgetSafeProjection() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)

		let projection = AppGroupDesktopSnapshotV1(envelope: envelope)
		try store.write(projection)

		XCTAssertEqual(try store.read(), projection)
		let encoded = try String(contentsOf: store.snapshotURL, encoding: .utf8)
		XCTAssertFalse(encoded.contains("session_id"))
		XCTAssertFalse(encoded.contains("session-1"))
		XCTAssertFalse(encoded.contains("recovery_command"))
		XCTAssertFalse(encoded.contains("credential"))
	}

	func testProjectionFiltersRawDiagnosticTextAndUnknownCodes() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(modifiedPartialFixture())
		let projection = AppGroupDesktopSnapshotV1(envelope: envelope)
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)

		try store.write(projection)

		XCTAssertEqual(
			projection.issueCodes,
			["provider_unavailable", "sessions_unavailable", "usage_unavailable"]
		)
		XCTAssertEqual(projection.health.issueCodes, ["database_missing"])
		let encoded = try String(contentsOf: store.snapshotURL, encoding: .utf8)
		XCTAssertTrue(encoded.contains("database_missing"))
		XCTAssertFalse(encoded.contains("diagnostic path /Users/example/secret"))
		XCTAssertFalse(encoded.contains("health check detail /Users/example/secret"))
		XCTAssertFalse(encoded.contains("recovery_command"))
		XCTAssertFalse(encoded.contains("unrecognized_private_code"))
	}

	func testPrivatePermissionsExistBeforePublicationAndOnFinalCache() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let projection = AppGroupDesktopSnapshotV1(envelope: envelope)
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let recorder = FileModeRecorder()
		let store = AppGroupSnapshotStore(
			directoryURL: temporaryDirectory,
			atomicReplace: { temporaryURL, destinationURL in
				try recorder.record(modeAt: temporaryURL)
				try FileManager.default.moveItem(at: temporaryURL, to: destinationURL)
			}
		)

		try store.write(projection)

		XCTAssertEqual(recorder.mode, 0o600)
		XCTAssertEqual(try fileMode(at: temporaryDirectory), 0o700)
		XCTAssertEqual(try fileMode(at: store.snapshotURL), 0o600)
	}

	func testFailedReplacementPreservesPreviousCache() throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let partial = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-partial.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let originalStore = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		let original = AppGroupDesktopSnapshotV1(envelope: complete)
		try originalStore.write(original)

		let failingStore = AppGroupSnapshotStore(
			directoryURL: temporaryDirectory,
			atomicReplace: { _, _ in throw ReplacementError.failed }
		)
		XCTAssertThrowsError(try failingStore.write(AppGroupDesktopSnapshotV1(envelope: partial)))
		XCTAssertEqual(try originalStore.read(), original)
	}

	func testReadRejectsUnsupportedCacheSchemaVersion() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let projection = AppGroupDesktopSnapshotV1(envelope: envelope)
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		try store.write(projection)

		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: store.snapshotURL)) as? [String: Any])
		object["schema_version"] = AppGroupDesktopSnapshotV1.schemaVersion + 1
		try JSONSerialization.data(withJSONObject: object).write(to: store.snapshotURL)

		XCTAssertThrowsError(try store.read()) { error in
			XCTAssertEqual(
				error as? AppGroupSnapshotStoreError,
				.unsupportedSchemaVersion(AppGroupDesktopSnapshotV1.schemaVersion + 1)
			)
		}
	}

	func testReadRejectsMalformedCacheData() throws {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		try FileManager.default.createDirectory(at: temporaryDirectory, withIntermediateDirectories: true)
		try Data("not-json".utf8).write(to: store.snapshotURL)

		XCTAssertThrowsError(try store.read())
	}

	private func modifiedPartialFixture() throws -> Data {
		let fixture = try desktopFixtureData("snapshot-partial.json")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: fixture) as? [String: Any])
		object["warnings"] = [
			"provider_unavailable",
			"sessions_unavailable",
			"usage_unavailable",
			"diagnostic path /Users/example/secret",
		]
		var data = try XCTUnwrap(object["data"] as? [String: Any])
		var health = try XCTUnwrap(data["health"] as? [String: Any])
		health["checks"] = [
			[
				"name": "health check detail /Users/example/secret",
				"status": "error",
				"code": "database_missing",
				"recovery_command": "do-not-persist",
			],
			[
				"name": "another private detail",
				"status": "error",
				"code": "unrecognized_private_code",
			],
		]
		data["health"] = health
		object["data"] = data
		return try JSONSerialization.data(withJSONObject: object)
	}

	private func fileMode(at url: URL) throws -> Int {
		let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
		let permissions = try XCTUnwrap(attributes[.posixPermissions] as? NSNumber)
		return permissions.intValue & 0o777
	}
}

private final class FileModeRecorder: @unchecked Sendable {
	private let lock = NSLock()
	private var capturedMode: Int?

	var mode: Int? {
		lock.lock()
		defer { lock.unlock() }
		return capturedMode
	}

	func record(modeAt url: URL) throws {
		let attributes = try FileManager.default.attributesOfItem(atPath: url.path)
		let permissions = try XCTUnwrap(attributes[.posixPermissions] as? NSNumber)
		lock.lock()
		capturedMode = permissions.intValue & 0o777
		lock.unlock()
	}
}

private enum ReplacementError: Error {
	case failed
}
