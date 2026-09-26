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
		XCTAssertEqual(projection.nextRefreshAt, "2026-08-13T10:01:00Z")
		XCTAssertEqual(projection.usage.presentation.scopes.map(\.client), ["all", "codex", "claude"])
		XCTAssertEqual(
			projection.usage.presentation.scopes.first?.periods.items.map(\.period),
			["today", "7d", "30d"]
		)
		XCTAssertFalse(encoded.contains("projects"))
	}

	func testFirstPublicationReloadsEveryWidgetKindExactlyOnce() async throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let recorder = TimelineReloadRecorder()
		let store = AppGroupSnapshotStore(
			directoryURL: temporaryDirectory,
			atomicReplace: { temporaryURL, destinationURL in
				try FileManager.default.moveItem(at: temporaryURL, to: destinationURL)
			}
		)
		let publisher = WidgetSnapshotPublisher(
			store: store,
			reloader: WidgetTimelineReloader { recorder.record($0) }
		)

		let result = await publisher.publish(AppGroupDesktopSnapshotV1(envelope: envelope), generation: 1)

		XCTAssertEqual(result, .published(affectedKinds: Set(AppGroupWidgetKind.allCases)))
		XCTAssertEqual(recorder.calls, [Set(AppGroupWidgetKind.allCases)])
	}

	func testGeneratedAndScheduleFieldsDoNotReloadWidgets() async throws {
		let original = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let changed = try snapshot(from: original) { object in
			object["generated_at"] = "2026-08-13T11:00:00Z"
			object["next_refresh_at"] = "2026-08-13T11:01:00Z"
		}
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let recorder = TimelineReloadRecorder()
		let publisher = WidgetSnapshotPublisher(
			store: AppGroupSnapshotStore(directoryURL: directory),
			reloader: WidgetTimelineReloader { recorder.record($0) }
		)

		_ = await publisher.publish(original, generation: 1)
		let result = await publisher.publish(changed, generation: 2)

		XCTAssertEqual(result, .published(affectedKinds: []))
		XCTAssertEqual(recorder.calls, [Set(AppGroupWidgetKind.allCases)])
	}

	func testSubscriptionChangeReloadsOnlyQuota() async throws {
		let original = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let changed = try snapshot(from: original) { object in
			var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
			var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
			clients[0]["stale"] = true
			subscription["clients"] = clients
			object["subscription"] = subscription
		}
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let recorder = TimelineReloadRecorder()
		let publisher = WidgetSnapshotPublisher(
			store: AppGroupSnapshotStore(directoryURL: directory),
			reloader: WidgetTimelineReloader { recorder.record($0) }
		)

		_ = await publisher.publish(original, generation: 1)
		let result = await publisher.publish(changed, generation: 2)

		XCTAssertEqual(result, .published(affectedKinds: [.quota]))
		XCTAssertEqual(recorder.calls.last, [.quota])
	}

	func testKindSemanticFieldsRouteToAffectedWidgets() async throws {
		let original = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))

		let magnitude = try snapshot(from: original) { object in
			var usage = try XCTUnwrap(object["usage"] as? [String: Any])
			var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
			var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
			var periods = try XCTUnwrap(scopes[0]["periods"] as? [String: Any])
			var items = try XCTUnwrap(periods["items"] as? [[String: Any]])
			var totals = try XCTUnwrap(items[0]["totals"] as? [String: Any])
			totals["tokens"] = 987_654
			items[0]["totals"] = totals
			periods["items"] = items
			scopes[0]["periods"] = periods
			presentation["scopes"] = scopes
			usage["presentation"] = presentation
			object["usage"] = usage
		}
		let composition = try snapshot(from: original) { object in
			var usage = try XCTUnwrap(object["usage"] as? [String: Any])
			var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
			var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
			var periods = try XCTUnwrap(scopes[0]["periods"] as? [String: Any])
			var items = try XCTUnwrap(periods["items"] as? [[String: Any]])
			var models = try XCTUnwrap(items[0]["models"] as? [[String: Any]])
			models[0]["share"] = "31.5"
			items[0]["models"] = models
			periods["items"] = items
			scopes[0]["periods"] = periods
			presentation["scopes"] = scopes
			usage["presentation"] = presentation
			object["usage"] = usage
		}
		let trust = try snapshot(from: original) { object in
			var usage = try XCTUnwrap(object["usage"] as? [String: Any])
			var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
			var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
			var quality = try XCTUnwrap(scopes[0]["quality"] as? [String: Any])
			var items = try XCTUnwrap(quality["items"] as? [[String: Any]])
			var tiers = try XCTUnwrap(items[0]["tiers"] as? [[String: Any]])
			tiers[0]["share"] = "22.2"
			items[0]["tiers"] = tiers
			quality["items"] = items
			scopes[0]["quality"] = quality
			presentation["scopes"] = scopes
			usage["presentation"] = presentation
			object["usage"] = usage
		}
		let rhythm = try snapshot(from: original) { object in
			var usage = try XCTUnwrap(object["usage"] as? [String: Any])
			var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
			var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
			var value = try XCTUnwrap(scopes[0]["rhythm"] as? [String: Any])
			value["active_days"] = 6
			scopes[0]["rhythm"] = value
			presentation["scopes"] = scopes
			usage["presentation"] = presentation
			object["usage"] = usage
		}

		try await assertAffectedKinds(original: original, changed: magnitude, expected: [.magnitude, .composition])
		try await assertAffectedKinds(original: original, changed: composition, expected: [.composition])
		try await assertAffectedKinds(original: original, changed: trust, expected: [.trust])
		try await assertAffectedKinds(original: original, changed: rhythm, expected: [.rhythm])
	}

	func testNewerPublicationSupersedesOlderSuspendedBeforeCommit() async throws {
		let original = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let newer = try snapshot(from: original) { object in
			object["partial"] = true
		}
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let barrier = PublicationBarrier()
		let recorder = TimelineReloadRecorder()
		let store = AppGroupSnapshotStore(directoryURL: directory)
		let publisher = WidgetSnapshotPublisher(
			store: store,
			reloader: WidgetTimelineReloader { recorder.record($0) },
			beforeCommit: { await barrier.waitOnce() }
		)

		let olderTask = Task { await publisher.publish(original, generation: 1) }
		await barrier.waitUntilBlocked()
		let newerResult = await publisher.publish(newer, generation: 2)
		await barrier.resume()
		let olderResult = await olderTask.value

		XCTAssertEqual(newerResult, .published(affectedKinds: Set(AppGroupWidgetKind.allCases)))
		XCTAssertEqual(olderResult, .superseded)
		XCTAssertEqual(try store.read(), newer)
		XCTAssertEqual(recorder.calls, [Set(AppGroupWidgetKind.allCases)])
	}

	func testNewerPreCommitFailureStillSupersedesOlderSuspendedWrite() async throws {
		let snapshot = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let barrier = PublicationBarrier()
		let store = AppGroupSnapshotStore(
			directoryURL: directory,
			atomicReplace: { _, _ in throw ReplacementError.failed }
		)
		let publisher = WidgetSnapshotPublisher(store: store, beforeCommit: { await barrier.waitOnce() })

		let olderTask = Task { await publisher.publish(snapshot, generation: 1) }
		await barrier.waitUntilBlocked()
		let newerResult = await publisher.publish(snapshot, generation: 2)
		await barrier.resume()
		let olderResult = await olderTask.value

		XCTAssertEqual(
			newerResult,
			.failedBeforeCommit(retainedVerifiedGeneration: nil, issue: .storageUnavailable)
		)
		XCTAssertEqual(olderResult, .superseded)
	}

	func testPostCommitDurabilityFailureIsIndeterminateAndReloadsNothing() async throws {
		let snapshot = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let recorder = TimelineReloadRecorder()
		let store = AppGroupSnapshotStore(
			directoryURL: directory,
			atomicReplace: { temporaryURL, destinationURL in
				try FileManager.default.moveItem(at: temporaryURL, to: destinationURL)
			},
			durabilitySync: { _, _ in throw ReplacementError.failed }
		)
		let publisher = WidgetSnapshotPublisher(
			store: store,
			reloader: WidgetTimelineReloader { recorder.record($0) }
		)

		let result = await publisher.publish(snapshot, generation: 1)

		XCTAssertEqual(
			result,
			.indeterminateAfterCommit(attemptedGeneration: 1, issue: .storageUnavailable)
		)
		let verifiedGeneration = await publisher.lastVerifiedGeneration
		XCTAssertEqual(verifiedGeneration, nil)
		XCTAssertEqual(recorder.calls, [])
		XCTAssertEqual(store.readExisting(), .known(snapshot))
	}

	func testUnknownBaselineRepairsMalformedFileAndReloadsEveryKind() async throws {
		let snapshot = try AppGroupDesktopSnapshotV1(envelope: decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		try Data("not-json".utf8).write(to: directory.appendingPathComponent(AppGroupSnapshotStore.fileName))
		let recorder = TimelineReloadRecorder()
		let store = AppGroupSnapshotStore(directoryURL: directory)
		let publisher = WidgetSnapshotPublisher(
			store: store,
			reloader: WidgetTimelineReloader { recorder.record($0) }
		)

		let result = await publisher.publish(snapshot, generation: 1)

		XCTAssertEqual(result, .published(affectedKinds: Set(AppGroupWidgetKind.allCases)))
		XCTAssertEqual(try store.read(), snapshot)
	}

	func testBoundedReaderRejectsSymlinkAndNPlusOneBytes() throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		let target = directory.appendingPathComponent("target")
		try Data("safe".utf8).write(to: target)
		let link = directory.appendingPathComponent("link")
		try FileManager.default.createSymbolicLink(at: link, withDestinationURL: target)
		XCTAssertThrowsError(try AppGroupSnapshotBytes.readBounded(at: link)) { error in
			XCTAssertEqual(error as? AppGroupSnapshotByteReadError, .unsafeFile)
		}

		let oversized = directory.appendingPathComponent("oversized")
		try Data(repeating: 0, count: AppGroupSnapshotBytes.maximumBytes + 1).write(to: oversized)
		XCTAssertThrowsError(try AppGroupSnapshotBytes.readBounded(at: oversized)) { error in
			XCTAssertEqual(error as? AppGroupSnapshotByteReadError, .oversized)
		}
	}

	func testBoundedReaderRejectsPathReplacementBetweenLstatAndOpen() throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		let snapshotURL = directory.appendingPathComponent("snapshot")
		try Data("original".utf8).write(to: snapshotURL)

		XCTAssertThrowsError(
			try AppGroupSnapshotBytes.readBounded(at: snapshotURL) {
				try FileManager.default.removeItem(at: snapshotURL)
				try Data("replacement".utf8).write(to: snapshotURL)
			}
		) { error in
			XCTAssertEqual(error as? AppGroupSnapshotByteReadError, .unsafeFile)
		}
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

	func testPrepareWriteCleansOnlyBoundedStaleSnapshotTemporaryFiles() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let projection = AppGroupDesktopSnapshotV1(envelope: envelope)
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let store = AppGroupSnapshotStore(directoryURL: directory)
		try store.write(projection)

		let fileManager = FileManager.default
		let staleDate = Date().addingTimeInterval(-(AppGroupSnapshotStore.abandonedTemporaryFileMinimumAge + 60))
		var staleURLs = [URL]()
		for _ in 0 ..< (AppGroupSnapshotStore.abandonedTemporaryFileCleanupLimit + 4) {
			let url = directory.appendingPathComponent(
				"\(AppGroupSnapshotStore.abandonedTemporaryFilePrefix)\(UUID().uuidString)\(AppGroupSnapshotStore.abandonedTemporaryFileSuffix)"
			)
			try Data("orphan".utf8).write(to: url)
			try fileManager.setAttributes([.modificationDate: staleDate], ofItemAtPath: url.path)
			staleURLs.append(url)
		}

		let freshURL = directory.appendingPathComponent(
			"\(AppGroupSnapshotStore.abandonedTemporaryFilePrefix)\(UUID().uuidString)\(AppGroupSnapshotStore.abandonedTemporaryFileSuffix)"
		)
		try Data("active".utf8).write(to: freshURL)
		let malformedURL = directory.appendingPathComponent(
			"\(AppGroupSnapshotStore.abandonedTemporaryFilePrefix)not-a-uuid\(AppGroupSnapshotStore.abandonedTemporaryFileSuffix)"
		)
		try Data("unrelated".utf8).write(to: malformedURL)
		try fileManager.setAttributes([.modificationDate: staleDate], ofItemAtPath: malformedURL.path)
		let targetURL = directory.appendingPathComponent("symlink-target")
		try Data("target".utf8).write(to: targetURL)
		let symlinkURL = directory.appendingPathComponent(
			"\(AppGroupSnapshotStore.abandonedTemporaryFilePrefix)\(UUID().uuidString)\(AppGroupSnapshotStore.abandonedTemporaryFileSuffix)"
		)
		try fileManager.createSymbolicLink(at: symlinkURL, withDestinationURL: targetURL)

		let prepared = try store.prepareWrite(projection)
		store.discard(prepared)

		let remainingStale = staleURLs.filter { fileManager.fileExists(atPath: $0.path) }
		XCTAssertEqual(remainingStale.count, 4, "cleanup must remain bounded per publication")
		XCTAssertTrue(fileManager.fileExists(atPath: freshURL.path), "fresh temporary files may still be active")
		XCTAssertTrue(fileManager.fileExists(atPath: malformedURL.path), "only the exact UUID temporary-file pattern is eligible")
		XCTAssertTrue(fileManager.fileExists(atPath: symlinkURL.path), "cleanup must not follow or remove symlinks")
		XCTAssertEqual(try store.read(), projection, "cleanup must never touch the destination snapshot")
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

	private func snapshot(
		from source: AppGroupDesktopSnapshotV1,
		mutating body: (inout [String: Any]) throws -> Void
	) throws -> AppGroupDesktopSnapshotV1 {
		let data = try JSONEncoder().encode(source)
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
		try body(&object)
		return try JSONDecoder().decode(
			AppGroupDesktopSnapshotV1.self,
			from: JSONSerialization.data(withJSONObject: object)
		)
	}

	private func assertAffectedKinds(
		original: AppGroupDesktopSnapshotV1,
		changed: AppGroupDesktopSnapshotV1,
		expected: Set<AppGroupWidgetKind>
	) async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let publisher = WidgetSnapshotPublisher(
			store: AppGroupSnapshotStore(directoryURL: directory),
			reloader: WidgetTimelineReloader { _ in }
		)
		_ = await publisher.publish(original, generation: 1)
		let result = await publisher.publish(changed, generation: 2)
		XCTAssertEqual(result, .published(affectedKinds: expected))
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

private final class TimelineReloadRecorder: @unchecked Sendable {
	private let lock = NSLock()
	private var recordedCalls = [Set<AppGroupWidgetKind>]()

	var calls: [Set<AppGroupWidgetKind>] {
		lock.lock()
		defer { lock.unlock() }
		return recordedCalls
	}

	func record(_ kinds: Set<AppGroupWidgetKind>) {
		lock.lock()
		recordedCalls.append(kinds)
		lock.unlock()
	}
}

private actor PublicationBarrier {
	private var blocked = false
	private var entryWaiters = [CheckedContinuation<Void, Never>]()
	private var release: CheckedContinuation<Void, Never>?

	func waitOnce() async {
		guard !blocked else { return }
		blocked = true
		let waiters = entryWaiters
		entryWaiters = []
		for waiter in waiters {
			waiter.resume()
		}
		await withCheckedContinuation { continuation in
			release = continuation
		}
	}

	func waitUntilBlocked() async {
		guard !blocked else { return }
		await withCheckedContinuation { continuation in
			entryWaiters.append(continuation)
		}
	}

	func resume() {
		release?.resume()
		release = nil
	}
}
