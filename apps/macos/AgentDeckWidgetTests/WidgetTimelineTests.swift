import XCTest

final class WidgetTimelineTests: XCTestCase {
	func testPlaceholderContainsNoSnapshotOrRealValue() {
		let loader = WidgetSnapshotLoader(readSnapshot: { fatalError("placeholder must not read") })
		let entry = loader.entry(kind: .magnitude, client: .all, period: .today, now: Date(), placeholder: true)

		XCTAssertNil(entry.snapshot)
		XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .placeholder)
	}

	func testRefreshAfterClampsToFifteenAndSixtyMinutes() {
		let now = Date(timeIntervalSince1970: 10_000)
		let below = ISO8601DateFormatter().string(from: now.addingTimeInterval(60))
		let above = ISO8601DateFormatter().string(from: now.addingTimeInterval(2 * 60 * 60))

		XCTAssertEqual(WidgetTimelinePolicy.refreshDate(suggestedAt: below, now: now), now.addingTimeInterval(15 * 60))
		XCTAssertEqual(WidgetTimelinePolicy.refreshDate(suggestedAt: above, now: now), now.addingTimeInterval(60 * 60))
		XCTAssertEqual(WidgetTimelinePolicy.refreshDate(suggestedAt: "malformed", now: now), now.addingTimeInterval(60 * 60))
	}

	func testUnsupportedOrMalformedReadRendersUnavailable() {
		let loader = WidgetSnapshotLoader(readSnapshot: {
			throw WidgetSnapshotError.unsupportedSchemaVersion(2)
		})
		let entry = loader.entry(kind: .trust, client: .all, period: .today, now: Date())

		XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .unavailable)
	}

	func testConfiguredMissingClientRendersUnavailableWithoutChangingConfiguration() throws {
		let snapshot = try snapshotWithoutClient(widgetFixture("snapshot-complete"), client: "claude")
		let entry = AgentDeckWidgetEntry(
			date: Date(), snapshot: snapshot, kind: .composition,
			client: .claude, period: .sevenDays, isPlaceholder: false
		)

		XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .unavailable)
		XCTAssertEqual(entry.client, .claude)
		XCTAssertEqual(entry.period, .sevenDays)
	}

	func testAgingOldAndPartialQualifiersUseFixedOrder() throws {
		let partial = try snapshotWithPartial(widgetFixture("snapshot-complete"))
		let generated = try XCTUnwrap(WidgetTimelinePolicy.date(partial.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: generated.addingTimeInterval(7 * 60 * 60), snapshot: partial,
			kind: .magnitude, client: .all, period: .today, isPlaceholder: false
		)

		XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).qualifiers.prefix(2), [.partial, .old])
	}
}
