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

	// Codex PR #5 P2 / ux/widget-quota.md "The countdown lives on the label
	// line": quota rows rendered only the label and percentage, dropping
	// resets_at entirely. quotaResetETA is the compact (never-translated)
	// span that belongs on that line.
	func testQuotaResetETAUsesCompactVendorUnits() {
		let now = Date(timeIntervalSince1970: 10_000)
		func at(_ seconds: TimeInterval) -> String { ISO8601DateFormatter().string(from: now.addingTimeInterval(seconds)) }

		XCTAssertEqual(quotaResetETA(at(45 * 60), now: now), "45m")
		XCTAssertEqual(quotaResetETA(at(59 * 60 + 59), now: now), "59m")
		XCTAssertEqual(quotaResetETA(at(2 * 3600), now: now), "2h")
		XCTAssertEqual(quotaResetETA(at(23 * 3600 + 59 * 60), now: now), "23h")
		XCTAssertEqual(quotaResetETA(at(3 * 86400), now: now), "3d")
		XCTAssertEqual(quotaResetETA(at(3 * 86400 + 5 * 3600), now: now), "3d5h")
		XCTAssertNil(quotaResetETA(at(-60), now: now), "a reset already in the past must not show a countdown")
		XCTAssertNil(quotaResetETA("not a timestamp", now: now))
	}

	// Codex PR #5 P2: the large family's presentedQuotaClients filtered out
	// every client whose windows were empty, including the legitimate
	// reading-off case (failure == .probe_disabled), leaving the frame with
	// no clients to show instead of QuotaWidgetView's own "Not read" branch.
	func testPresentedQuotaClientsKeepsReadingOffClientsVisibleOnLarge() throws {
		let snapshot = try snapshotWithQuotaReadingOff(widgetFixture("snapshot-complete"))
		let entry = AgentDeckWidgetEntry(
			date: Date(), snapshot: snapshot, kind: .quota,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		let shown = model.presentedQuotaClients(family: .systemLarge)
		XCTAssertEqual(Set(shown.map(\.client)), ["codex", "claude"])
		XCTAssertTrue(shown.allSatisfy { $0.failure == .probeDisabled })
	}

	// Codex PR #5 P1: small/medium narrow an .all-configured widget to one
	// client, but the header kept labeling it "All clients" -- a default
	// widget claimed to show everything while displaying only Codex.
	func testQuotaScopeClientNamesTheOneClientActuallyShownOnSmallAndMedium() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let entry = AgentDeckWidgetEntry(
			date: Date(), snapshot: snapshot, kind: .quota,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		XCTAssertEqual(model.quotaScopeClient(family: .systemSmall), .codex)
		XCTAssertEqual(model.quotaScopeClient(family: .systemMedium), .codex)
		XCTAssertEqual(model.quotaScopeClient(family: .systemLarge), .all, "large already shows both clients, each labeled in its own block")
	}
}
