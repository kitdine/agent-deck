import Foundation
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

		XCTAssertEqual(quotaResetETA(at(30), now: now), "<1m", "Int truncation must not round a sub-minute reset down to 0m")
		XCTAssertNil(quotaResetETA(at(-30), now: now), "Int truncation toward zero must not round a reset up to 59 seconds in the past to 0 and let it slip past the guard")
		XCTAssertEqual(quotaResetETA(at(45 * 60), now: now), "45m")
		XCTAssertEqual(quotaResetETA(at(59 * 60 + 59), now: now), "59m")
		XCTAssertEqual(quotaResetETA(at(2 * 3600), now: now), "2h")
		XCTAssertEqual(quotaResetETA(at(23 * 3600 + 59 * 60), now: now), "23h")
		XCTAssertEqual(quotaResetETA(at(3 * 86400), now: now), "3d")
		XCTAssertEqual(quotaResetETA(at(3 * 86400 + 5 * 3600), now: now), "3d5h")
		XCTAssertNil(quotaResetETA(at(-60), now: now), "a reset already in the past must not show a countdown")
		XCTAssertNil(quotaResetETA("not a timestamp", now: now))
	}

	// Codex PR #5 ninth review, P2: a flat "%.0f%%" rounded 89.6 up to a
	// displayed "90%" that had not actually crossed the 90% threshold the
	// progress bar's tint still correctly evaluates against the raw value.
	func testQuotaPercentTextPreservesPrecisionAtThresholds() {
		XCTAssertEqual(quotaPercentText(64), "64%")
		XCTAssertEqual(quotaPercentText(89.6), "89.6%")
		XCTAssertEqual(quotaPercentText(74.6), "74.6%")
		XCTAssertEqual(quotaPercentText(90), "90%")
	}

	// Codex PR #5 second review, P2: a Codex limit's primary and secondary
	// windows share the same vendor label, so the label alone cannot tell a
	// 5-hour row from a 7-day row for the same limit.
	func testQuotaWindowNameAppendsTheSpanRatherThanReplacingItWithTheVendorLabel() throws {
		func window(label: String?, minutes: Int?) throws -> DesktopSubscriptionWindowV1 {
			try JSONDecoder().decode(
				DesktopSubscriptionWindowV1.self,
				from: JSONSerialization.data(withJSONObject: [
					"key": "codex", "label": label as Any, "window_minutes": minutes as Any,
					"window_minutes_reason": NSNull(), "used_percent": 12, "resets_at": NSNull(),
				])
			)
		}
		XCTAssertEqual(
			quotaWindowName(try window(label: "GPT-5.3-Codex-Spark", minutes: 300)),
			"GPT-5.3-Codex-Spark · " + WidgetCopy.text("5h window")
		)
		XCTAssertEqual(quotaWindowName(try window(label: nil, minutes: 300)), WidgetCopy.text("5h window"))
	}

	// Codex PR #5 fifth review, P2: the large widget's per-client rows
	// rendered the same "No quota window to show" text regardless of why a
	// client had no windows, so two unavailable clients with different
	// reasons (not applicable vs. never probed vs. a successful-but-empty
	// response) were indistinguishable. quotaClientReason must resolve each
	// client's own reason, including the notReported/neverProbed split on
	// whether a successful observation ever landed.
	func testQuotaClientReasonDistinguishesEachUnavailableReason() throws {
		func client(applicable: Bool, applicableReason: String?, observedAt: String?, failure: String?) throws -> DesktopSubscriptionClientV1 {
			try JSONDecoder().decode(
				DesktopSubscriptionClientV1.self,
				from: JSONSerialization.data(withJSONObject: [
					"client": "codex", "applicable": applicable, "applicable_reason": applicableReason as Any,
					"source": observedAt == nil ? NSNull() : "codex_app_server",
					"observed_at": observedAt as Any, "stale": false, "attribution_confirmed": true,
					"plan": NSNull(), "plan_reason": NSNull(), "windows": [],
					"tightest_window_key": NSNull(), "reset_allowance": NSNull(),
					"reset_allowance_reason": NSNull(), "observed_reset_at": NSNull(), "failure": failure as Any,
				])
			)
		}

		let notOfficial = try client(applicable: false, applicableReason: "not_official", observedAt: nil, failure: nil)
		XCTAssertEqual(quotaClientReason(notOfficial), .notOfficial)

		let probeDisabled = try client(applicable: true, applicableReason: nil, observedAt: nil, failure: "probe_disabled")
		XCTAssertEqual(quotaClientReason(probeDisabled), .probeDisabled)

		let successfulButEmpty = try client(applicable: true, applicableReason: nil, observedAt: "2026-09-18T02:00:00Z", failure: nil)
		XCTAssertEqual(quotaClientReason(successfulButEmpty), .notReported, "a successful, empty observation must not read as never probed")

		let neverProbed = try client(applicable: true, applicableReason: nil, observedAt: nil, failure: nil)
		XCTAssertEqual(quotaClientReason(neverProbed), .neverProbed)

		XCTAssertEqual(quotaReasonText(.notOfficial), WidgetCopy.text("Not applicable"))
		XCTAssertEqual(quotaReasonText(.probeDisabled), WidgetCopy.text("Not read"))
		XCTAssertEqual(quotaReasonText(.notReported), WidgetCopy.text("Data unavailable"))
		XCTAssertEqual(quotaReasonText(.neverProbed), WidgetCopy.text("Data unavailable"))
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
