import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

@MainActor
final class SwitchFlowTests: XCTestCase {
	private func makeReadyModel(outcome: ProviderSwitchTransportOutcome) async -> (MenuBarViewModel, StubSwitchTransport, StubDesktopHost) {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let transport = StubSwitchTransport(outcome: outcome)
		let model = await makeModel(host: host, transport: transport)
		await model.coordinator.refresh()
		return (model, transport, host)
	}

	func testSelectingAnOptionOnlyOpensConfirmation() async {
		let (model, transport, _) = await makeReadyModel(outcome: .succeeded)
		let row = model.footer.sections.first(where: { $0.id == "codex" })?.rows.first { $0.enabled }
		XCTAssertNotNil(row?.target)

		model.pendingConfirmation = row?.target
		guard case let .confirming(target, message) = model.switchPresentation else {
			return XCTFail("selecting an option must open a confirmation")
		}
		XCTAssertEqual(target.provider, "aigocode")
		XCTAssertTrue(message.contains("work"), "the confirmation names the credential")
		let recorded = await transport.recordedTargets()
		XCTAssertTrue(recorded.isEmpty, "no switch runs before confirmation")
	}

	func testConfirmationUsesExactlyTheResolvedOption() async {
		let (model, transport, host) = await makeReadyModel(outcome: .succeeded)
		let row = model.footer.sections.first(where: { $0.id == "codex" })?.rows.first { $0.enabled }
		model.pendingConfirmation = row?.target
		let refreshesBefore = host.refreshCount
		model.confirmSwitch()
		await Task.yield()
		try? await Task.sleep(for: .milliseconds(200))

		let recorded = await transport.recordedTargets()
		XCTAssertEqual(recorded.count, 1)
		XCTAssertEqual(recorded.first?.client, "codex")
		XCTAssertEqual(recorded.first?.provider, "aigocode")
		XCTAssertEqual(recorded.first?.credential, "work")
		XCTAssertEqual(recorded.first?.viaWrapper, true)
		XCTAssertGreaterThan(host.refreshCount, refreshesBefore, "a successful switch triggers one replacement refresh")
	}

	func testTypedFailureLeavesPresentedStateUnchangedAndShowsTheCodeOnly() async {
		let (model, _, _) = await makeReadyModel(outcome: .failed(code: "state_busy"))
		let routesBefore = model.footer.routesText
		let row = model.footer.sections.first(where: { $0.id == "codex" })?.rows.first { $0.enabled }
		model.pendingConfirmation = row?.target
		model.confirmSwitch()
		try? await Task.sleep(for: .milliseconds(200))

		guard case let .failed(message, _) = model.switchPresentation else {
			return XCTFail("a typed failure must be presented as a failure")
		}
		XCTAssertEqual(message, t(DesktopCopy.switchFailed, "state_busy"))
		XCTAssertEqual(model.footer.routesText, routesBefore, "the previous provider is still current")
	}

	func testIndeterminateOutcomeClaimsNeitherSuccessNorFailure() async {
		let (model, _, _) = await makeReadyModel(outcome: .indeterminate)
		let row = model.footer.sections.first(where: { $0.id == "codex" })?.rows.first { $0.enabled }
		model.pendingConfirmation = row?.target
		model.confirmSwitch()
		try? await Task.sleep(for: .milliseconds(200))

		guard case let .indeterminate(message, _) = model.switchPresentation else {
			return XCTFail("an unclassifiable result must read as unconfirmed")
		}
		XCTAssertEqual(message, t(DesktopCopy.switchIndeterminate))
	}

	func testEveryOtherOptionIsBlockedWhileASwitchIsInFlight() async {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let transport = SuspendingSwitchTransport()
		let model = await makeModel(host: host, transport: transport)
		await model.coordinator.refresh()

		let row = model.footer.sections.first(where: { $0.id == "codex" })?.rows.first { $0.enabled }
		model.pendingConfirmation = row?.target
		model.confirmSwitch()
		try? await Task.sleep(for: .milliseconds(100))

		XCTAssertTrue(model.switchPresentation.blocksSurface)
		for section in model.footer.sections {
			for candidate in section.rows {
				XCTAssertFalse(candidate.enabled)
				XCTAssertEqual(candidate.detail, t(DesktopCopy.switchOptionBlocked), "the overlay replaces each option's own reason")
			}
		}
		await transport.release()
	}

	func testOptionReasonsAreLocalizedAndAnUnknownCodeIsShownVerbatim() async {
		let (model, _, _) = await makeReadyModel(outcome: .succeeded)
		XCTAssertEqual(model.reasonCopy("wrapper_not_configured"), t(DesktopCopy.reasonWrapperNotConfigured))
		XCTAssertEqual(model.reasonCopy("already_selected"), t(DesktopCopy.reasonAlreadySelected))
		XCTAssertTrue(model.reasonCopy("brand_new_code")?.contains("brand_new_code") ?? false)
		XCTAssertNil(model.reasonCopy(nil))
	}

	func testCurrentRoutesRemainVisibleWhenNoCandidatesAreOffered() async {
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope(candidates: [])))
		let model = await makeModel(host: host)
		await model.coordinator.refresh()

		XCTAssertTrue(model.footer.switchingAvailable)
		XCTAssertFalse(model.footer.routesText.isEmpty, "current routes stay readable")
		let codex = model.footer.sections.first(where: { $0.id == "codex" })
		XCTAssertEqual(codex?.rows.map(\.label), ["official"])
		XCTAssertTrue(codex?.rows.first?.isCurrent ?? false)
	}
}

/// Holds the switch in flight until the test releases it.
actor SuspendingSwitchTransport: ProviderSwitching {
	private var continuation: CheckedContinuation<Void, Never>?

	func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome {
		await withCheckedContinuation { continuation = $0 }
		return .succeeded
	}

	func release() {
		continuation?.resume()
		continuation = nil
	}
}
