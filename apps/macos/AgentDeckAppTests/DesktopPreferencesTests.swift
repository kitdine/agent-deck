import ServiceManagement
import XCTest
@testable import AgentDeck

@MainActor
final class DesktopPreferencesTests: XCTestCase {
	func testDebugXCTestHostDisablesAutomaticRefreshAndRejectsUnsafeAcceptanceHome() throws {
		XCTAssertFalse(debugAutomaticRefreshEnabled(environment: [
			"XCTestConfigurationFilePath": "/private/tmp/test.xctestconfiguration",
		]))
		XCTAssertTrue(debugAutomaticRefreshEnabled(environment: [:]))
		XCTAssertThrowsError(try debugTestHome(environment: [
			"AGENTDECK_TEST_HOME": NSHomeDirectory(),
		])) { error in
			XCTAssertEqual(error as? DebugTestIsolationError, .unsafeHome)
		}

		let isolated = try XCTUnwrap(debugTestHome(environment: [
			"AGENTDECK_TEST_HOME": "/private/tmp/agentdeck-menubar-acceptance.fixture/home",
		]))
		XCTAssertEqual(isolated.path, "/private/tmp/agentdeck-menubar-acceptance.fixture/home")
		let xctestHome = try XCTUnwrap(debugTestHome(environment: [
			"AGENTDECK_TEST_HOME": "/private/tmp/agentdeck-macos-xctest.fixture/home",
		]))
		XCTAssertEqual(xctestHome.path, "/private/tmp/agentdeck-macos-xctest.fixture/home")
		XCTAssertNil(try debugTestHome(environment: [:]))
	}

	func testDefaultsOnACleanDomainAreTheQuietChoice() {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())

		XCTAssertFalse(preferences.periodicRefreshEnabled)
		XCTAssertEqual(preferences.menuBarValue, .cost)
		XCTAssertEqual(preferences.menuBarScope, .allClients)
		XCTAssertEqual(preferences.loginItem, .disabled)
		XCTAssertFalse(preferences.quotaProbeEnabled, "requirements.md clause 1: reading is off by default")
		XCTAssertEqual(preferences.quotaProbeInterval, .fiveMinutes)
	}

	func testPreferencesPersistAcrossARelaunch() {
		let defaults = isolatedDefaults()
		let first = DesktopPreferences(defaults: defaults, registrar: StubLoginItemRegistrar())
		first.periodicRefreshEnabled = true
		first.menuBarValue = .tokens
		first.menuBarScope = .followPanel
		first.quotaProbeEnabled = true
		first.quotaProbeInterval = .thirtyMinutes

		let second = DesktopPreferences(defaults: defaults, registrar: StubLoginItemRegistrar())
		XCTAssertTrue(second.periodicRefreshEnabled)
		XCTAssertEqual(second.menuBarValue, .tokens)
		XCTAssertEqual(second.menuBarScope, .followPanel)
		XCTAssertTrue(second.quotaProbeEnabled)
		XCTAssertEqual(second.quotaProbeInterval, .thirtyMinutes)
	}

	func testQuotaProbeIntervalFallsBackToFiveMinutesForAnUnrecognizedStoredValue() {
		let defaults = isolatedDefaults()
		defaults.set(7, forKey: "quota.probeIntervalMinutes")

		let preferences = DesktopPreferences(defaults: defaults, registrar: StubLoginItemRegistrar())

		XCTAssertEqual(preferences.quotaProbeInterval, .fiveMinutes)
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
