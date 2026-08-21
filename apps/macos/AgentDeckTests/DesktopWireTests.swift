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
		XCTAssertEqual(complete.data.provider.candidates.count, 1)
		// One option per client per route. The fixture seeds a current route for
		// both clients, so the built-in candidate offers codex and claude, each
		// direct and through a wrapper. A bare count hid which of those changed
		// when the fixture was regenerated, so the identities are asserted.
		XCTAssertEqual(
			complete.data.provider.candidates[0].options.map { "\($0.client)/\($0.viaWrapper ? "via" : "direct")" },
			["codex/direct", "codex/via", "claude/direct", "claude/via"]
		)
		XCTAssertTrue(complete.data.usage.presentation.available)
		XCTAssertEqual(complete.data.usage.presentation.scopes.first?.client, "all")

        XCTAssertTrue(partial.partial)
        XCTAssertFalse(partial.warnings.isEmpty)
        XCTAssertFalse(partial.data.provider.available)
        XCTAssertTrue(partial.data.health.available)
		XCTAssertTrue(partial.data.provider.candidates.isEmpty)
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
		XCTAssertTrue(legacy.data.provider.candidates.isEmpty)
		XCTAssertEqual(legacy.data.usage.presentation, .unavailable)
		// A payload that predates the additive session periods decodes as an
		// unavailable family, not as an error: the change raises no wire version.
		XCTAssertEqual(legacy.data.sessions.periods, .unavailable)
		XCTAssertTrue(legacy.data.sessions.available)
	}

	func testMissingAdditiveHourlyRhythmHoverAndSessionProjectFamiliesDefaultWithoutRaisingTheWireVersion() throws {
		let complete = try desktopFixtureData("snapshot-complete.json")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
		var data = try XCTUnwrap(object["data"] as? [String: Any])
		var usage = try XCTUnwrap(data["usage"] as? [String: Any])
		var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
		var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
		for index in scopes.indices {
			scopes[index].removeValue(forKey: "hourly")
			var rhythm = try XCTUnwrap(scopes[index]["rhythm"] as? [String: Any])
			let intensities = try XCTUnwrap(rhythm.removeValue(forKey: "intensities") as? [Int])
			rhythm.removeValue(forKey: "tokens")
			rhythm.removeValue(forKey: "provider_costs")
			rhythm.removeValue(forKey: "cost_incomplete")
			rhythm["cells"] = intensities.enumerated().map { index, intensity in
				["weekday": index / 24, "hour": index % 24, "intensity": intensity]
			}
			scopes[index]["rhythm"] = rhythm
		}
		presentation["scopes"] = scopes
		usage["presentation"] = presentation
		data["usage"] = usage
		var sessions = try XCTUnwrap(data["sessions"] as? [String: Any])
		var periods = try XCTUnwrap(sessions["periods"] as? [String: Any])
		var periodItems = try XCTUnwrap(periods["items"] as? [[String: Any]])
		for index in periodItems.indices {
			periodItems[index].removeValue(forKey: "projects")
		}
		periods["items"] = periodItems
		sessions["periods"] = periods
		data["sessions"] = sessions
		object["data"] = data

		let decoded = try decodeDesktopWireEnvelopeV1(JSONSerialization.data(withJSONObject: object))
		XCTAssertTrue(decoded.data.usage.presentation.scopes.allSatisfy { $0.hourly == nil })
		XCTAssertTrue(decoded.data.usage.presentation.scopes.flatMap(\.rhythm.cells).allSatisfy {
			$0.tokens == nil && $0.providerCost == nil && $0.costIncomplete == nil
		})
		XCTAssertTrue(decoded.data.sessions.periods.items.allSatisfy { $0.projects.isEmpty })
	}

	func testPresentMalformedAdditiveFieldsAreRejected() throws {
		let complete = try desktopFixtureData("snapshot-complete.json")
		for mutation in ["candidates", "options", "presentation", "hourly", "rhythm-cost"] {
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
			var data = try XCTUnwrap(object["data"] as? [String: Any])
			switch mutation {
			case "candidates":
				var provider = try XCTUnwrap(data["provider"] as? [String: Any])
				provider["candidates"] = "not-an-array"
				data["provider"] = provider
			case "options":
				var provider = try XCTUnwrap(data["provider"] as? [String: Any])
				var candidates = try XCTUnwrap(provider["candidates"] as? [[String: Any]])
				candidates[0]["options"] = "not-an-array"
				provider["candidates"] = candidates
				data["provider"] = provider
			case "presentation":
				var usage = try XCTUnwrap(data["usage"] as? [String: Any])
				usage["presentation"] = "not-an-object"
				data["usage"] = usage
			case "hourly":
				var usage = try XCTUnwrap(data["usage"] as? [String: Any])
				var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
				var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
				scopes[0]["hourly"] = "not-an-object"
				presentation["scopes"] = scopes
				usage["presentation"] = presentation
				data["usage"] = usage
			default:
				var usage = try XCTUnwrap(data["usage"] as? [String: Any])
				var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
				var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
				var rhythm = try XCTUnwrap(scopes[0]["rhythm"] as? [String: Any])
				rhythm["provider_costs"] = ["not": "an-array"]
				scopes[0]["rhythm"] = rhythm
				presentation["scopes"] = scopes
				usage["presentation"] = presentation
				data["usage"] = usage
			}
			object["data"] = data
			let malformed = try JSONSerialization.data(withJSONObject: object)
			XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(malformed), mutation)
		}
	}

	func testHourlyFamilyRejectsNullPartialOutOfRangeDuplicateDescendingAndPostBoundaryShapes() throws {
		let complete = try desktopFixtureData("snapshot-complete.json")
		for mutation in ["null", "missing-boundary", "partial", "out-of-range", "duplicate", "descending", "post-boundary", "unavailable-with-items"] {
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
			var data = try XCTUnwrap(object["data"] as? [String: Any])
			var usage = try XCTUnwrap(data["usage"] as? [String: Any])
			var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
			var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
			if mutation == "null" {
				scopes[0]["hourly"] = NSNull()
			} else {
				var hourly = try XCTUnwrap(scopes[0]["hourly"] as? [String: Any])
				var items = try XCTUnwrap(hourly["items"] as? [[String: Any]])
				switch mutation {
				case "missing-boundary": hourly.removeValue(forKey: "through_hour")
				case "partial": items.removeLast()
				case "out-of-range": items[0]["hour"] = 24
				case "duplicate": items[1]["hour"] = 0
				case "descending": items.swapAt(1, 2)
				case "post-boundary": hourly["through_hour"] = 9
				default: hourly["available"] = false
				}
				hourly["items"] = items
				scopes[0]["hourly"] = hourly
			}
			presentation["scopes"] = scopes
			usage["presentation"] = presentation
			data["usage"] = usage
			object["data"] = data
			XCTAssertThrowsError(
				try decodeDesktopWireEnvelopeV1(JSONSerialization.data(withJSONObject: object)),
				mutation
			)
		}
	}

	// The canonical fixtures are producer output. These assertions pin the fixed
	// collection bounds the contract states, so a payload the producer cannot
	// emit cannot satisfy this decoder gate.
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
			let hourly = try XCTUnwrap(scope.hourly, scope.client)
			XCTAssertTrue(hourly.available, scope.client)
			XCTAssertEqual(hourly.throughHour, 10, scope.client)
			XCTAssertEqual(hourly.items.map(\.hour), Array(0 ... 10), scope.client)
			XCTAssertEqual(scope.rhythm.cells.count, 168, scope.client)
			XCTAssertEqual(scope.rhythm.intensities.count, 168, scope.client)
			XCTAssertEqual(scope.rhythm.tokens.count, 168, scope.client)
			XCTAssertEqual(scope.rhythm.providerCosts.count, 168, scope.client)
			XCTAssertEqual(scope.rhythm.costIncomplete.count, 168, scope.client)
			XCTAssertEqual(scope.pricing.items.map(\.period), ["today", "7d", "30d"], scope.client)
			XCTAssertEqual(Set(scope.quality.items.map(\.period)), ["today", "7d", "30d"], scope.client)
			// Copying today's figures into the wider periods must be visible.
			let totals = scope.periods.items.map(\.totals.tokens)
			XCTAssertNotEqual(totals[0], totals[1], scope.client)
			XCTAssertNotEqual(totals[1], totals[2], scope.client)
		}
		let hourEight = try XCTUnwrap(presentation.scopes[0].hourly?.items.first { $0.hour == 8 })
		XCTAssertEqual(hourEight.value.tokens, 1_440)
		XCTAssertEqual(hourEight.value.events, 1)
		XCTAssertFalse(hourEight.value.providerCost.isEmpty)
		let rhythmHourEight = try XCTUnwrap(
			presentation.scopes[0].rhythm.cells.first { $0.weekday == 3 && $0.hour == 8 }
		)
		XCTAssertEqual(rhythmHourEight.tokens, 1_440)
		XCTAssertNotNil(rhythmHourEight.providerCost)
		XCTAssertNotNil(rhythmHourEight.costIncomplete)
		XCTAssertLessThan(
			try JSONEncoder().encode(complete).count,
			128 * 1024,
			"the compact complete bounded snapshot must preserve the 128 KiB contract budget"
		)
		let todayProjects = try XCTUnwrap(
			complete.data.sessions.periods.items.first { $0.period == "today" && $0.client == "all" }?.projects
		)
		XCTAssertFalse(todayProjects.isEmpty)
		XCTAssertGreaterThan(todayProjects[0].durationSeconds, 0)

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
