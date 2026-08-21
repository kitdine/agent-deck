import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

@MainActor
final class MenuBarViewModelTests: XCTestCase {
	private func readyModel(
		envelope: DesktopWireEnvelopeV1 = WireFixture.envelope(),
		preferences: DesktopPreferences? = nil
	) async -> MenuBarViewModel {
		let host = StubDesktopHost(behavior: .envelope(envelope))
		let model = await makeModel(host: host, preferences: preferences)
		await model.coordinator.refresh()
		return model
	}

	// MARK: Surface

	func testRefreshingStateIsVisibleUntilTheHelperCompletes() async {
		let host = StubDesktopHost(behavior: .suspendedEnvelope(WireFixture.envelope()))
		let model = await makeModel(host: host)
		let refresh = Task { await model.coordinator.refresh() }
		while host.refreshCount == 0 {
			await Task.yield()
		}

		XCTAssertTrue(model.isRefreshing)
		host.resume()
		await refresh.value
		XCTAssertFalse(model.isRefreshing)
	}

	func testSectionExpansionSurvivesPanelSwitches() async {
		let model = await readyModel()
		XCTAssertTrue(model.sectionIsExpanded("breakdown.models"))

		model.setSection("breakdown.models", expanded: false)
		model.selectedPanel = .sessions
		model.selectedPanel = .breakdown

		XCTAssertFalse(model.sectionIsExpanded("breakdown.models"))
		XCTAssertTrue(model.sectionIsExpanded("breakdown.token-mix"))
	}

	func testRetainedSnapshotIsShownWhenRefreshTimesOut() async {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		host.behavior = .failure(HelperExecutionError.timedOut)
		await model.coordinator.refresh()

		XCTAssertEqual(model.surface, .dataSurface)
		XCTAssertTrue(model.qualifiers.contains(.failing))
		XCTAssertEqual(model.errorCopy, t(DesktopCopy.refreshTimedOut))
		XCTAssertNotNil(model.hero)
	}

	func testErrorSurfaceWithoutAnyRetainedSnapshot() async {
		let host = StubDesktopHost(behavior: .failure(HelperExecutionError.timedOut))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		XCTAssertEqual(model.surface, .errorSurface)
		XCTAssertEqual(model.errorCopy, t(DesktopCopy.refreshTimedOut))
		XCTAssertTrue(model.notices.isEmpty, "an error surface has no snapshot to qualify")
	}

	// MARK: Filter propagation

	func testEveryFilteredPanelReReadsAtTheSelectedClientAndPeriod() async {
		let model = await readyModel()

		model.selectedClient = "all"
		model.selectedPeriod = "today"
		let todayHero = model.hero?.amount
		let todayModels = model.breakdownPanel.models.first?.value
		let todayTiers = model.attributionPanel.tiers.first?.value
		let todayPricing = model.attributionPanel.pricingHeadline

		model.selectedPeriod = "30d"
		XCTAssertNotEqual(model.hero?.amount, nil)
		XCTAssertNotEqual(model.breakdownPanel.models.first?.value, todayModels)
		XCTAssertNotEqual(model.attributionPanel.tiers.first?.value, todayTiers)
		XCTAssertNotEqual(model.attributionPanel.pricingHeadline, todayPricing)
		XCTAssertNotNil(todayHero)

		model.selectedPeriod = "today"
		model.selectedClient = "claude"
		XCTAssertEqual(model.sessionStatistics?.client, "claude")
		XCTAssertEqual(model.sessionsPanel.stats.first?.value, DesktopFormat.count(0))
	}

	func testRhythmBlockIgnoresBothFilters() async {
		let model = await readyModel()
		model.selectedClient = "all"
		model.selectedPeriod = "today"
		let baseline = model.rhythmBlock

		model.selectedClient = "codex"
		model.selectedPeriod = "30d"

		XCTAssertEqual(model.rhythmBlock, baseline)
		XCTAssertEqual(model.rhythmBlock.scopeLine, t(DesktopCopy.rhythmScope))
		XCTAssertTrue(model.rhythmBlock.figures.allSatisfy { $0.note?.isEmpty == false }, "all four Rhythm cards explain what their value means")
		let rhythmCell = try? XCTUnwrap(model.rhythmBlock.cells.first)
		XCTAssertFalse(rhythmCell?.accessibilityValue.contains("%") ?? true)
		XCTAssertTrue(rhythmCell?.accessibilityValue.contains(t(DesktopCopy.settingsMenuBarValueTokens)) ?? false)
		XCTAssertTrue(rhythmCell?.accessibilityValue.contains("$") ?? false)
	}

	// MARK: Notice strip

	func testNoticeStripIsAbsentWhenNothingIsWrong() async {
		let model = await readyModel()
		XCTAssertTrue(model.notices.isEmpty)
	}

	func testNoticeStripOrdersUnreadableThenPartialThenHealth() async {
		let envelope = WireFixture.envelope(
			sessionsAvailable: false,
			health: WireFixture.failingHealth,
			warnings: ["sessions_unavailable"],
			partial: true
		)
		let host = StubDesktopHost(behavior: .envelope(envelope))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		host.behavior = .failure(HelperExecutionError.timedOut)
		await model.coordinator.refresh()

		XCTAssertEqual(
			model.notices.map(\.id),
			["failing", "partial", "health", "warning.sessions_unavailable"]
		)
		XCTAssertTrue(model.notices.contains { $0.opensHealthDetail })
	}

	func testNoticeStripBoundsWarningsAtThreeAndOffersTheRest() async {
		let codes = ["usage_unavailable", "sessions_unavailable", "health_unavailable", "state_close_failed"]
		let model = await readyModel(envelope: WireFixture.envelope(warnings: codes, partial: true))

		let warningNotices = model.notices.filter { $0.id.hasPrefix("warning.") }
		XCTAssertEqual(warningNotices.count, 4)
		XCTAssertEqual(warningNotices.last?.id, "warning.more")
		XCTAssertEqual(warningNotices.last?.text, t(DesktopCopy.noticeMore, Int64(1)))
		XCTAssertTrue(warningNotices.last?.opensHealthDetail ?? false)
	}

	func testUnrecognizedWarningCodeIsShownVerbatimRatherThanDropped() async {
		let model = await readyModel()
		let copy = model.warningCopy("something_nobody_mapped")
		XCTAssertTrue(copy.contains("something_nobody_mapped"))
	}

	// MARK: Panels

	func testUnavailableDomainsMarkTheirPanelAndRenderUnavailable() async {
		let scopes = [
			WireFixture.scope(client: "all", qualityAvailable: false, pricingAvailable: false),
			WireFixture.scope(client: "codex"),
		]
		let model = await readyModel(envelope: WireFixture.envelope(scopes: scopes, sessionsPeriodsAvailable: false))

		let tabs = Dictionary(uniqueKeysWithValues: model.panelTabs.map { ($0.id, $0) })
		XCTAssertTrue(tabs[.attribution]?.marked ?? false)
		XCTAssertTrue(tabs[.sessions]?.marked ?? false)
		XCTAssertFalse(tabs[.usage]?.marked ?? true)
		XCTAssertEqual(tabs[.attribution]?.accessibilityLabel, t(DesktopCopy.panelUnavailableMark, MenuBarPanel.attribution.title))
		XCTAssertFalse(model.attributionPanel.qualityAvailable)
		XCTAssertFalse(model.attributionPanel.pricingAvailable)
	}

	func testIncompleteCostIsNeverLabeledComplete() async {
		var scope = WireFixture.scope(client: "all")
		scope["periods"] = [
			"available": true,
			"items": [
				WireFixture.period("today", tokens: 1440, pricingComplete: false, unpricedComponents: 14),
				WireFixture.period("7d", tokens: 5000),
				WireFixture.period("30d", tokens: 12000),
			],
		]
		let model = await readyModel(envelope: WireFixture.envelope(scopes: [scope]))

		XCTAssertEqual(model.hero?.costIncomplete, t(DesktopCopy.costIncomplete, Int64(14)))
		XCTAssertTrue(model.hero?.amount.hasPrefix("≈") ?? false)

		model.selectedPeriod = "7d"
		XCTAssertNil(model.hero?.costIncomplete)
		XCTAssertFalse(model.hero?.amount.hasPrefix("≈") ?? true)
	}

	func testEmptyPeriodStatesTheDayOnlyOnACurrentIssueFreeSurface() async {
		let scope = WireFixture.scope(client: "all", tokensByPeriod: ["today": 0, "7d": 0, "30d": 0])
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(scopes: [scope])))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		XCTAssertEqual(model.usagePanel.emptyCopy, t(DesktopCopy.emptyToday))

		host.behavior = .failure(HelperExecutionError.timedOut)
		await model.coordinator.refresh()
		XCTAssertEqual(model.usagePanel.emptyCopy, t(DesktopCopy.emptySnapshot))
	}

	func testTodayUsesRealHourlyTrendAndPriciestHourWhenTheProducerSuppliesThem() async {
		let model = await readyModel()

		model.selectedPeriod = "today"
		let today = model.usagePanel.chips
		XCTAssertEqual(today.map(\.id), ["priciest-hour", "events", "cache-hit"])
		XCTAssertEqual(today.first?.value, DesktopFormat.hourWindow(15))
		XCTAssertEqual(today.first?.note, "$0.48")
		XCTAssertEqual(model.usagePanel.buckets.count, 24)
		XCTAssertEqual(model.usagePanel.buckets[15].events, 4)

		model.selectedPeriod = "30d"
		XCTAssertEqual(model.usagePanel.chips.map(\.id), ["average", "peak", "cache-hit"])
	}

	func testTodayHourlyModelAndAxisEndAtMorningNoonAndEndOfDayNow() async {
		for sample in [
			(throughHour: 8, middle: "04:00"),
			(throughHour: 12, middle: "06:00"),
			(throughHour: 23, middle: "12:00"),
		] {
			let scope = WireFixture.scope(client: "all", throughHour: sample.throughHour)
			let model = await readyModel(envelope: WireFixture.envelope(scopes: [scope]))
			model.selectedPeriod = "today"
			XCTAssertEqual(model.usagePanel.buckets.count, sample.throughHour + 1)
			XCTAssertEqual(model.usagePanel.buckets.last?.id, "hour.\(sample.throughHour)")
			XCTAssertEqual(
				TrendChartInteraction.hourlyAxis(
					bucketIDs: model.usagePanel.buckets.map(\.id),
					nowLabel: t(DesktopCopy.trendNow)
				),
				TrendChartAxis(leading: "00:00", middle: sample.middle, trailing: t(DesktopCopy.trendNow))
			)
		}
	}

	func testPriciestHourIgnoresEmptyBucketsAndBreaksCostTiesTowardTheEarlierHour() async throws {
		var scope = WireFixture.scope(client: "all", throughHour: 2)
		var hourly = try XCTUnwrap(scope["hourly"] as? [String: Any])
		var items = try XCTUnwrap(hourly["items"] as? [[String: Any]])
		items[0]["value"] = WireFixture.displayValue(tokens: 9_000, events: 0)
		items[1]["value"] = WireFixture.displayValue(tokens: 5_000, events: 1)
		items[2]["value"] = WireFixture.displayValue(tokens: 5_000, events: 1)
		hourly["items"] = items
		scope["hourly"] = hourly
		let model = await readyModel(envelope: WireFixture.envelope(scopes: [scope]))

		model.selectedPeriod = "today"
		XCTAssertEqual(model.usagePanel.chips.first?.value, DesktopFormat.hourWindow(1))
		XCTAssertEqual(model.usagePanel.chips.first?.note, "$5.00")
	}

	func testLegacySnapshotWithoutHourlyDataKeepsTheDailyFallbackAndDoesNotInventAnHour() async {
		var legacyScope = WireFixture.scope(client: "all")
		legacyScope.removeValue(forKey: "hourly")
		let model = await readyModel(envelope: WireFixture.envelope(scopes: [legacyScope]))

		model.selectedPeriod = "today"
		XCTAssertEqual(model.usagePanel.buckets.count, 1)
		XCTAssertEqual(model.usagePanel.chips.first?.value, "—")
		XCTAssertEqual(model.usagePanel.chips.first?.note, t(DesktopCopy.notCapturedYet))
	}

	func testUnavailableHourlyFamilyUsesTheSameDailyFallbackForChartAndChip() async {
		let scope = WireFixture.scope(client: "all", hourlyAvailable: false)
		let model = await readyModel(envelope: WireFixture.envelope(scopes: [scope]))

		model.selectedPeriod = "today"
		XCTAssertEqual(model.usagePanel.buckets.count, 1)
		XCTAssertEqual(model.usagePanel.chips.first?.value, "—")
		XCTAssertEqual(model.usagePanel.chips.first?.note, t(DesktopCopy.notCapturedYet))
	}

	func testSessionStatisticsComeFromTheProducerRatherThanTheRecentList() async {
		let model = await readyModel(envelope: WireFixture.envelope(sessionItems: [WireFixture.defaultSessionItem]))
		model.selectedClient = "codex"
		model.selectedPeriod = "today"

		XCTAssertEqual(model.sessionsPanel.stats.first(where: { $0.id == "sessions" })?.value, DesktopFormat.count(2))
		XCTAssertEqual(model.sessionsPanel.stats.map(\.id), ["sessions", "average", "projects"])
		XCTAssertEqual(model.sessionsPanel.stats.first(where: { $0.id == "average" })?.value, DesktopFormat.duration(3_600))
		XCTAssertEqual(model.sessionsPanel.projects.map(\.label), ["agent-deck", "ai-tools"])
		XCTAssertEqual(model.sessionsPanel.projects.map(\.shareText), [DesktopFormat.duration(4_200), DesktopFormat.duration(3_000)])
		XCTAssertEqual(model.sessionsPanel.recent.count, 1, "the recent list stays a bounded recent list")
		XCTAssertEqual(model.sessionsPanel.recent[0].when, DesktopFormat.duration(18 * 60))
	}

	func testTrendUsesOnlyTheSelectedDailyWindowAndCarriesRealDetail() async {
		let model = await readyModel()

		model.selectedPeriod = "today"
		XCTAssertEqual(model.usagePanel.buckets.count, 24)
		XCTAssertFalse(model.usagePanel.buckets[0].cost.isEmpty)
		XCTAssertTrue(model.usagePanel.buckets[0].accessibilityValue.contains(model.usagePanel.buckets[0].cost))
		XCTAssertTrue(model.usagePanel.buckets[15].accessibilityValue.contains(t(DesktopCopy.trendEvents, 4)))

		model.selectedPeriod = "7d"
		XCTAssertEqual(model.usagePanel.buckets.count, 3, "the fixture supplies only three bounded daily records")

		model.selectedPeriod = "30d"
		XCTAssertEqual(model.usagePanel.buckets.count, 3, "the host never invents absent daily buckets")
	}

	func testProviderSelectorIsGroupedAndCarriesOneStatusPerRow() async {
		let model = await readyModel()
		let footer = model.footer

		XCTAssertEqual(footer.sections.map(\.id), ["codex", "claude"])
		XCTAssertEqual(footer.sections.map(\.title), ["Codex", "Claude"])
		XCTAssertEqual(footer.sections[0].rows.map(\.label), ["official", "aigocode"])
		XCTAssertTrue(footer.sections[0].rows[0].isCurrent)
		XCTAssertEqual(footer.sections[0].rows[1].target?.provider, "aigocode")
		XCTAssertTrue(footer.sections[1].rows.isEmpty, "an empty client group stays visible instead of borrowing another client's options")
		for row in footer.sections.flatMap(\.rows) {
			XCTAssertFalse(row.isCurrent && row.enabled, "a current route is status, not another mutation target")
			XCTAssertEqual(row.enabled, row.target != nil || !row.choices.isEmpty)
			XCTAssertTrue(row.isCurrent || row.enabled || row.detail != nil)
		}
	}

	func testProviderWithMultipleReadyTargetsUsesOneRowAndASecondLevel() async {
		let model = await readyModel(envelope: WireFixture.envelope(candidates: [WireFixture.multiTargetCandidate]))
		let rows = model.footer.sections.first(where: { $0.id == "codex" })?.rows ?? []
		let provider = try! XCTUnwrap(rows.first(where: { $0.label == "aigocode" }))

		XCTAssertNil(provider.target)
		XCTAssertEqual(provider.choices.map(\.label), ["work · wrapper", "work · direct"])
		XCTAssertEqual(provider.detail, t(DesktopCopy.switchChooseTarget, Int64(2)))
	}

	// MARK: Accessibility and privacy

	func testActiveQualifiersAppearInTheAccessibleValue() async {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionsAvailable: false, partial: true)))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()
		host.behavior = .failure(DesktopWireError.invalidEnvelope)
		await model.coordinator.refresh()

		let summary = model.qualifierSummary ?? ""
		XCTAssertTrue(summary.contains(t(DesktopCopy.partial)))
		XCTAssertTrue(summary.contains(t(DesktopCopy.failing)))
		let failingIndex = summary.range(of: t(DesktopCopy.failing))?.lowerBound
		let partialIndex = summary.range(of: t(DesktopCopy.partial))?.lowerBound
		XCTAssertNotNil(failingIndex)
		XCTAssertNotNil(partialIndex)
		XCTAssertLessThan(failingIndex!, partialIndex!, "reachability precedes completeness")
	}

	func testSessionIdentifiersNeverReachPresentationState() async {
		let model = await readyModel()
		model.selectedClient = "codex"

		let presented = model.sessionsPanel.recent.flatMap { [$0.id, $0.title, $0.detail, $0.when] }
			+ model.sessionsPanel.projects.map(\.label)
		for value in presented {
			XCTAssertFalse(value.contains("session-secret-1"))
		}
	}

	func testHealthDetailCarriesEveryCheckAndItsRecoveryCommand() async {
		let model = await readyModel(envelope: WireFixture.envelope(health: WireFixture.failingHealth))

		XCTAssertEqual(model.healthDetail.rows.count, 3)
		XCTAssertEqual(model.healthDetail.rows[0].status, t(DesktopCopy.healthStatusOK))
		XCTAssertEqual(model.healthDetail.rows[1].status, t(DesktopCopy.healthStatusWarning))
		XCTAssertEqual(model.healthDetail.rows[2].status, t(DesktopCopy.healthStatusFailed))
		XCTAssertEqual(model.healthDetail.rows[2].recovery, "agentdeck usage price update")
	}

	// MARK: Menu-bar item

	func testMenuBarValueModesChangeWhatTheItemRenders() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let model = await readyModel(preferences: preferences)

		XCTAssertEqual(preferences.menuBarValue, .cost)
		let cost = model.menuBarText
		XCTAssertNotNil(cost)

		preferences.menuBarValue = .tokens
		XCTAssertNotEqual(model.menuBarText, cost)

		preferences.menuBarValue = .icon
		XCTAssertNil(model.menuBarText, "icon-only renders no text at all")
	}

	func testMenuBarScopeFollowsThePanelFilterOnlyWhenAsked() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let model = await readyModel(preferences: preferences)

		model.selectedClient = "claude"
		let allClients = model.menuBarText
		preferences.menuBarScope = .followPanel
		XCTAssertNotEqual(model.menuBarText, allClients)

		preferences.menuBarScope = .allClients
		XCTAssertEqual(model.menuBarText, allClients)
	}

	func testGlyphBadgesOnlyForOfflineAndFailing() async {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(sessionsAvailable: false, partial: true)))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		XCTAssertTrue(model.qualifiers.contains(.partial))
		XCTAssertFalse(model.menuBarBadged, "a partial snapshot is still usable")
		XCTAssertEqual(model.menuBarAccessibilityLabel, t(DesktopCopy.appName))

		host.behavior = .failure(HelperExecutionError.timedOut)
		await model.coordinator.refresh()
		XCTAssertTrue(model.menuBarBadged)
		XCTAssertEqual(model.menuBarAccessibilityLabel, t(DesktopCopy.badgedFailing))
	}
}
