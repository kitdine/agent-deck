import AppKit
import SwiftUI
import WidgetKit
import XCTest

final class WidgetPresentationTests: XCTestCase {
	func testAllFifteenKindSizeConfigurationsUseThePrototypeComposition() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let families: [WidgetFamily] = [.systemSmall, .systemMedium, .systemLarge]
		var configurations = 0

		for kind in AgentDeckWidgetKind.allCases {
			for family in families {
				XCTAssertEqual(
					WidgetLayoutContract.sections(kind, family: family),
					expectedSections(kind, family: family),
					"\(kind) \(family) must follow the matching Widgets.jsx size branch"
				)
				let entry = AgentDeckWidgetEntry(
					date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
					kind: kind, client: .all, period: .today, isPlaceholder: false
				)
				XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .data)
				configurations += 1
			}
		}

		XCTAssertEqual(configurations, 15)
		XCTAssertEqual(
			[.systemSmall, .systemMedium, .systemLarge].map(WidgetLayoutContract.bucketCount),
			[7, 20, 90]
		)
		XCTAssertEqual(WidgetLayoutContract.canvas(.systemLarge).width, WidgetLayoutContract.canvas(.systemMedium).width)
		XCTAssertGreaterThan(WidgetLayoutContract.canvas(.systemLarge).height, WidgetLayoutContract.canvas(.systemMedium).height * 2)
	}

	func testPrototypeNumberFormattingDropsFalsePrecision() {
		XCTAssertEqual(WidgetFormat.share("100.00"), "100%")
		XCTAssertEqual(WidgetFormat.share("59.24"), "59.2%")
		XCTAssertEqual(WidgetFormat.share(nil), "—")
		XCTAssertEqual(WidgetFormat.percentage(59, total: 100), "59.00")
		let symbols = DateFormatter().standaloneWeekdaySymbols!
		XCTAssertEqual(WidgetFormat.weekday("monday"), symbols[1])
		XCTAssertEqual(WidgetFormat.weekday("sunday"), symbols[0])
	}

	func testSmallQuotaWidgetSelectsTheConfiguredClientsHighestUsedWindow() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .codex, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)
		XCTAssertEqual(model.quotaClients.count, 1)
		let client = model.quotaClients[0]
		let selectedWindows = model.quotaWindows(for: client, family: .systemSmall)
		XCTAssertEqual(selectedWindows.count, 1)
		let selected = selectedWindows[0]

		XCTAssertEqual(selected.usedPercent, client.windows.map(\.usedPercent).max())
	}

	@MainActor
	func testLargeQuotaWidgetUsesTwoVerticalCenteredEqualHeightSlotsAndSingleClientUsesNoEmptySlot() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)
		let clients = model.presentedQuotaClients(family: .systemLarge)

		XCTAssertEqual(clients.map(\.client), ["codex", "claude"])
		XCTAssertEqual(
			QuotaWidgetLayoutContract.presentation(family: .systemLarge, clientCount: clients.count),
			QuotaWidgetLayoutContract(axis: .vertical, equalHeightSlots: 2)
		)
		XCTAssertEqual(
			QuotaWidgetLayoutContract.presentation(family: .systemLarge, clientCount: 1),
			QuotaWidgetLayoutContract(axis: .single, equalHeightSlots: 1)
		)

		let capture = QuotaGeometryCapture()
		let size = WidgetLayoutContract.canvas(.systemLarge)
		let view = QuotaGeometryProbe(
			content: AgentDeckWidgetView(entry: entry, familyOverride: .systemLarge)
				.frame(width: size.width, height: size.height),
			capture: capture
		)
		_ = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
		let codexSlot = try XCTUnwrap(capture.frames["slot.codex"])
		let claudeSlot = try XCTUnwrap(capture.frames["slot.claude"])
		let codexContent = try XCTUnwrap(capture.frames["content.codex"])
		let claudeContent = try XCTUnwrap(capture.frames["content.claude"])
		XCTAssertEqual(codexSlot.height, claudeSlot.height, accuracy: 1)
		XCTAssertLessThan(codexContent.height, codexSlot.height, "centering assertion must have spare vertical space")
		XCTAssertLessThan(claudeContent.height, claudeSlot.height, "centering assertion must have spare vertical space")
		XCTAssertEqual(codexContent.midY, codexSlot.midY, accuracy: 1)
		XCTAssertEqual(claudeContent.midY, claudeSlot.midY, accuracy: 1)
	}

	@MainActor
	func testSingleClientQuotaWidgetsCenterContentWithinTheirWholeBody() throws {
		let original = try widgetFixture("snapshot-complete")
		for configured in [WidgetClient.codex, .claude] {
			for windowCount in [1, 2] {
				var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
				var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
				let clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
				var client = try XCTUnwrap(clients.first { $0["client"] as? String == configured.rawValue })
				let windows = try XCTUnwrap(client["windows"] as? [[String: Any]])
				XCTAssertGreaterThanOrEqual(windows.count, windowCount)
				client["windows"] = Array(windows.prefix(windowCount))
				client["tightest_window_key"] = windows[0]["key"]
				subscription["clients"] = [client]
				object["subscription"] = subscription
				let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
				let entry = AgentDeckWidgetEntry(
					date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
					kind: .quota, client: configured, period: .today, isPlaceholder: false
				)
				for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
					let context = "\(configured.rawValue) \(familyName(family)) \(windowCount) windows"
					let model = WidgetSurfaceModel(entry: entry, now: entry.date)
					XCTAssertEqual(model.presentedQuotaClients(family: family).map(\.client), [configured.rawValue], context)
					let capture = QuotaGeometryCapture()
					let size = WidgetLayoutContract.canvas(family)
					let view = QuotaGeometryProbe(
						content: AgentDeckWidgetView(entry: entry, familyOverride: family)
							.frame(width: size.width, height: size.height), capture: capture
					)
					let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
					let slot = try XCTUnwrap(capture.frames["slot.\(configured.rawValue)"], context)
					let content = try XCTUnwrap(capture.frames["content.\(configured.rawValue)"], context)
					XCTAssertEqual(
						capture.frames.keys.filter {
							($0.hasPrefix("slot.") || $0.hasPrefix("content.")) && !$0.hasSuffix(".absolute")
						}.count, 2,
						"single client must have exactly one slot and content: \(context)"
					)
					XCTAssertGreaterThan(slot.height, size.height / 2, "single slot must use the full body: \(context)")
					XCTAssertLessThan(content.height, slot.height, "centering assertion must have spare space: \(context)")
					XCTAssertEqual(content.midY, slot.midY, accuracy: 1, context)
					let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
					attachment.name = "Centered quota — \(context)"
					attachment.lifetime = .keepAlways
					add(attachment)
				}
			}
		}
	}

	@MainActor
	func testOverflowingSingleQuotaWidgetKeepsLeadingContentAtBodyTop() throws {
		let original = try widgetFixture("snapshot-complete")
		for configured in [WidgetClient.codex, .claude] {
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
			var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
			let clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
			var client = try XCTUnwrap(clients.first { $0["client"] as? String == configured.rawValue })
			let template = try XCTUnwrap((client["windows"] as? [[String: Any]])?.first)
			client["windows"] = (0 ..< 12).map { index in
				var window = template
				window["key"] = "additional-\(index)"
				return window
			}
			client["tightest_window_key"] = "additional-0"
			subscription["clients"] = [client]
			object["subscription"] = subscription
			let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
			let entry = AgentDeckWidgetEntry(
				date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
				kind: .quota, client: configured, period: .today, isPlaceholder: false
			)
			for family in [WidgetFamily.systemMedium, .systemLarge] {
				let context = "\(configured.rawValue) \(familyName(family)) overflowing windows"
				let model = WidgetSurfaceModel(entry: entry, now: entry.date)
				let presented = try XCTUnwrap(model.presentedQuotaClients(family: family).first)
				XCTAssertEqual(model.quotaWindows(for: presented, family: family).count, 12)
				let capture = QuotaGeometryCapture()
				let size = WidgetLayoutContract.canvas(family)
				let view = QuotaGeometryProbe(
					content: AgentDeckWidgetView(entry: entry, familyOverride: family)
						.frame(width: size.width, height: size.height)
						.background(GeometryReader { proxy in
							Color.clear.preference(
								key: QuotaWidgetGeometryPreferenceKey.self,
								value: ["canvas": proxy.frame(in: .global)]
							)
						}), capture: capture
				)
				_ = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				let canvas = try XCTUnwrap(capture.frames["canvas"], context)
				let content = try XCTUnwrap(capture.frames["content.\(configured.rawValue).absolute"], context)
				let slot = try XCTUnwrap(capture.frames["slot.\(configured.rawValue).absolute"], context)
				XCTAssertGreaterThan(content.height, canvas.height, "fixture must exceed the card: \(context)")
				XCTAssertLessThan(slot.height, canvas.height, "body must leave room for header/footer: \(context)")
				XCTAssertEqual(content.minY, slot.minY, accuracy: 1, "overflow must start at its own body top: \(context)")
				XCTAssertGreaterThanOrEqual(content.minY, canvas.minY, "leading rows must remain in the card: \(context), content=\(content), canvas=\(canvas)")
			}
		}
	}

	@MainActor
	func testClaudeMediumOverflowKeepsAttributionVisible() throws {
		let original = try widgetFixture("snapshot-complete")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
		var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
		let clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
		var client = try XCTUnwrap(clients.first { $0["client"] as? String == "claude" })
		let template = try XCTUnwrap((client["windows"] as? [[String: Any]])?.first)
		client["windows"] = (0 ..< 3).map { index in
			var window = template
			window["key"] = "additional-\(index)"
			return window
		}
		client["tightest_window_key"] = "additional-0"
		client["attribution_confirmed"] = false
		subscription["clients"] = [client]
		object["subscription"] = subscription
		let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .claude, period: .today, isPlaceholder: false
		)
		let capture = QuotaGeometryCapture()
		let size = WidgetLayoutContract.canvas(.systemMedium)
		let view = QuotaGeometryProbe(
			content: AgentDeckWidgetView(entry: entry, familyOverride: .systemMedium)
				.frame(width: size.width, height: size.height)
				.background(GeometryReader { proxy in
					Color.clear.preference(key: QuotaWidgetGeometryPreferenceKey.self, value: ["canvas": proxy.frame(in: .global)])
				}), capture: capture
		)
		let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
		let attribution = try XCTUnwrap(capture.frames["attribution.claude.absolute"])
		let canvas = try XCTUnwrap(capture.frames["canvas"])
		let slot = try XCTUnwrap(capture.frames["slot.claude.absolute"])
		XCTAssertGreaterThanOrEqual(attribution.minY, canvas.minY, "attribution=\(attribution), canvas=\(canvas)")
		XCTAssertLessThanOrEqual(attribution.maxY, canvas.maxY, "attribution=\(attribution), canvas=\(canvas)")
		XCTAssertGreaterThanOrEqual(attribution.minY, slot.minY, "attribution must be inside clipped body")
		XCTAssertLessThanOrEqual(attribution.maxY, slot.maxY, "attribution must be inside clipped body")
		let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
		attachment.name = "Claude medium three-window attribution"
		attachment.lifetime = .keepAlways
		add(attachment)
	}

	func testLargeQuotaWidgetRetainsEveryUnavailableClientReason() throws {
		let original = try widgetFixture("snapshot-complete")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
		var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
		var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
		for index in clients.indices {
			clients[index]["windows"] = []
			clients[index]["tightest_window_key"] = NSNull()
			if clients[index]["client"] as? String == "codex" {
				clients[index]["applicable"] = false
				clients[index]["applicable_reason"] = "not_official"
				clients[index]["failure"] = NSNull()
			} else {
				clients[index]["applicable"] = true
				clients[index]["applicable_reason"] = NSNull()
				clients[index]["failure"] = "parse_failed"
			}
		}
		subscription["clients"] = clients
		object["subscription"] = subscription
		let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .all, period: .today, isPlaceholder: false
		)
		let shown = WidgetSurfaceModel(entry: entry, now: entry.date).presentedQuotaClients(family: .systemLarge)

		XCTAssertEqual(shown.map(\.client), ["codex", "claude"])
		XCTAssertEqual(shown[0].applicableReason, .notOfficial)
		XCTAssertEqual(shown[1].failure, .parseFailed)
	}

	// Codex PR #5 eighth review, P2: isEmpty's quota branch used
	// quotaClients, narrowed to the configured single client, even on the
	// large family, where presentedQuotaClients shows both. With the
	// configured client empty but the other client still carrying real
	// windows, the large widget rendered those figures while also claiming
	// "No activity."
	func testQuotaEmptyQualifierReflectsThePresentedFamilyNotTheConfiguredClient() throws {
		let original = try widgetFixture("snapshot-complete")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
		var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
		var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
		for index in clients.indices where clients[index]["client"] as? String == "codex" {
			clients[index]["windows"] = []
			clients[index]["tightest_window_key"] = NSNull()
		}
		subscription["clients"] = clients
		object["subscription"] = subscription
		let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .codex, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		XCTAssertTrue(model.qualifiers(family: .systemSmall).contains(.empty), "small narrows to the configured, now-empty client: genuinely empty")
		XCTAssertFalse(model.qualifiers(family: .systemLarge).contains(.empty), "large also shows Claude's real windows and must not also claim no activity")
	}

	func testQuotaFooterUsesOldestDisplayedObservationRatherThanFreshSnapshotTime() throws {
		let original = try widgetFixture("snapshot-complete")
		let snapshot = try snapshotWithQuotaObservedAt(original, values: [
			"codex": "2026-08-13T09:40:00Z",
			"claude": "2026-08-13T09:50:00Z",
		])
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		XCTAssertEqual(model.quotaFooterObservedAt(family: .systemLarge), "2026-08-13T09:40:00Z")
		XCTAssertNotEqual(model.quotaFooterObservedAt(family: .systemLarge), snapshot.generatedAt)
	}

	// Codex PR #5 second review, P1: after a partial update leaves one
	// client's windows at different ages, the footer must date the window
	// actually displayed on small (the tightest one), not whichever window
	// happens to be newest -- even when that newer window is not shown.
	func testQuotaFooterDatesTheDisplayedWindowNotAnUndisplayedNewerOne() throws {
		let original = try widgetFixture("snapshot-complete")
		// codex's tightest window (used 79%, small's only displayed window)
		// becomes the oldest; the other codex window -- never shown on
		// small -- becomes the newest. The footer must still report the
		// tightest window's own (older) instant.
		let snapshot = try snapshotWithWindowObservedAt(
			snapshotWithWindowObservedAt(original, client: "codex", windowKey: "codex", value: "2026-08-13T08:00:00Z"),
			client: "codex", windowKey: "codex_bengalfox", value: "2026-08-13T11:00:00Z"
		)
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .codex, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		XCTAssertEqual(model.quotaWindows(for: model.quotaClients[0], family: .systemSmall).first?.key, "codex")
		XCTAssertEqual(model.quotaFooterObservedAt(family: .systemSmall), "2026-08-13T08:00:00Z")
	}

	// Codex PR #5 second review, P2: the wire resolves the tightest window
	// once, from vendor order; the small widget must use that selection
	// rather than recomputing its own tie-break by key, which can disagree
	// when two windows share the highest percentage.
	func testSmallQuotaWidgetHonorsTheWiresTightestWindowOnATie() throws {
		let original = try widgetFixture("snapshot-complete")
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
		var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
		var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
		let codexIndex = try XCTUnwrap(clients.firstIndex { ($0["client"] as? String) == "codex" })
		var windows = try XCTUnwrap(clients[codexIndex]["windows"] as? [[String: Any]])
		// Tie both windows at the same used_percent: "codex" sorts before
		// "codex_bengalfox" by key, so a local by-key tie-break would pick
		// "codex" even though tightest_window_key (vendor order) names
		// "codex_bengalfox" here.
		for index in windows.indices { windows[index]["used_percent"] = 50 }
		clients[codexIndex]["windows"] = windows
		clients[codexIndex]["tightest_window_key"] = "codex_bengalfox"
		subscription["clients"] = clients
		object["subscription"] = subscription
		let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		let entry = AgentDeckWidgetEntry(
			date: try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt)), snapshot: snapshot,
			kind: .quota, client: .codex, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)

		XCTAssertEqual(model.quotaWindows(for: model.quotaClients[0], family: .systemSmall).first?.key, "codex_bengalfox")
	}

	func testLargestDynamicTypeDegradesToTheNextFamily() {
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemLarge, dynamicTypeSize: .accessibility5),
			.systemMedium
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemMedium, dynamicTypeSize: .accessibility5),
			.systemSmall
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemSmall, dynamicTypeSize: .accessibility5),
			.systemSmall
		)
		XCTAssertEqual(
			WidgetLayoutContract.presentationFamily(.systemLarge, dynamicTypeSize: .xxLarge),
			.systemLarge
		)
	}

	@MainActor
	func testPrototypeAlignedDarkRenderingsAreAttached() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: now,
					snapshot: snapshot,
					kind: kind,
					client: .all,
					period: prototypePeriod(kind: kind, family: family),
					isPlaceholder: false
				)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.colorScheme, .dark)
					.frame(width: size.width, height: size.height)
					.background(Color(red: 0.071, green: 0.082, blue: 0.110))
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget prototype alignment — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	@MainActor
	func testLargeBottomAnchorsAreAttached() throws {
		let incomplete = try widgetFixture("snapshot-complete")
		let priced = try snapshotWithPricedToday(incomplete, client: "all")
		let size = WidgetLayoutContract.canvas(.systemLarge)

		for (kind, snapshot) in [(AgentDeckWidgetKind.composition, priced), (.trust, incomplete)] {
			let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
			let entry = AgentDeckWidgetEntry(
				date: now,
				snapshot: snapshot,
				kind: kind,
				client: .all,
				period: prototypePeriod(kind: kind, family: .systemLarge),
				isPlaceholder: false
			)
			let view = AgentDeckWidgetView(entry: entry, familyOverride: .systemLarge)
				.environment(\.colorScheme, .dark)
				.frame(width: size.width, height: size.height)
				.background(Color(red: 0.071, green: 0.082, blue: 0.110))
			let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
			XCTAssertGreaterThan(png.count, 1_000)
			let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
			attachment.name = "Widget bottom anchor — \(kind.rawValue) large"
			attachment.lifetime = XCTAttachment.Lifetime.keepAlways
			add(attachment)
		}
	}

	@MainActor
	func testLargestDynamicTypeRenderingsAreAttached() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: now,
					snapshot: snapshot,
					kind: kind,
					client: .all,
					period: prototypePeriod(kind: kind, family: family),
					isPlaceholder: false
				)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.dynamicTypeSize, .accessibility5)
					.environment(\.colorScheme, .light)
					.frame(width: size.width, height: size.height)
					.background(Color.white)
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget largest Dynamic Type — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	@MainActor
	func testGalleryPlaceholderRenderingsContainNoSnapshotValues() throws {
		for kind in AgentDeckWidgetKind.allCases {
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				let entry = AgentDeckWidgetEntry(
					date: Date(timeIntervalSince1970: 0),
					snapshot: nil,
					kind: kind,
					client: .all,
					period: .today,
					isPlaceholder: true
				)
				XCTAssertNil(entry.snapshot)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.colorScheme, .light)
					.frame(width: size.width, height: size.height)
					.background(Color.white)
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget gallery placeholder — \(kind.rawValue) \(familyName(family))"
				attachment.lifetime = XCTAttachment.Lifetime.keepAlways
				add(attachment)
			}
		}
	}

	@MainActor
	func testTypedFailureRenderingsCoverEveryKindFamilyAndTheme() throws {
		let failures: [WidgetLoadFailure] = [
			.missing,
			.containerUnavailable,
			.unreadable(category: .decode),
			.unsupportedSchemaVersion(found: 2),
		]
		for failure in failures {
			for kind in AgentDeckWidgetKind.allCases {
				for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
					for (scheme, dynamicType) in [
						(ColorScheme.light, DynamicTypeSize.large),
						(.dark, .accessibility5),
					] {
						let entry = AgentDeckWidgetEntry(
							date: Date(timeIntervalSince1970: 42_000),
							outcome: .failed(failure),
							kind: kind,
							client: .all,
							period: prototypePeriod(kind: kind, family: family)
						)
						XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .unavailable)
						let size = WidgetLayoutContract.canvas(family)
						let capture = WidgetFailureRenderCapture()
						let content = AgentDeckWidgetView(
								entry: entry,
								familyOverride: family,
								reduceMotionOverride: true
							)
							.environment(\.colorScheme, scheme)
							.environment(\.dynamicTypeSize, dynamicType)
							.frame(width: size.width, height: size.height)
							.background(scheme == .dark ? Color.black : Color.white)
						let view = WidgetFailureRenderProbe(content: content, capture: capture)
						let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
						let image = try XCTUnwrap(NSBitmapImageRep(data: png))
						XCTAssertEqual(image.size.width, size.width)
						XCTAssertEqual(image.size.height, size.height)

						let expectedSemantics = WidgetFailureSurfaceSemantics(
							entry: entry, family: WidgetLayoutContract.presentationFamily(family, dynamicTypeSize: dynamicType), failure: failure
						)
						XCTAssertEqual(capture.accessibilityNodes, expectedSemantics.accessibilityNodes)
						XCTAssertEqual(Set(capture.frames.keys), Set([.header, .body, .footer]))
						let header = try XCTUnwrap(capture.frames[.header])
						let body = try XCTUnwrap(capture.frames[.body])
						let footer = try XCTUnwrap(capture.frames[.footer])
						let canvas = CGRect(origin: .zero, size: size)
						for frame in [header, body, footer] {
							XCTAssertGreaterThan(frame.width, 0)
							XCTAssertGreaterThan(frame.height, 0)
							XCTAssertTrue(canvas.insetBy(dx: -1, dy: -1).contains(frame), "failure element must remain inside the canvas")
						}
						XCTAssertLessThanOrEqual(header.maxY, body.minY + 1)
						XCTAssertLessThanOrEqual(body.maxY, footer.minY + 1)
						XCTAssertFalse(header.intersects(footer))
					}
				}
			}
		}
	}

	func testTypedFailureSemanticMatrixUsesExactLocalizedAXOrderAndNoMotion() throws {
		let failures: [WidgetLoadFailure] = [
			.missing,
			.containerUnavailable,
			.unreadable(category: .decode),
			.unsupportedSchemaVersion(found: 2),
		]
		let bundle = Bundle(for: WidgetPresentationTests.self)
		for identifier in ["en", "zh-Hans"] {
			let path = try XCTUnwrap(bundle.path(forResource: identifier, ofType: "lproj"))
			let localized = try XCTUnwrap(Bundle(path: path))
			for failure in failures {
				for kind in AgentDeckWidgetKind.allCases {
					for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
						let entry = AgentDeckWidgetEntry(
							date: Date(timeIntervalSince1970: 42_000),
							outcome: .failed(failure),
							kind: kind,
							client: .all,
							period: prototypePeriod(kind: kind, family: family)
						)
						let semantics = WidgetFailureSurfaceSemantics(
							entry: entry, family: family, failure: failure, bundle: localized
						)
						XCTAssertEqual(semantics.accessibilityNodes.map(\.role), [.header, .body, .footer])
						XCTAssertEqual(semantics.accessibilityNodes[1].descriptor.label, semantics.title)
						XCTAssertEqual(semantics.accessibilityNodes[1].descriptor.value, "")
						XCTAssertEqual(semantics.accessibilityNodes[2].descriptor.label, semantics.footer)
						XCTAssertEqual(semantics.accessibilityNodes[2].descriptor.value, "")
						XCTAssertFalse(semantics.accessibilityNodes[0].descriptor.label.isEmpty)
						XCTAssertFalse(semantics.accessibilityNodes[0].descriptor.value.isEmpty)
						XCTAssertFalse(semantics.usesMotion)
					}
				}
			}
		}
	}

	func testChartsAndDerivedFactsReadTheProjectionSeries() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let generated = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: generated, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: generated)
		let expected = try XCTUnwrap(model.scope).daily.items.map { WidgetFormat.chartValue($0.value) }

		XCTAssertEqual(model.chartValues, expected)
		XCTAssertEqual(model.period?.period, "30d")
	}

	func testAccessibilityDescriptorsNameMetricsAndSummarizeCharts() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let generated = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: generated, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)
		let scope = try XCTUnwrap(WidgetSurfaceModel(entry: entry, now: generated).scope)
		let trend = WidgetAccessibility.trend(items: scope.daily.items)

		XCTAssertEqual(trend.label, WidgetCopy.text("Usage trend"))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Date range")))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Peak")))
		XCTAssertTrue(trend.value.contains(WidgetCopy.text("Trend")))

		let hourGrid = WidgetAccessibility.hourGrid(scope.rhythm)
		XCTAssertEqual(hourGrid.label, WidgetCopy.text("Activity by hour"))
		XCTAssertTrue(hourGrid.value.contains(WidgetCopy.text("Date range")))
		XCTAssertTrue(hourGrid.value.contains(WidgetCopy.text("Peak")))

		let provider = WidgetAccessibility.metric(label: "Official", values: ["$1.25", "88.3%"])
		XCTAssertEqual(provider.label, "Official", "the provider name must be announced before its values")
		XCTAssertEqual(provider.value, "$1.25, 88.3%")
	}

	func testAccessibilityTrendDirectionUsesTheRenderedSeriesEndpoints() {
		XCTAssertEqual(WidgetAccessibility.trendDirection([1, 2]), WidgetCopy.text("Rising"))
		XCTAssertEqual(WidgetAccessibility.trendDirection([2, 1]), WidgetCopy.text("Falling"))
		XCTAssertEqual(WidgetAccessibility.trendDirection([2, 2]), WidgetCopy.text("Steady"))
	}

	func testEmptyIsPerWidgetRatherThanProjectionWide() throws {
		let snapshot = try snapshotWithEmptyToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let magnitude = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .magnitude,
			client: .all, period: .today, isPlaceholder: false
		)
		let rhythm = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .rhythm,
			client: .all, period: .thirtyDays, isPlaceholder: false
		)

		XCTAssertTrue(WidgetSurfaceModel(entry: magnitude, now: now).qualifiers.contains(.empty))
		XCTAssertFalse(WidgetSurfaceModel(entry: rhythm, now: now).qualifiers.contains(.empty))
	}

	func testTrustKeepsTheUnattributedAmountWhenPricingIsIncomplete() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .trust,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: now)

		XCTAssertEqual(model.trustTiers.map(\.quality), ["determinable", "inferred", "unattributed"])
		let unattributed = try XCTUnwrap(model.trustTiers.first { $0.quality == "unattributed" })
		XCTAssertEqual(unattributed.amount, .tokens("1.8k"))
		XCTAssertTrue(unattributed.costIncomplete)
		XCTAssertNil(unattributed.share)
		XCTAssertEqual(model.trustTiers.first { $0.quality == "inferred" }?.amount, .cost("$0.00"))

		let headline = try XCTUnwrap(model.trustHeadline)
		XCTAssertEqual(headline.quality, "determinable")
		XCTAssertNil(headline.share, "an unavailable share must not become the small headline")

		let pricing = try XCTUnwrap(model.trustPricing)
		XCTAssertTrue(pricing.incomplete)
		XCTAssertEqual(pricing.coverage, "0.00")
		XCTAssertEqual(pricing.unpricedIdentifiers, ["claude/claude-sonnet-5", "codex/gpt-5"])
		XCTAssertTrue(model.trustCostIncomplete, "every size must state that cost is incomplete")
		XCTAssertFalse(model.qualifiers.contains(.empty), "1800 unattributed tokens are not an empty period")
	}

	func testTrustUsesTheDeterminableShareOnlyWhileItIsMeaningful() throws {
		let snapshot = try snapshotWithPricedToday(widgetFixture("snapshot-complete"), client: "all")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		let entry = AgentDeckWidgetEntry(
			date: now, snapshot: snapshot, kind: .trust,
			client: .all, period: .today, isPlaceholder: false
		)
		let model = WidgetSurfaceModel(entry: entry, now: now)
		let headline = try XCTUnwrap(model.trustHeadline)

		XCTAssertEqual(headline.share, "62.50")
		XCTAssertEqual(headline.amount, .cost("$1.25"))
		XCTAssertFalse(headline.costIncomplete)
		XCTAssertEqual(model.trustTiers.first { $0.quality == "unattributed" }?.amount, .cost("$0.25"))
		XCTAssertFalse(model.trustCostIncomplete)
		XCTAssertEqual(model.trustPricing?.coverage, "100.00")
	}

	private func expectedSections(_ kind: AgentDeckWidgetKind, family: WidgetFamily) -> [String] {
		switch (kind, family) {
		case (.magnitude, .systemSmall): ["headline", "volume", "mini-bars"]
		case (.magnitude, .systemMedium): ["periods", "mini-bars", "date-axis"]
		case (.magnitude, .systemLarge): ["headline", "volume", "periods", "area", "date-axis", "statistics"]
		case (.composition, .systemSmall): ["eyebrow", "top-model", "headline-share", "share-track", "volume"]
		case (.composition, .systemMedium): ["models", "share-tracks"]
		case (.composition, .systemLarge): ["models", "share-tracks", "token-mix", "clients"]
		case (.trust, .systemSmall): ["eyebrow", "headline", "share-track", "support"]
		case (.trust, .systemMedium): ["quality", "share-tracks", "coverage"]
		case (.trust, .systemLarge): ["eyebrow", "headline", "quality", "providers", "pricing-summary"]
		case (.rhythm, .systemSmall): ["eyebrow", "active-days", "busiest"]
		case (.rhythm, .systemMedium): ["hour-axis", "legend", "hour-grid"]
		case (.rhythm, .systemLarge): ["legend", "hour-axis", "hour-grid", "daily-grid", "day-statistics"]
		case (.quota, .systemSmall): ["client", "tightest-window", "attribution"]
		case (.quota, .systemMedium): ["client", "windows", "attribution"]
		case (.quota, .systemLarge): ["codex", "claude", "windows", "attribution"]
		default: []
		}
	}

	private func prototypePeriod(kind: AgentDeckWidgetKind, family: WidgetFamily) -> WidgetPeriod {
		guard kind == .magnitude || kind == .composition else {
			return kind == .rhythm ? .thirtyDays : .today
		}
		return family == .systemLarge ? .thirtyDays : .today
	}

	private func familyName(_ family: WidgetFamily) -> String {
		switch family {
		case .systemSmall: "small"
		case .systemMedium: "medium"
		case .systemLarge: "large"
		default: "other"
		}
	}

	@MainActor
	func testConfiguredQuotaIntentRenderingsAtNativeSizesAreAttached() throws {
		let snapshot = try widgetFixture("snapshot-complete")
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for configured in QuotaWidgetClient.allCases {
			var intent = QuotaWidgetIntent()
			intent.client = configured
			for family in [WidgetFamily.systemSmall, .systemMedium, .systemLarge] {
				for scheme in [ColorScheme.light, .dark] {
					let entry = AgentDeckWidgetEntry(date: now, snapshot: snapshot, kind: .quota,
						client: try XCTUnwrap(intent.client).widgetClient, period: .today, isPlaceholder: false)
					let size = WidgetLayoutContract.canvas(family)
					let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
						.environment(\.colorScheme, scheme)
						.frame(width: size.width, height: size.height)
						.background(scheme == .dark ? Color.black : Color.white)
					let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
					let image = try XCTUnwrap(NSBitmapImageRep(data: png))
					XCTAssertEqual(image.size, NSSize(width: size.width, height: size.height))
					let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
					attachment.name = "Configured quota — \(configured.rawValue) \(familyName(family)) \(scheme)"
					attachment.lifetime = .keepAlways
					add(attachment)
				}
			}
		}
	}

	@MainActor
	private func renderedViewPNG<Content: View>(_ content: Content, size: NSSize) throws -> Data {
		let hosting = NSHostingView(rootView: content)
		hosting.frame = NSRect(origin: .zero, size: size)
		hosting.layoutSubtreeIfNeeded()
		hosting.displayIfNeeded()
		let representation = try XCTUnwrap(hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds))
		hosting.cacheDisplay(in: hosting.bounds, to: representation)
		return try XCTUnwrap(representation.representation(using: .png, properties: [:]))
	}
}

@MainActor
private final class QuotaGeometryCapture {
	var frames = [String: CGRect]()
}

@MainActor
private final class WidgetFailureRenderCapture {
	var frames = [WidgetFailureAccessibilityRole: CGRect]()
	var accessibilityNodes = [WidgetFailureAccessibilityNode]()
}

private struct WidgetFailureRenderProbe<Content: View>: View {
	let content: Content
	let capture: WidgetFailureRenderCapture

	var body: some View {
		content
			.onPreferenceChange(WidgetFailureGeometryPreferenceKey.self) { capture.frames = $0 }
			.onPreferenceChange(WidgetFailureAccessibilityPreferenceKey.self) { capture.accessibilityNodes = $0 }
	}
}

private struct QuotaGeometryProbe<Content: View>: View {
	let content: Content
	let capture: QuotaGeometryCapture

	var body: some View {
		content.onPreferenceChange(QuotaWidgetGeometryPreferenceKey.self) { capture.frames = $0 }
	}
}

// MARK: Task 7 — session scope summary, Go → DesktopWire → host → App Group → Widget

private final class WidgetBatchBundleMarker {}

/// The Go producer's `desktop session-scope` envelopes, decoded through the
/// production DesktopWire decoder.
private struct WidgetBatchResource {
	let asOf: String
	let batches: [String: [DesktopSessionScopeViewV1]]
	let popover: DesktopSessionScopeViewV1
	let oracle: [String: [String: (count: Int, unknown: Int)]]

	static func load() throws -> WidgetBatchResource {
		let url = try XCTUnwrap(Bundle(for: WidgetBatchBundleMarker.self).url(forResource: "session-scope-batch", withExtension: "json"))
		let root = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
		func view(_ object: Any) throws -> DesktopSessionScopeViewV1 {
			try decodeDesktopSessionScopeEnvelopeV1(JSONSerialization.data(withJSONObject: object))
		}
		let rawBatches = try XCTUnwrap(root["batches"] as? [String: [Any]])
		let rawOracle = try XCTUnwrap(root["oracle"] as? [String: [String: [String: Int]]])
		return WidgetBatchResource(
			asOf: try XCTUnwrap(root["as_of"] as? String),
			batches: try rawBatches.mapValues { try $0.map(view) },
			popover: try view(XCTUnwrap(root["popover"])),
			oracle: rawOracle.mapValues { rows in rows.mapValues { (count: $0["count"] ?? -1, unknown: $0["unknown"] ?? -1) } }
		)
	}
}

private struct WidgetBatchReaderError: Error {}

/// Go's answers by selection; records which scopes the host asked for.
private actor WidgetBatchReader: WidgetSessionScopeReading {
	private var views: [DesktopSessionScopeSelectionV1: DesktopSessionScopeViewV1]
	private(set) var scopes = [DesktopSessionScopeV1]()
	var failing = false

	init(_ views: [DesktopSessionScopeViewV1]) {
		self.views = Dictionary(uniqueKeysWithValues: views.compactMap { view in view.selection.map { ($0, view) } })
	}

	func fail() { failing = true }

	func widgetSessionScopeView(_ selection: DesktopSessionScopeSelectionV1, generationID: String?, asOf: String) async throws -> DesktopSessionScopeViewV1 {
		scopes.append(selection.scope)
		guard !failing, let view = views[selection] else { throw WidgetBatchReaderError() }
		return view
	}
}

private let widgetBatchClock: @Sendable () -> Date = { ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")! }

extension WidgetPresentationTests {
	private func appGroupWrite(_ summary: AppGroupSessionScopeSummarySetV1?, to directory: URL) throws -> WidgetDesktopSnapshotV1 {
		let url = try XCTUnwrap(Bundle(for: WidgetBatchBundleMarker.self).url(forResource: "snapshot-complete", withExtension: "json"))
		let envelope = try decodeDesktopWireEnvelopeV1(Data(contentsOf: url))
		try AppGroupSnapshotStore(directoryURL: directory).write(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: summary))
		return try WidgetSnapshotReader(directoryURL: directory).read()
	}

	private func row(_ snapshot: WidgetDesktopSnapshotV1, _ client: WidgetClient, _ period: WidgetPeriod) -> WidgetSessionScopeRow {
		let entry = AgentDeckWidgetEntry(date: widgetBatchClock(), outcome: .loaded(snapshot), kind: .magnitude, client: client, period: period)
		return WidgetSurfaceModel(entry: entry, now: entry.date).sessionScopeRow
	}

	private func temporaryAppGroup() -> URL {
		let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		addTeardownBlock { try? FileManager.default.removeItem(at: url) }
		return url
	}

	/// The L3 joint fixture: Go's nine Auxiliary answers (each client's
	/// today/7d/30d deliberately distinct) reach every actual Widget client ×
	/// period as the same-publication oracle's figure, while the Popover's
	/// temporary Work choice never enters. Confirming All replaces the whole
	/// batch; a failed batch keeps every old figure, marked stale.
	func testJointFixtureCarriesGoFiguresToEveryActualClientAndPeriod() async throws {
		let resource = try WidgetBatchResource.load()
		let auxiliary = try XCTUnwrap(resource.batches["auxiliary"])
		let directory = temporaryAppGroup()
		// The Popover's Work view exists alongside, as it would in the App.
		let reader = WidgetBatchReader(auxiliary + [resource.popover])
		let collector = WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchClock)
		guard case let .accepted(first) = await collector.collect(default: auxiliary[0].default) else { return XCTFail("rejected") }
		let snapshot = try appGroupWrite(first, to: directory)
		var figures = Set<Int>()
		for client in WidgetClient.allCases {
			for period in WidgetPeriod.allCases {
				let oracle = try XCTUnwrap(resource.oracle["auxiliary"]?["\(client.rawValue)/\(period.rawValue)"])
				XCTAssertEqual(row(snapshot, client, period), .value(scope: .auxiliary, count: oracle.count, unknownCount: 0, retainedAt: nil), "\(client)/\(period)")
				figures.insert(oracle.count)
			}
		}
		XCTAssertEqual(figures.count, 9, "every combination's figure is distinct, so none can be borrowed")
		let scopes = await reader.scopes
		XCTAssertEqual(Set(scopes), [.auxiliary], "the Popover's Work selection is never read")

		// Switching only the Popover changes nothing the host publishes.
		let bytes = try Data(contentsOf: directory.appendingPathComponent(WidgetSnapshotReader.fileName))
		guard case let .accepted(again) = await collector.collect(default: auxiliary[0].default) else { return XCTFail("rejected") }
		XCTAssertEqual(again, first)
		_ = try appGroupWrite(again, to: directory)
		XCTAssertEqual(try Data(contentsOf: directory.appendingPathComponent(WidgetSnapshotReader.fileName)), bytes)

		// Confirmed All: only a complete new batch replaces the figures, whole.
		let all = try XCTUnwrap(resource.batches["all"])
		// The host has read the newly confirmed All default from its publication.
		guard case let .accepted(replaced) = await WidgetSessionScopeBatchCollector(reader: WidgetBatchReader(all), now: widgetBatchClock)
			.collect(default: all[0].default)
		else { return XCTFail("rejected") }
		let allSnapshot = try appGroupWrite(replaced, to: directory)
		for client in WidgetClient.allCases {
			for period in WidgetPeriod.allCases {
				let oracle = try XCTUnwrap(resource.oracle["all"]?["\(client.rawValue)/\(period.rawValue)"])
				XCTAssertEqual(row(allSnapshot, client, period), .value(scope: .all, count: oracle.count, unknownCount: oracle.unknown, retainedAt: nil))
			}
		}

		// A failed batch publishes the previous accepted set whole, stale.
		let failing = WidgetBatchReader(all)
		await failing.fail()
		let outcome = await WidgetSessionScopeBatchCollector(reader: failing, now: widgetBatchClock).collect(default: all[0].default)
		XCTAssertEqual(outcome, .rejected(reason: AppGroupSessionScopeSummaryReason.selectionUnavailable))
		let staleSnapshot = try appGroupWrite(replaced.retained(reason: AppGroupSessionScopeSummaryReason.selectionUnavailable), to: directory)
		XCTAssertEqual(row(staleSnapshot, .codex, .sevenDays), .value(
			scope: .all, count: try XCTUnwrap(resource.oracle["all"]?["codex/7d"]).count,
			unknownCount: try XCTUnwrap(resource.oracle["all"]?["codex/7d"]).unknown, retainedAt: widgetBatchClock()
		), "a kept batch carries its own success time, not the snapshot's")
	}

	/// Codec rules: each malformed form degrades only the session row; the
	/// card and its usage stay readable.
	func testSummaryCodecDegradesOnlyTheSessionRow() throws {
		let item: [String: Any] = [
			"available": true, "scope": "work", "count": 5, "unknown_count": 2, "client": "codex", "period": "7d",
			"publication_id": "g", "generated_at": "2026-08-13T10:00:00Z",
		]
		func snapshot(_ summary: Any?) throws -> WidgetDesktopSnapshotV1 {
			let base = try widgetFixture("snapshot-complete")
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(base)) as? [String: Any])
			object["session_scope_summary"] = summary
			return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		}
		func set(_ items: [[String: Any]], stale: Bool = false) -> [String: Any] {
			["stale": stale, "reason": NSNull(), "items": items]
		}
		func edited(_ change: (inout [String: Any]) -> Void) -> [String: Any] {
			var copy = item
			change(&copy)
			return copy
		}
		XCTAssertEqual(row(try snapshot(set([item])), .codex, .sevenDays), .value(scope: .work, count: 5, unknownCount: 2, retainedAt: nil))
		XCTAssertEqual(row(try snapshot(set([edited { $0["unknown_count"] = NSNull() }])), .codex, .sevenDays),
			.value(scope: .work, count: 5, unknownCount: nil, retainedAt: nil), "null is unknown coverage")
		let unavailable: [(String, Any?)] = [
			("legacy file", nil),
			("not an object", "invalid"),
			("missing unknown_count", set([edited { $0.removeValue(forKey: "unknown_count") }])),
			("negative count", set([edited { $0["count"] = -1 }])),
			("unknown above count", set([edited { $0["unknown_count"] = 6 }])),
			("unknown scope", set([edited { $0["scope"] = "maintenance" }])),
			("not available", set([edited { $0["available"] = false }])),
			("auxiliary with unknown", set([edited { $0["scope"] = "auxiliary"; $0["unknown_count"] = 1 }])),
			("auxiliary with null unknown", set([edited { $0["scope"] = "auxiliary"; $0["unknown_count"] = NSNull() }])),
			("no publication", set([edited { $0["publication_id"] = "" }])),
			("bad time", set([edited { $0["generated_at"] = "yesterday" }])),
			("another client", set([edited { $0["client"] = "claude" }])),
			("another period", set([edited { $0["period"] = "30d" }])),
			("duplicate", set([item, item])),
			("empty unavailable set", set([])),
			// ASC-WS-R1-F3: reason is a closed safe enum; an unknown code is
			// never read as a trustworthy summary, stale or not.
			("unknown reason", ["stale": false, "reason": "future_reason", "items": [item]]),
			("unknown reason on a kept set", ["stale": true, "reason": "future_reason", "items": [item]]),
			("non-string reason", ["stale": false, "reason": 7, "items": [item]]),
		]
		for (name, summary) in unavailable {
			let value = try snapshot(summary)
			XCTAssertEqual(row(value, .codex, .sevenDays), .unavailable, name)
			let entry = AgentDeckWidgetEntry(date: widgetBatchClock(), outcome: .loaded(value), kind: .magnitude, client: .codex, period: .sevenDays)
			XCTAssertEqual(WidgetSurfaceModel(entry: entry, now: entry.date).surface, .data, "\(name): usage stays readable")
		}
		// Every known reason keeps a well-formed kept set readable.
		for reason in DesktopSessionScopeSummaryReasonV1.allCases {
			XCTAssertEqual(
				row(try snapshot(["stale": true, "reason": reason.rawValue, "items": [item]]), .codex, .sevenDays),
				.value(scope: .work, count: 5, unknownCount: 2, retainedAt: widgetBatchClock()), reason.rawValue
			)
		}
		// The legacy all-session total is never renamed into the row.
		let legacy = try snapshot(nil)
		XCTAssertEqual(legacy.sessionScopeSummary, .legacy)
		XCTAssertEqual(WidgetSessionScopeText.visible(row(legacy, .all, .today), locale: Locale(identifier: "en_US")), "Session scope unavailable")
	}

	func testSessionRowTextKeepsScopeCountAndUnknownTogether() {
		let english = Locale(identifier: "en_US")
		XCTAssertEqual(WidgetSessionScopeText.visible(.value(scope: .work, count: 3, unknownCount: 1, retainedAt: nil), locale: english), "Work 3 sessions · Unknown 1")
		XCTAssertEqual(WidgetSessionScopeText.visible(.value(scope: .auxiliary, count: 0, unknownCount: 0, retainedAt: nil), locale: english), "Aux 0 sessions · Unknown 0")
		XCTAssertEqual(WidgetSessionScopeText.visible(.value(scope: .all, count: 4, unknownCount: nil, retainedAt: nil), locale: english), "All 4 sessions · Unknown —")
		// Large counts compact visibly but are read in full.
		let large = WidgetSessionScopeRow.value(scope: .all, count: 1_234_567, unknownCount: 12_345, retainedAt: nil)
		XCTAssertEqual(WidgetSessionScopeText.visible(large, locale: english), "All 1.2M sessions · Unknown 12.3K")
		let spoken = WidgetSessionScopeText.accessibility(large, period: .thirtyDays, locale: english)
		XCTAssertTrue(spoken.value.contains("1,234,567"), spoken.value)
		XCTAssertTrue(spoken.value.contains("12,345"), spoken.value)
		XCTAssertTrue(spoken.value.contains(WidgetCopy.period(.thirtyDays)), spoken.value)
		XCTAssertEqual(WidgetFormat.sessionCount(9_999, locale: english), "9,999")
		let aux = WidgetSessionScopeText.accessibility(.value(scope: .auxiliary, count: 2, unknownCount: 0, retainedAt: nil), period: .today, locale: english)
		XCTAssertTrue(aux.value.hasPrefix(WidgetCopy.text("Auxiliary")), "VoiceOver expands Aux: \(aux.value)")
	}

	/// ASC-WS-R1-F1: a newer usage snapshot carrying a summary kept from two
	/// days earlier must not read as freshly updated. The row shows the
	/// summary's own success age; only a current summary omits it.
	func testRetainedSummaryShowsItsOwnSuccessTimeUnderANewerSnapshot() throws {
		let english = Locale(identifier: "en_US")
		let summaryTime = "2026-08-13T10:00:00Z"
		let item: [String: Any] = [
			"available": true, "scope": "work", "count": 5, "unknown_count": 2, "client": "codex", "period": "7d",
			"publication_id": "g", "generated_at": summaryTime,
		]
		func snapshot(outer: String, stale: Bool) throws -> WidgetDesktopSnapshotV1 {
			let base = try widgetFixture("snapshot-complete")
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(base)) as? [String: Any])
			object["generated_at"] = outer
			object["session_scope_summary"] = ["stale": stale, "reason": stale ? "selection_unavailable" : NSNull(), "items": [item]]
			return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
		}
		let newer = "2026-08-15T10:00:00Z"
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(newer))
		let summaryDate = try XCTUnwrap(WidgetTimelinePolicy.date(summaryTime))
		func surface(_ value: WidgetDesktopSnapshotV1) -> (WidgetSessionScopeRow, String) {
			let entry = AgentDeckWidgetEntry(date: now, outcome: .loaded(value), kind: .magnitude, client: .codex, period: .sevenDays)
			let model = WidgetSurfaceModel(entry: entry, now: now)
			return (model.sessionScopeRow, WidgetSessionScopeText.visible(model.sessionScopeRow, now: now, locale: english))
		}
		let fresh = "Work 5 sessions · Unknown 2"
		let expectedAge = WidgetSessionScopeText.retainedAge(summaryDate, now: now, locale: english)
		XCTAssertEqual(expectedAge, WidgetCopy.format("Last updated %@", value: {
			let formatter = RelativeDateTimeFormatter()
			formatter.unitsStyle = .full
			formatter.locale = english
			return formatter.localizedString(for: summaryDate, relativeTo: now)
		}()), "two days is the existing old tier")
		// Kept after a failed batch (stale), under a newer usage snapshot.
		let (staleRow, staleText) = surface(try snapshot(outer: newer, stale: true))
		XCTAssertEqual(staleRow, .value(scope: .work, count: 5, unknownCount: 2, retainedAt: summaryDate))
		XCTAssertEqual(staleText, "\(fresh) · \(try XCTUnwrap(expectedAge))")
		XCTAssertNotEqual(staleText, fresh, "stale numbers never read as current")
		XCTAssertFalse(staleText.contains(WidgetCopy.text("Updated now")))
		// Not marked stale but older than the snapshot carrying it (a
		// publication that reused the last summary): still its own age.
		let (olderRow, olderText) = surface(try snapshot(outer: newer, stale: false))
		XCTAssertEqual(olderRow, .value(scope: .work, count: 5, unknownCount: 2, retainedAt: summaryDate))
		XCTAssertEqual(olderText, staleText)
		let spoken = WidgetSessionScopeText.accessibility(olderRow, period: .sevenDays, now: now, locale: english)
		XCTAssertTrue(spoken.value.hasSuffix(try XCTUnwrap(expectedAge)), spoken.value)
		// A summary answered with its own snapshot shows no separate age.
		let (currentRow, currentText) = surface(try snapshot(outer: summaryTime, stale: false))
		XCTAssertEqual(currentRow, .value(scope: .work, count: 5, unknownCount: 2, retainedAt: nil))
		XCTAssertEqual(currentText, fresh)
		// Within the batch-duration tolerance it is still the same answer.
		let (nearRow, _) = surface(try snapshot(outer: "2026-08-13T10:00:59Z", stale: false))
		XCTAssertEqual(nearRow, .value(scope: .work, count: 5, unknownCount: 2, retainedAt: nil))
		// A recent kept summary uses the non-old wording.
		let recent = try XCTUnwrap(WidgetSessionScopeText.retainedAge(now.addingTimeInterval(-20 * 60), now: now, locale: english))
		XCTAssertTrue(recent.hasPrefix(WidgetCopy.format("Updated %@", value: "")), recent)
	}

	/// ASC-WS-R1-F2: the host's retention path reads the App Group file with
	/// the same rules as the Widget, so a missing unknown_count is never
	/// re-published as a valid explicit null, while an explicit null survives
	/// retention and re-publication as unknown coverage.
	func testHostRetentionRejectsMissingUnknownCountAndKeepsExplicitNull() throws {
		let url = try XCTUnwrap(Bundle(for: WidgetBatchBundleMarker.self).url(forResource: "snapshot-complete", withExtension: "json"))
		let envelope = try decodeDesktopWireEnvelopeV1(Data(contentsOf: url))
		let valid = AppGroupSessionScopeSummaryV1(
			scope: .work, count: 5, unknownCount: 2, client: "codex", period: "7d", publicationID: "g", generatedAt: "2026-08-13T10:00:00Z"
		)
		func file(_ edit: (inout [String: Any]) -> Void, setEdit: (inout [String: Any]) -> Void = { _ in }) throws -> URL {
			let directory = temporaryAppGroup()
			let store = AppGroupSnapshotStore(directoryURL: directory)
			try store.write(AppGroupDesktopSnapshotV1(
				envelope: envelope, sessionScopeSummary: AppGroupSessionScopeSummarySetV1(stale: false, reason: nil, items: [valid])
			))
			let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
			var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
			var items = try XCTUnwrap(set["items"] as? [[String: Any]])
			edit(&items[0])
			set["items"] = items
			setEdit(&set)
			object["session_scope_summary"] = set
			try JSONSerialization.data(withJSONObject: object).write(to: path)
			return directory
		}
		/// The host's rejected-batch path (EmbeddedHelperRunner): keep the
		/// last accepted set as stale, or publish the reason as unavailable.
		func republish(_ directory: URL) throws -> (AppGroupSessionScopeSummarySetV1?, WidgetDesktopSnapshotV1) {
			let store = AppGroupSnapshotStore(directoryURL: directory)
			guard case let .known(existing) = store.readExisting() else {
				XCTFail("the outer snapshot stays readable")
				throw CocoaError(.fileReadCorruptFile)
			}
			let kept = existing.sessionScopeSummary.flatMap { $0.items.isEmpty ? nil : $0 }
			let reason = AppGroupSessionScopeSummaryReason.selectionUnavailable
			try store.write(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: kept?.retained(reason: reason) ?? .unavailable(reason)))
			return (kept, try WidgetSnapshotReader(directoryURL: directory).read())
		}

		// Missing key: the Widget rejects it directly, and the host keeps nothing.
		let missing = try file { $0.removeValue(forKey: "unknown_count") }
		XCTAssertEqual(row(try WidgetSnapshotReader(directoryURL: missing).read(), .codex, .sevenDays), .unavailable)
		let (keptMissing, afterMissing) = try republish(missing)
		XCTAssertNil(keptMissing, "a malformed item is never retained")
		XCTAssertEqual(row(afterMissing, .codex, .sevenDays), .unavailable, "not Work 5 · Unknown —")
		let bytes = String(decoding: try Data(contentsOf: missing.appendingPathComponent(AppGroupSnapshotStore.fileName)), as: UTF8.self)
		XCTAssertFalse(bytes.contains("\"unknown_count\":null"), "the malformed item is not re-encoded as a valid null")

		// Unknown reason and other Widget-rejected forms are not kept either.
		for (name, edit, setEdit) in [
			("unknown reason", { (_: inout [String: Any]) in }, { (set: inout [String: Any]) in set["reason"] = "future_reason" }),
			("unknown above count", { (item: inout [String: Any]) in item["unknown_count"] = 9 }, { (_: inout [String: Any]) in }),
			("bad time", { (item: inout [String: Any]) in item["generated_at"] = "yesterday" }, { (_: inout [String: Any]) in }),
		] as [(String, (inout [String: Any]) -> Void, (inout [String: Any]) -> Void)] {
			let (kept, after) = try republish(try file(edit, setEdit: setEdit))
			XCTAssertNil(kept, name)
			XCTAssertEqual(row(after, .codex, .sevenDays), .unavailable, name)
		}

		// Explicit null: unknown coverage, kept and re-published as null.
		let explicit = try file { $0["unknown_count"] = NSNull() }
		let (keptNull, afterNull) = try republish(explicit)
		XCTAssertEqual(keptNull?.items.first?.unknownCount, .some(nil))
		let summaryDate = try XCTUnwrap(WidgetTimelinePolicy.date("2026-08-13T10:00:00Z"))
		XCTAssertEqual(row(afterNull, .codex, .sevenDays), .value(scope: .work, count: 5, unknownCount: nil, retainedAt: summaryDate))
		let nullBytes = String(decoding: try Data(contentsOf: explicit.appendingPathComponent(AppGroupSnapshotStore.fileName)), as: UTF8.self)
		XCTAssertTrue(nullBytes.contains("\"unknown_count\":null"), "explicit null is written, not omitted")
	}

	@MainActor
	func testSessionRowRenderingsAreAttached() throws {
		let directory = temporaryAppGroup()
		let item = AppGroupSessionScopeSummaryV1(scope: .work, count: 1_234_567, unknownCount: nil, client: "all", period: "today", publicationID: "g", generatedAt: "2026-08-13T10:00:00Z")
		let thirty = AppGroupSessionScopeSummaryV1(scope: .auxiliary, count: 42, unknownCount: 0, client: "all", period: "30d", publicationID: "g", generatedAt: "2026-08-13T10:00:00Z")
		let snapshot = try appGroupWrite(AppGroupSessionScopeSummarySetV1(stale: false, reason: nil, items: [item, thirty]), to: directory)
		let now = try XCTUnwrap(WidgetTimelinePolicy.date(snapshot.generatedAt))
		for (family, period) in [(WidgetFamily.systemSmall, WidgetPeriod.today), (.systemLarge, .thirtyDays), (.systemMedium, .today)] {
			for dynamicType in [DynamicTypeSize.large, .accessibility5] {
				let entry = AgentDeckWidgetEntry(date: now, snapshot: snapshot, kind: .magnitude, client: .all, period: period, isPlaceholder: false)
				let size = WidgetLayoutContract.canvas(family)
				let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
					.environment(\.dynamicTypeSize, dynamicType)
					.environment(\.colorScheme, .light)
					.frame(width: size.width, height: size.height)
					.background(Color.white)
				let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
				XCTAssertGreaterThan(png.count, 1_000)
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Widget session row — magnitude \(familyName(family)) \(period.rawValue) \(dynamicType)"
				attachment.lifetime = .keepAlways
				add(attachment)
			}
		}
		// A kept summary two days older than the card's usage: the row carries
		// its own age (ASC-WS-R1-F1).
		let kept = try appGroupWrite(
			AppGroupSessionScopeSummarySetV1(stale: false, reason: nil, items: [item, thirty]).retained(reason: "selection_unavailable"),
			to: directory
		)
		let later = now.addingTimeInterval(2 * 24 * 60 * 60)
		for (family, period) in [(WidgetFamily.systemSmall, WidgetPeriod.today), (.systemLarge, .thirtyDays)] {
			let entry = AgentDeckWidgetEntry(date: later, snapshot: kept, kind: .magnitude, client: .all, period: period, isPlaceholder: false)
			let size = WidgetLayoutContract.canvas(family)
			let view = AgentDeckWidgetView(entry: entry, familyOverride: family)
				.environment(\.colorScheme, .light)
				.frame(width: size.width, height: size.height)
				.background(Color.white)
			let png = try renderedViewPNG(view, size: NSSize(width: size.width, height: size.height))
			XCTAssertGreaterThan(png.count, 1_000)
			let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
			attachment.name = "Widget session row — retained \(familyName(family)) \(period.rawValue)"
			attachment.lifetime = .keepAlways
			add(attachment)
		}
	}
}

// MARK: ASC-WS-R2-F1 — the Widget sees the same eight rows before and after a cold host

extension WidgetPresentationTests {
	func testOneMalformedItemOfNineDegradesOnlyItsRowAcrossColdHostRepublication() async throws {
		let resource = try WidgetBatchResource.load()
		let auxiliary = try XCTUnwrap(resource.batches["auxiliary"])
		guard case let .accepted(accepted) = await WidgetSessionScopeBatchCollector(
			reader: WidgetBatchReader(auxiliary), now: widgetBatchClock
		).collect(default: auxiliary[0].default) else { return XCTFail("rejected") }
		let directory = temporaryAppGroup()
		_ = try appGroupWrite(accepted, to: directory)
		let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
		var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
		var items = try XCTUnwrap(set["items"] as? [[String: Any]])
		let target = try XCTUnwrap(items.firstIndex { $0["client"] as? String == "codex" && $0["period"] as? String == "7d" })
		items[target].removeValue(forKey: "unknown_count")
		set["items"] = items
		object["session_scope_summary"] = set
		try JSONSerialization.data(withJSONObject: object).write(to: path)
		let summaryTime = try XCTUnwrap(WidgetTimelinePolicy.date(resource.asOf))

		func rows(_ snapshot: WidgetDesktopSnapshotV1, retained: Bool) throws -> (valid: Int, unavailable: [String]) {
			var valid = 0
			var unavailable = [String]()
			for client in WidgetClient.allCases {
				for period in WidgetPeriod.allCases {
					let value = row(snapshot, client, period)
					guard value != .unavailable else { unavailable.append("\(client.rawValue)/\(period.rawValue)"); continue }
					let oracle = try XCTUnwrap(resource.oracle["auxiliary"]?["\(client.rawValue)/\(period.rawValue)"])
					XCTAssertEqual(value, .value(scope: .auxiliary, count: oracle.count, unknownCount: 0, retainedAt: retained ? summaryTime : nil),
						"\(client)/\(period) keeps its own figure and success time")
					valid += 1
				}
			}
			return (valid, unavailable)
		}

		// Read directly: eight rows, only codex/7d unavailable.
		let direct = try rows(try WidgetSnapshotReader(directoryURL: directory).read(), retained: false)
		XCTAssertEqual(direct.valid, 8)
		XCTAssertEqual(direct.unavailable, ["codex/7d"])

		// Cold host: reads the file with the host's rules, keeps the set stale
		// and re-publishes it (the rejected-batch path), twice.
		for round in 1 ... 2 {
			let store = AppGroupSnapshotStore(directoryURL: directory)
			guard case let .known(existing) = store.readExisting(), let kept = existing.sessionScopeSummary, !kept.items.isEmpty else {
				return XCTFail("round \(round): the host kept nothing of the eight well-formed items")
			}
			let url = try XCTUnwrap(Bundle(for: WidgetBatchBundleMarker.self).url(forResource: "snapshot-complete", withExtension: "json"))
			try store.write(AppGroupDesktopSnapshotV1(
				envelope: try decodeDesktopWireEnvelopeV1(Data(contentsOf: url)),
				sessionScopeSummary: kept.retained(reason: AppGroupSessionScopeSummaryReason.selectionUnavailable)
			))
			let after = try rows(try WidgetSnapshotReader(directoryURL: directory).read(), retained: true)
			XCTAssertEqual(after.valid, 8, "round \(round)")
			XCTAssertEqual(after.unavailable, ["codex/7d"], "round \(round)")
			let bytes = String(decoding: try Data(contentsOf: path), as: UTF8.self)
			XCTAssertFalse(bytes.contains("\"unknown_count\":null"), "round \(round): the missing key never becomes an explicit null")
		}
	}
}

// MARK: ASC-WS-R3-F1 — duplicates stay ambiguous through cold-host re-publication

extension WidgetPresentationTests {
	func testDuplicateSelectionsShowTheSameRowsBeforeAndAfterColdHostRepublication() async throws {
		let resource = try WidgetBatchResource.load()
		let auxiliary = try XCTUnwrap(resource.batches["auxiliary"])
		guard case let .accepted(accepted) = await WidgetSessionScopeBatchCollector(
			reader: WidgetBatchReader(auxiliary), now: widgetBatchClock
		).collect(default: auxiliary[0].default) else { return XCTFail("rejected") }
		let url = try XCTUnwrap(Bundle(for: WidgetBatchBundleMarker.self).url(forResource: "snapshot-complete", withExtension: "json"))
		let envelope = try decodeDesktopWireEnvelopeV1(Data(contentsOf: url))
		let summaryTime = try XCTUnwrap(WidgetTimelinePolicy.date(resource.asOf))

		func codexWeek(_ items: [[String: Any]]) -> [String: Any] {
			items.first { $0["client"] as? String == "codex" && $0["period"] as? String == "7d" } ?? [:]
		}
		func malformedTwin(_ items: [[String: Any]]) -> [String: Any] {
			var copy = codexWeek(items)
			copy.removeValue(forKey: "unknown_count")
			copy["count"] = 105
			return copy
		}
		// name, extra items, whether codex/7d is ambiguous (unavailable)
		let cases: [(String, ([[String: Any]]) -> [[String: Any]], Bool)] = [
			("readable malformed twin", { [malformedTwin($0)] }, true),
			("two malformed twins", { [malformedTwin($0), malformedTwin($0)] }, true),
			("malformed twin plus valid twin", { items in
				var valid = codexWeek(items)
				valid["count"] = 106
				return [malformedTwin(items), valid]
			}, true),
			("well-formed twin", { items in
				var valid = codexWeek(items)
				valid["count"] = 105
				return [valid]
			}, true),
			// An item without a readable, supported selection answers no entry,
			// for the Widget directly as after re-publication.
			("twin without a client", { items in
				var copy = malformedTwin(items)
				copy.removeValue(forKey: "client")
				return [copy]
			}, false),
			("twin with an unsupported period", { items in
				var copy = malformedTwin(items)
				copy["period"] = "90d"
				return [copy]
			}, false),
		]
		for (name, extras, ambiguous) in cases {
			let directory = temporaryAppGroup()
			_ = try appGroupWrite(accepted, to: directory)
			let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
			var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
			var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
			let items = try XCTUnwrap(set["items"] as? [[String: Any]])
			set["items"] = items + extras(items)
			object["session_scope_summary"] = set
			try JSONSerialization.data(withJSONObject: object).write(to: path)

			func check(_ snapshot: WidgetDesktopSnapshotV1, retained: Bool, _ label: String) throws {
				for client in WidgetClient.allCases {
					for period in WidgetPeriod.allCases {
						let key = "\(client.rawValue)/\(period.rawValue)"
						let value = row(snapshot, client, period)
						if key == "codex/7d", ambiguous {
							XCTAssertEqual(value, .unavailable, "\(name) \(label): an ambiguous selection is never trusted")
							continue
						}
						let oracle = try XCTUnwrap(resource.oracle["auxiliary"]?[key])
						XCTAssertEqual(value, .value(scope: .auxiliary, count: oracle.count, unknownCount: 0, retainedAt: retained ? summaryTime : nil),
							"\(name) \(label): \(key) keeps its own figure and time")
					}
				}
			}
			try check(try WidgetSnapshotReader(directoryURL: directory).read(), retained: false, "direct")
			for round in 1 ... 2 {
				// The host's rejected-batch path from a cold start.
				let store = AppGroupSnapshotStore(directoryURL: directory)
				guard case let .known(existing) = store.readExisting(), let kept = existing.sessionScopeSummary, !kept.items.isEmpty else {
					return XCTFail("\(name) round \(round): the host kept nothing")
				}
				try store.write(AppGroupDesktopSnapshotV1(
					envelope: envelope, sessionScopeSummary: kept.retained(reason: AppGroupSessionScopeSummaryReason.selectionUnavailable)
				))
				let snapshot = try WidgetSnapshotReader(directoryURL: directory).read()
				try check(snapshot, retained: true, "round \(round)")
				XCTAssertEqual(snapshot.usage.available, true, "\(name): usage stays readable")
				let bytes = String(decoding: try Data(contentsOf: path), as: UTF8.self)
				XCTAssertFalse(bytes.contains("\"unknown_count\":null"), "\(name) round \(round)")
			}
		}
	}
}
