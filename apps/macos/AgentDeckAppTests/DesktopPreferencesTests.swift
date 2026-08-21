import ServiceManagement
import XCTest
@testable import AgentDeck

@MainActor
final class DesktopPreferencesTests: XCTestCase {
	func testDefaultsOnACleanDomainAreTheQuietChoice() {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())

		XCTAssertFalse(preferences.periodicRefreshEnabled)
		XCTAssertEqual(preferences.menuBarValue, .cost)
		XCTAssertEqual(preferences.menuBarScope, .allClients)
		XCTAssertEqual(preferences.loginItem, .disabled)
	}

	func testPreferencesPersistAcrossARelaunch() {
		let defaults = isolatedDefaults()
		let first = DesktopPreferences(defaults: defaults, registrar: StubLoginItemRegistrar())
		first.periodicRefreshEnabled = true
		first.menuBarValue = .tokens
		first.menuBarScope = .followPanel

		let second = DesktopPreferences(defaults: defaults, registrar: StubLoginItemRegistrar())
		XCTAssertTrue(second.periodicRefreshEnabled)
		XCTAssertEqual(second.menuBarValue, .tokens)
		XCTAssertEqual(second.menuBarScope, .followPanel)
	}

	func testLoginItemEnableAndDisableAreIdempotent() {
		let registrar = StubLoginItemRegistrar()
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: registrar)

		preferences.setLoginItem(enabled: true)
		preferences.setLoginItem(enabled: true)
		XCTAssertEqual(preferences.loginItem, .enabled)
		XCTAssertEqual(registrar.registerCount, 2)

		preferences.setLoginItem(enabled: false)
		preferences.setLoginItem(enabled: false)
		XCTAssertEqual(preferences.loginItem, .disabled)
		XCTAssertEqual(registrar.unregisterCount, 2)
	}

	func testARefusedLoginItemReportsTheRealStatusRatherThanTheRequestedOne() {
		let registrar = StubLoginItemRegistrar()
		registrar.registerError = CocoaError(.fileWriteNoPermission)
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: registrar)

		preferences.setLoginItem(enabled: true)

		XCTAssertEqual(preferences.loginItem, .refused)
		XCTAssertFalse(preferences.loginItem.isOn, "a refused registration stays visibly off")
	}

	func testAwaitingApprovalIsNotWordedAsAFailure() {
		let registrar = StubLoginItemRegistrar()
		registrar.status = .requiresApproval
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: registrar)

		XCTAssertEqual(preferences.loginItem, .requiresApproval)
		XCTAssertTrue(preferences.loginItem.isOn)
	}
}
