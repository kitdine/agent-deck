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

	func testWidgetProjectionPreservesCLIWireValuesAcrossCoverageStates() throws {
		for name in ["snapshot-complete.json", "snapshot-empty-client.json", "snapshot-schema-ahead.json"] {
			let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData(name))
			let widget = AppGroupDesktopSnapshotV1(envelope: envelope)
			XCTAssertEqual(widget.partial, envelope.partial, name)
			XCTAssertEqual(widget.usage.available, envelope.data.usage.available, name)
			XCTAssertEqual(widget.usage.tokens, envelope.data.usage.tokens, name)
			XCTAssertEqual(widget.usage.counts, envelope.data.usage.counts, name)
			XCTAssertEqual(widget.usage.providerCost, envelope.data.usage.providerCost, name)
			XCTAssertEqual(widget.usage.catalogBaseCost, envelope.data.usage.catalogBaseCost, name)
			XCTAssertEqual(widget.usage.pricingComplete, envelope.data.usage.pricingComplete, name)
			XCTAssertEqual(widget.usage.presentation, envelope.data.usage.presentation, name)
			XCTAssertEqual(widget.subscription, envelope.data.subscription, name)
		}
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

// MARK: Task 7 — Widget session-scope batch

/// The Go producer's own `desktop session-scope` output for each batch
/// (internal/desktop/widget_summary_test.go), keyed by confirmed default.
struct WidgetBatchFixture: Decodable {
	struct OracleRow: Decodable, Equatable { let count: Int; let unknown: Int }
	let asOf: String
	let batches: [String: [DesktopSessionScopeViewV1]]
	let popover: DesktopSessionScopeViewV1
	let oracle: [String: [String: OracleRow]]

	enum CodingKeys: String, CodingKey { case asOf = "as_of", batches, popover, oracle }

	init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		asOf = try container.decode(String.self, forKey: .asOf)
		let raw = try container.decode([String: [AnyEnvelope]].self, forKey: .batches)
		batches = raw.mapValues { $0.map(\.view) }
		popover = try container.decode(AnyEnvelope.self, forKey: .popover).view
		oracle = try container.decode([String: [String: OracleRow]].self, forKey: .oracle)
	}

	/// Decodes one envelope through the production decoder.
	struct AnyEnvelope: Decodable {
		let view: DesktopSessionScopeViewV1
		init(from decoder: Decoder) throws {
			let value = try decoder.singleValueContainer().decode(JSONValue.self)
			view = try decodeDesktopSessionScopeEnvelopeV1(JSONEncoder().encode(value))
		}
	}

	static func load() throws -> WidgetBatchFixture {
		try JSONDecoder().decode(WidgetBatchFixture.self, from: desktopFixtureData("session-scope-batch.json"))
	}
}

/// A minimal JSON tree so a fixture envelope can be re-encoded byte-for-byte
/// enough for the production envelope decoder.
indirect enum JSONValue: Codable {
	case null, bool(Bool), number(Double), int(Int64), string(String), array([JSONValue]), object([String: JSONValue])

	init(from decoder: Decoder) throws {
		let container = try decoder.singleValueContainer()
		if container.decodeNil() { self = .null }
		else if let value = try? container.decode(Bool.self) { self = .bool(value) }
		else if let value = try? container.decode(Int64.self) { self = .int(value) }
		else if let value = try? container.decode(Double.self) { self = .number(value) }
		else if let value = try? container.decode(String.self) { self = .string(value) }
		else if let value = try? container.decode([JSONValue].self) { self = .array(value) }
		else { self = .object(try container.decode([String: JSONValue].self)) }
	}

	func encode(to encoder: Encoder) throws {
		var container = encoder.singleValueContainer()
		switch self {
		case .null: try container.encodeNil()
		case let .bool(value): try container.encode(value)
		case let .number(value): try container.encode(value)
		case let .int(value): try container.encode(value)
		case let .string(value): try container.encode(value)
		case let .array(value): try container.encode(value)
		case let .object(value): try container.encode(value)
		}
	}
}

/// Answers each selection from a scripted table of views and records every
/// read, so a test sees exactly which selections, pins and instants the host
/// asked for. `override` replaces the answer for one read index.
actor ScriptedWidgetScopeReader: WidgetSessionScopeReading {
	struct Read: Equatable, Sendable {
		let selection: DesktopSessionScopeSelectionV1
		let generationID: String?
		let asOf: String
	}

	private var views: [DesktopSessionScopeSelectionV1: DesktopSessionScopeViewV1]
	private var overrides: [Int: Result<DesktopSessionScopeViewV1, HelperExecutionError>]
	private(set) var reads = [Read]()

	init(
		views: [DesktopSessionScopeViewV1],
		overrides: [Int: Result<DesktopSessionScopeViewV1, HelperExecutionError>] = [:]
	) {
		self.views = Dictionary(uniqueKeysWithValues: views.compactMap { view in view.selection.map { ($0, view) } })
		self.overrides = overrides
	}

	func replace(_ views: [DesktopSessionScopeViewV1]) {
		self.views = Dictionary(uniqueKeysWithValues: views.compactMap { view in view.selection.map { ($0, view) } })
	}

	func widgetSessionScopeView(
		_ selection: DesktopSessionScopeSelectionV1, generationID: String?, asOf: String
	) async throws -> DesktopSessionScopeViewV1 {
		let index = reads.count
		reads.append(Read(selection: selection, generationID: generationID, asOf: asOf))
		if let override = overrides[index] { return try override.get() }
		guard let view = views[selection] else { throw HelperExecutionError.malformedOutput }
		return view
	}
}

/// Re-encodes a view with JSON edits, through the production decoder.
func editedView(_ view: DesktopSessionScopeViewV1, _ edit: (inout [String: Any]) -> Void) throws -> DesktopSessionScopeViewV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(view)) as? [String: Any])
	edit(&object)
	let envelope: [String: Any] = ["schema_version": 1, "command": "desktop.session-scope", "generated_at": "2026-08-13T10:00:00Z",
		"data": object, "warnings": [String](), "partial": !(object["available"] as? Bool ?? false)]
	return try decodeDesktopSessionScopeEnvelopeV1(JSONSerialization.data(withJSONObject: envelope))
}

let widgetBatchNow: @Sendable () -> Date = { ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")! }

final class WidgetSessionScopeBatchTests: XCTestCase {
	func testGoBatchBecomesNineWhitelistedSummariesThatMatchTheOracle() async throws {
		let fixture = try WidgetBatchFixture.load()
		let reader = ScriptedWidgetScopeReader(views: try XCTUnwrap(fixture.batches["auxiliary"]))
		let collector = WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow)
		let defaults = try XCTUnwrap(fixture.batches["auxiliary"]?.first?.default)

		guard case let .accepted(set) = await collector.collect(default: defaults) else {
			return XCTFail("the producer's batch was rejected")
		}
		XCTAssertFalse(set.stale)
		XCTAssertNil(set.reason)
		XCTAssertEqual(set.items.count, 9)
		for item in set.items {
			let row = try XCTUnwrap(fixture.oracle["auxiliary"]?["\(item.client)/\(item.period)"])
			XCTAssertEqual(item.scope, .auxiliary)
			XCTAssertEqual(item.count, row.count, "\(item.client)/\(item.period)")
			XCTAssertEqual(item.unknownCount, 0, "Auxiliary unknown is 0")
			XCTAssertEqual(item.publicationID, "fixture-session-scope-generation-auxiliary")
			XCTAssertEqual(item.generatedAt, fixture.asOf)
		}
		// all × today first fixes the generation; the other eight pin it, and
		// every read shares one instant.
		let reads = await reader.reads
		XCTAssertEqual(reads.count, 9)
		XCTAssertEqual(reads.first?.selection, DesktopSessionScopeSelectionV1(scope: .auxiliary, client: "all", period: "today"))
		XCTAssertNil(reads.first?.generationID)
		XCTAssertEqual(Set(reads.dropFirst().map(\.generationID)), ["fixture-session-scope-generation-auxiliary"])
		XCTAssertEqual(Set(reads.map(\.asOf)), [fixture.asOf])
		XCTAssertEqual(Set(reads.map(\.selection.scope)), [.auxiliary])

		// Serialization allowlist: only aggregates and safe identity/time.
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let data = try JSONEncoder().encode(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: set))
		let root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
		let summary = try XCTUnwrap(root["session_scope_summary"] as? [String: Any])
		XCTAssertEqual(Set(summary.keys), ["stale", "reason", "items"])
		for item in try XCTUnwrap(summary["items"] as? [[String: Any]]) {
			XCTAssertEqual(Set(item.keys), ["available", "scope", "count", "unknown_count", "client", "period", "publication_id", "generated_at"])
		}
		let encoded = String(decoding: data, as: UTF8.self)
		for forbidden in ["session_id", "claude-10", "codex-0", "\"project", "synthetic", "recent", "origin", "revision", "request_id", "read_vector", "override"] {
			XCTAssertFalse(encoded.contains(forbidden), forbidden)
		}
	}

	func testUnconfirmedDefaultStartsNoBatch() async throws {
		let fixture = try WidgetBatchFixture.load()
		let reader = ScriptedWidgetScopeReader(views: try XCTUnwrap(fixture.batches["auxiliary"]))
		let collector = WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow)
		let outcome = await collector.collect(default: .unconfirmed)
		XCTAssertEqual(outcome, .rejected(reason: AppGroupSessionScopeSummaryReason.defaultUnconfirmed))
		let reads = await reader.reads
		XCTAssertTrue(reads.isEmpty)
	}

	func testMovedGenerationDiscardsTheBatchAndRetriesAtMostTwice() async throws {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		let moved = try editedView(views[4]) { $0["generation_id"] = "fixture-session-scope-generation-other" }
		let reader = ScriptedWidgetScopeReader(views: views, overrides: [4: .success(moved), 9: .success(moved), 14: .success(moved)])
		let outcome = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: views[0].default)
		XCTAssertEqual(outcome, .rejected(reason: AppGroupSessionScopeSummaryReason.batchInconsistent))
		let reads = await reader.reads
		XCTAssertEqual(reads.count, 15, "three attempts of five reads, never more")
	}

	func testGoReportingThePinnedGenerationUnavailableRetriesThenSucceeds() async throws {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		let unavailable = try editedView(views[3]) { object in
			for key in ["counts", "origin_coverage", "publication_id", "generation_id", "generated_at"] { object[key] = NSNull() }
			object["available"] = false
			object["reason"] = "generation_unavailable"
			object["recent"] = [Any]()
		}
		let reader = ScriptedWidgetScopeReader(views: views, overrides: [3: .success(unavailable)])
		let outcome = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: views[0].default)
		guard case let .accepted(set) = outcome else { return XCTFail("\(outcome)") }
		XCTAssertEqual(set.items.count, 9)
		let reads = await reader.reads
		XCTAssertEqual(reads.count, 4 + 9)
		XCTAssertNil(reads[4].generationID, "a retry starts a new batch identity")
	}

	func testChangedDefaultDiscardsTheBatchAndTheRetryUsesTheNewDefault() async throws {
		let fixture = try WidgetBatchFixture.load()
		let auxiliary = try XCTUnwrap(fixture.batches["auxiliary"])
		let all = try XCTUnwrap(fixture.batches["all"])
		// The host read Auxiliary; Go reports All as saved on the third read,
		// and answers every later read from the All generation.
		var overrides: [Int: Result<DesktopSessionScopeViewV1, HelperExecutionError>] = [2: .success(all[2])]
		for (offset, view) in all.enumerated() { overrides[3 + offset] = .success(view) }
		let reader = ScriptedWidgetScopeReader(views: auxiliary, overrides: overrides)
		let outcome = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: auxiliary[0].default)
		guard case let .accepted(set) = outcome else { return XCTFail("\(outcome)") }
		XCTAssertEqual(Set(set.items.map(\.scope)), [.all], "the retry used the newly confirmed default")
		let reads = await reader.reads
		XCTAssertEqual(reads.count, 3 + 9)
		XCTAssertEqual(Set(reads.dropFirst(3).map(\.selection.scope)), [.all])
		for item in set.items {
			XCTAssertEqual(item.count, fixture.oracle["all"]?["\(item.client)/\(item.period)"]?.count)
		}
	}

	func testMismatchedSelectionTimeOrAuxiliaryUnknownRejectsTheWholeBatch() async throws {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		let edits: [(String, (inout [String: Any]) -> Void)] = [
			("time", { $0["generated_at"] = "2026-08-13T10:00:01Z" }),
			("selection", { $0["selection"] = ["scope": "auxiliary", "client": "claude", "period": "30d"] }),
		]
		for (name, edit) in edits {
			let bad = try editedView(views[5], edit)
			let reader = ScriptedWidgetScopeReader(views: views, overrides: [5: .success(bad), 11: .success(bad), 17: .success(bad)])
			let outcome = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: views[0].default)
			XCTAssertEqual(outcome, .rejected(reason: AppGroupSessionScopeSummaryReason.batchInconsistent), name)
		}
		let unknown = try editedView(views[5]) { object in
			var coverage = object["origin_coverage"] as? [String: Any] ?? [:]
			coverage["unknown_count"] = 1
			object["origin_coverage"] = coverage
		}
		let reader = ScriptedWidgetScopeReader(views: views, overrides: [5: .success(unknown)])
		let outcome = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: views[0].default)
		XCTAssertEqual(outcome, .rejected(reason: AppGroupSessionScopeSummaryReason.batchInconsistent))
		let reads = await reader.reads
		XCTAssertEqual(reads.count, 6, "an inconsistent answer is not retried")
	}

	func testHelperFailureRejectsWithoutRetryAndIncompleteCoverageIsUnknown() async throws {
		let fixture = try WidgetBatchFixture.load()
		let all = try XCTUnwrap(fixture.batches["all"])
		let failing = ScriptedWidgetScopeReader(views: all, overrides: [7: .failure(.timedOut)])
		let failed = await WidgetSessionScopeBatchCollector(reader: failing, now: widgetBatchNow).collect(default: all[0].default)
		XCTAssertEqual(failed, .rejected(reason: AppGroupSessionScopeSummaryReason.selectionUnavailable))
		let failedReads = await failing.reads
		XCTAssertEqual(failedReads.count, 8)

		let partial = try all.map { view in
			try editedView(view) { object in
				var coverage = object["origin_coverage"] as? [String: Any] ?? [:]
				coverage["complete"] = false
				coverage["reasons"] = ["origin_source_unavailable"]
				object["origin_coverage"] = coverage
			}
		}
		let reader = ScriptedWidgetScopeReader(views: partial)
		guard case let .accepted(set) = await WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow).collect(default: all[0].default) else {
			return XCTFail("partial coverage still has trusted counts")
		}
		XCTAssertTrue(set.items.allSatisfy { $0.unknownCount == nil }, "incomplete coverage is unknown, never 0")
		XCTAssertEqual(set.items.map(\.count), all.compactMap { $0.counts?.sessions })
		let encoded = String(decoding: try JSONEncoder().encode(set), as: UTF8.self)
		XCTAssertTrue(encoded.contains("\"unknown_count\":null"), "null is written, not omitted")
	}

	func testUnsafeReasonsAreWrittenAsTheGenericCode() {
		XCTAssertEqual(AppGroupSessionScopeSummarySetV1.unavailable("/Users/example/claude-mem.db").reason, "selection_unavailable")
		XCTAssertEqual(AppGroupSessionScopeSummarySetV1.unavailable("default_unconfirmed").reason, "default_unconfirmed")
	}

	func testHelperArgumentsCarryThePinAndTheInstant() {
		let selection = DesktopSessionScopeSelectionV1(scope: .auxiliary, client: "codex", period: "7d")
		XCTAssertEqual(
			EmbeddedHelperRunner.sessionScopeArguments(selection, recentLimit: 1, generationID: "g", asOf: "2026-08-13T10:00:00Z"),
			["--format", "json", "desktop", "session-scope", "--scope", "auxiliary", "--client", "codex", "--period", "7d",
			 "--recent-limit", "1", "--generation-id", "g", "--as-of", "2026-08-13T10:00:00Z"]
		)
		XCTAssertEqual(
			EmbeddedHelperRunner.sessionScopeArguments(selection, recentLimit: 5),
			["--format", "json", "desktop", "session-scope", "--scope", "auxiliary", "--client", "codex", "--period", "7d", "--recent-limit", "5"]
		)
	}

	func testSummaryChangeReloadsOnlyMagnitude() async throws {
		let fixture = try WidgetBatchFixture.load()
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let recorder = TimelineReloadRecorder()
		let publisher = WidgetSnapshotPublisher(store: AppGroupSnapshotStore(directoryURL: temporaryDirectory), reloader: WidgetTimelineReloader { recorder.record($0) })
		guard case let .accepted(set) = await WidgetSessionScopeBatchCollector(
			reader: ScriptedWidgetScopeReader(views: try XCTUnwrap(fixture.batches["auxiliary"])), now: widgetBatchNow
		).collect(default: try XCTUnwrap(fixture.batches["auxiliary"]?[0].default)) else { return XCTFail() }

		_ = await publisher.publish(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: set), generation: 1)
		let stale = await publisher.publish(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: set.retained(reason: "default_changed")), generation: 2)
		XCTAssertEqual(stale, .published(affectedKinds: [.magnitude]))
		let unchanged = await publisher.publish(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: set.retained(reason: "default_changed")), generation: 3)
		XCTAssertEqual(unchanged, .published(affectedKinds: []))
		let retained = await publisher.retainedSessionScopeSummary()
		XCTAssertEqual(retained?.items, set.items)
	}

	/// ASC-WS-R1-F2/F3: a publisher reading the App Group file before its
	/// first publication keeps nothing the Widget would reject — a missing
	/// unknown_count or an unknown reason — while an explicit null is kept and
	/// re-encoded as null.
	func testPublisherRetentionReadsTheFileWithTheWidgetRules() async throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let item = AppGroupSessionScopeSummaryV1(
			scope: .work, count: 5, unknownCount: 2, client: "codex", period: "7d", publicationID: "g", generatedAt: "2026-08-13T10:00:00Z"
		)
		func publisher(_ edit: (inout [String: Any]) -> Void) throws -> WidgetSnapshotPublisher {
			let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
			addTeardownBlock { try? FileManager.default.removeItem(at: directory) }
			let store = AppGroupSnapshotStore(directoryURL: directory)
			try store.write(AppGroupDesktopSnapshotV1(
				envelope: envelope, sessionScopeSummary: AppGroupSessionScopeSummarySetV1(stale: false, reason: nil, items: [item])
			))
			let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
			var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
			edit(&set)
			object["session_scope_summary"] = set
			try JSONSerialization.data(withJSONObject: object).write(to: path)
			return WidgetSnapshotPublisher(store: store, reloader: WidgetTimelineReloader { _ in })
		}
		func editItem(_ change: @escaping (inout [String: Any]) -> Void) -> (inout [String: Any]) -> Void {
			{ set in
				var items = set["items"] as? [[String: Any]] ?? []
				change(&items[0])
				set["items"] = items
			}
		}
		let rejected: [(String, (inout [String: Any]) -> Void)] = [
			("missing unknown_count", editItem { $0.removeValue(forKey: "unknown_count") }),
			("unknown above count", editItem { $0["unknown_count"] = 6 }),
			("auxiliary unknown", editItem { $0["scope"] = "auxiliary" }),
			("no publication", editItem { $0["publication_id"] = "" }),
			("bad time", editItem { $0["generated_at"] = "yesterday" }),
			("not available", editItem { $0["available"] = false }),
			("unknown reason", { $0["reason"] = "future_reason" }),
		]
		for (name, edit) in rejected {
			let candidate = try publisher(edit)
			let retained = await candidate.retainedSessionScopeSummary()
			XCTAssertNil(retained, name)
			let last = await candidate.lastSessionScopeSummary()
			XCTAssertNil(last, "\(name): the reuse path falls back to unavailable, never the malformed set")
		}

		let explicit = try publisher(editItem { $0["unknown_count"] = NSNull() })
		let keptValue = await explicit.retainedSessionScopeSummary()
		let kept = try XCTUnwrap(keptValue)
		XCTAssertEqual(kept.items.map(\.unknownCount), [nil])
		let encoded = String(decoding: try JSONEncoder().encode(kept.retained(reason: "selection_unavailable")), as: UTF8.self)
		XCTAssertTrue(encoded.contains("\"unknown_count\":null"), encoded)
		XCTAssertTrue(encoded.contains("\"stale\":true"), encoded)

		let known = try publisher { $0["reason"] = "selection_stale"; $0["stale"] = true }
		let knownRetained = await known.retainedSessionScopeSummary()
		XCTAssertEqual(knownRetained?.items, [item], "a known reason keeps a well-formed set")
	}
}
