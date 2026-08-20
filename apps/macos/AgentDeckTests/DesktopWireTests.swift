import Foundation
import XCTest
@testable import AgentDeckShared

final class DesktopWireTests: XCTestCase {
    func testCanonicalCompleteAndPartialFixturesDecode() throws {
        let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
        let partial = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-partial.json"))

        XCTAssertFalse(complete.partial)
        XCTAssertTrue(complete.warnings.isEmpty)
        XCTAssertTrue(complete.data.provider.available)
        XCTAssertTrue(complete.data.sessions.available)
		XCTAssertTrue(complete.data.usage.presentation.available)
		XCTAssertEqual(complete.data.usage.presentation.scopes.first?.client, "all")

        XCTAssertTrue(partial.partial)
        XCTAssertFalse(partial.warnings.isEmpty)
        XCTAssertFalse(partial.data.provider.available)
        XCTAssertTrue(partial.data.health.available)
		XCTAssertFalse(partial.data.usage.presentation.available)
    }

    func testPeriodScopedAttributionAndSessionsDecodeForEveryPeriod() throws {
        let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
        let scope = try XCTUnwrap(complete.data.usage.presentation.scopes.first)

        // quality and pricing are the Client x Period product. A decoder that
        // still read them as current-period only would render one period's
        // figures under every filter position, which is the half-governed
        // filter this change exists to remove.
        XCTAssertEqual(Set(scope.quality.items.map(\.period)), ["today", "7d", "30d"])
        XCTAssertEqual(scope.quality.items.first?.period, "today")
        XCTAssertEqual(scope.pricing.items.map(\.period), ["today", "7d", "30d"])
        XCTAssertEqual(Set(scope.quality.items.map(\.id)).count, scope.quality.items.count)

        let periods = complete.data.sessions.periods
        XCTAssertTrue(periods.available)
        XCTAssertEqual(periods.items.count, 9)
        XCTAssertEqual(Set(periods.items.map(\.client)), ["all", "codex", "claude"])
        let todayAll = try XCTUnwrap(periods.items.first { $0.period == "today" && $0.client == "all" })
        XCTAssertGreaterThanOrEqual(todayAll.sessions, 0)
        XCTAssertGreaterThanOrEqual(todayAll.medianDurationSeconds, 0)
    }

	func testLegacyFixtureDefaultsMissingAdditiveFields() throws {
		let legacy = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-legacy.json"))
		XCTAssertEqual(legacy.data.usage.presentation, .unavailable)
		// A payload that predates the additive session periods decodes as an
		// unavailable family, not as an error: the change raises no wire version.
		XCTAssertEqual(legacy.data.sessions.periods, .unavailable)
		XCTAssertTrue(legacy.data.sessions.available)
	}

	// A missing additive family decodes as unavailable, but a present family of
	// the wrong type is invalid. `provider.candidates` is another task's
	// additive object and is covered by that task's decoder tests.
	func testPresentMalformedAdditiveFieldsAreRejected() throws {
		let complete = try desktopFixtureData("snapshot-complete.json")
		for mutation in ["presentation", "sessions-periods"] {
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
			var data = try XCTUnwrap(object["data"] as? [String: Any])
			switch mutation {
			case "presentation":
				var usage = try XCTUnwrap(data["usage"] as? [String: Any])
				usage["presentation"] = "not-an-object"
				data["usage"] = usage
			default:
				var sessions = try XCTUnwrap(data["sessions"] as? [String: Any])
				sessions["periods"] = "not-an-object"
				data["sessions"] = sessions
			}
			object["data"] = data
			let malformed = try JSONSerialization.data(withJSONObject: object)
			XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(malformed), mutation)
		}
	}

	func testCompleteFixtureCarriesTheContractCollectionBounds() throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let presentation = complete.data.usage.presentation
		XCTAssertTrue(presentation.available)
		XCTAssertEqual(presentation.scopes.map(\.client), ["all", "codex", "claude"])
		XCTAssertTrue(presentation.clientSubtotals.available)
		XCTAssertEqual(presentation.clientSubtotals.items.count, 6)
		for scope in presentation.scopes {
			XCTAssertEqual(scope.periods.items.map(\.period), ["today", "7d", "30d"], scope.client)
			XCTAssertEqual(scope.daily.items.count, 90, scope.client)
			XCTAssertEqual(scope.rhythm.cells.count, 168, scope.client)
			XCTAssertEqual(scope.pricing.items.map(\.period), ["today", "7d", "30d"], scope.client)
			XCTAssertEqual(Set(scope.quality.items.map(\.period)), ["today", "7d", "30d"], scope.client)
			// Copying today's figures into the wider periods must be visible.
			let totals = scope.periods.items.map(\.totals.tokens)
			XCTAssertNotEqual(totals[0], totals[1], scope.client)
			XCTAssertNotEqual(totals[1], totals[2], scope.client)
		}

		// The exact nine session records, so a dropped or duplicated key fails.
		XCTAssertEqual(
			complete.data.sessions.periods.items.map { $0.period + "/" + $0.client },
			[
				"today/all", "today/codex", "today/claude",
				"7d/all", "7d/codex", "7d/claude",
				"30d/all", "30d/codex", "30d/claude",
			]
		)
	}

	// An unavailable session index reports the family as unavailable with an
	// empty collection. A null collection is a decoding failure, not an
	// unavailable state, which is what blocked the real partial snapshot.
	func testPartialFixtureReportsAnUnavailableSessionPeriodFamily() throws {
		let partial = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-partial.json"))
		XCTAssertFalse(partial.data.sessions.available)
		XCTAssertFalse(partial.data.sessions.periods.available)
		XCTAssertTrue(partial.data.sessions.periods.items.isEmpty)

		var object = try XCTUnwrap(
			JSONSerialization.jsonObject(with: try desktopFixtureData("snapshot-partial.json")) as? [String: Any]
		)
		var data = try XCTUnwrap(object["data"] as? [String: Any])
		var sessions = try XCTUnwrap(data["sessions"] as? [String: Any])
		var periods = try XCTUnwrap(sessions["periods"] as? [String: Any])
		periods["items"] = NSNull()
		sessions["periods"] = periods
		data["sessions"] = sessions
		object["data"] = data
		let nulled = try JSONSerialization.data(withJSONObject: object)
		XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(nulled))
	}

	// A concrete client with no data keeps its record and reports that no family
	// was supplied, rather than presenting synthetic zeros as a measurement.
	func testEmptyClientFixtureReportsUnavailableFamiliesRatherThanZeros() throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let scopes = envelope.data.usage.presentation.scopes
		XCTAssertEqual(scopes.map(\.client), ["all", "codex", "claude"])
		let empty = try XCTUnwrap(scopes.first { $0.client == "claude" })
		XCTAssertFalse(empty.periods.available)
		XCTAssertTrue(empty.periods.items.isEmpty)
		XCTAssertFalse(empty.daily.available)
		XCTAssertTrue(empty.daily.items.isEmpty)
		XCTAssertFalse(empty.quality.available)
		XCTAssertTrue(empty.quality.items.isEmpty)
		XCTAssertFalse(empty.pricing.available)
		XCTAssertTrue(empty.pricing.items.isEmpty)
		XCTAssertFalse(empty.rhythm.available)
		XCTAssertTrue(empty.rhythm.cells.isEmpty)

		let populated = try XCTUnwrap(scopes.first { $0.client == "codex" })
		XCTAssertTrue(populated.periods.available)
		XCTAssertEqual(populated.daily.items.count, 90)
	}

	func testUnsupportedWireVersionIsRejected() throws {
        let complete = try desktopFixtureData("snapshot-complete.json")
        var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
        var data = try XCTUnwrap(object["data"] as? [String: Any])
        data["wire_version"] = 2
        object["data"] = data
        let unsupported = try JSONSerialization.data(withJSONObject: object)

        XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(unsupported)) { error in
            XCTAssertEqual(error as? DesktopWireError, .unsupportedWireVersion(2))
		}
	}

	func testRFC3339TimestampsAllowOffsetsAndFractionsAndRejectMalformedValues() throws {
		let fixture = try desktopFixtureData("snapshot-complete.json")
		let offset = "2026-08-13T18:00:00+08:00"
		let offsetData = try replacingTimestamps(
			in: fixture,
			generatedAt: offset,
			dataGeneratedAt: offset,
			nextRefreshAt: "2026-08-13T18:05:00+08:00"
		)
		XCTAssertEqual(try decodeDesktopWireEnvelopeV1(offsetData).generatedAt, offset)

		let fractional = "2026-08-13T10:00:00.123Z"
		let fractionalData = try replacingTimestamps(
			in: fixture,
			generatedAt: fractional,
			dataGeneratedAt: fractional,
			nextRefreshAt: "2026-08-13T10:05:00.123Z"
		)
		XCTAssertEqual(try decodeDesktopWireEnvelopeV1(fractionalData).data.generatedAt, fractional)

		for invalidData in [
			try replacingTimestamps(in: fixture, generatedAt: "not-a-timestamp"),
			try replacingTimestamps(in: fixture, dataGeneratedAt: "2026-08-13 10:00:00Z"),
		] {
			XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(invalidData)) { error in
				XCTAssertEqual(error as? DesktopWireError, .invalidTimestamp)
			}
		}
	}

	private func replacingTimestamps(
		in fixture: Data,
		generatedAt: String? = nil,
		dataGeneratedAt: String? = nil,
		nextRefreshAt: String? = nil
	) throws -> Data {
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: fixture) as? [String: Any])
		var data = try XCTUnwrap(object["data"] as? [String: Any])
		if let generatedAt {
			object["generated_at"] = generatedAt
		}
		if let dataGeneratedAt {
			data["generated_at"] = dataGeneratedAt
		}
		if let nextRefreshAt {
			data["next_refresh_at"] = nextRefreshAt
		}
		object["data"] = data
		return try JSONSerialization.data(withJSONObject: object)
	}
}
