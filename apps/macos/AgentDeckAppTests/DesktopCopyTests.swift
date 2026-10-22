import XCTest
import AgentDeckShared
@testable import AgentDeck

final class DesktopCopyTests: XCTestCase {
	func testLocaleOverrideRemainsLiveAcrossLanguageChanges() throws {
		let previous = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let previous { setenv("AGENTDECK_TEST_LOCALE", previous, 1) }
			else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		for language in ["en", "zh-Hans", "en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			XCTAssertEqual(DesktopLocale.override, language)
			XCTAssertEqual(DesktopLocale.current.identifier, language)
			XCTAssertEqual(t(DesktopCopy.refreshAction),
				try bundle(language).localizedString(forKey: DesktopCopy.refreshAction, value: nil, table: nil))
		}
		for unsupported in ["", "fr", "en_US"] {
			setenv("AGENTDECK_TEST_LOCALE", unsupported, 1)
			XCTAssertNil(DesktopLocale.override)
			XCTAssertEqual(DesktopLocale.bundle.bundleURL, Bundle.main.bundleURL)
		}
		unsetenv("AGENTDECK_TEST_LOCALE")
		XCTAssertNil(DesktopLocale.override)
		XCTAssertEqual(DesktopLocale.bundle.bundleURL, Bundle.main.bundleURL)
		setenv("AGENTDECK_TEST_LOCALE", "en", 1)
		XCTAssertEqual(t(DesktopCopy.refreshAction), "Refresh")
	}

	func testCalendarDayParsingPreservesLocalDatesAndTimestampFallbacks() throws {
		let previous = NSTimeZone.default
		defer { NSTimeZone.default = previous }
		for zone in ["UTC", "America/New_York", "Asia/Kathmandu", "Pacific/Apia", "UTC"] {
			NSTimeZone.default = try XCTUnwrap(TimeZone(identifier: zone))
			let legacyDayParser = DateFormatter()
			legacyDayParser.locale = Locale(identifier: "en_US_POSIX")
			legacyDayParser.dateFormat = "yyyy-MM-dd"
			for value in ["2024-02-29", "2026-03-08", "2026-11-01", "2011-12-30", "2026-02-30", "2026-2-9", "not-a-date", "2024-02-29"] {
				XCTAssertEqual(DesktopFormat.timestamp(value), legacyDayParser.date(from: value), "\(zone): \(value)")
			}
			for value in ["2026-10-19T08:00:00Z", "2026-10-19T08:00:00.123456789123Z", "2026-10-19T08:00:00+05:45"] {
				let expected = try XCTUnwrap(DesktopTimestamp.date(from: value))
				XCTAssertEqual(DesktopFormat.timestamp(value), expected)
			}
		}
	}

	func testCalendarDayParserSupportsConcurrentReaders() {
		let legacy = DateFormatter()
		legacy.locale = Locale(identifier: "en_US_POSIX")
		legacy.dateFormat = "yyyy-MM-dd"
		let expected = ["2024-02-29", "2026-03-08", "2026-11-01", "2026-02-30", "2026-2-9"]
			.map { ($0, legacy.date(from: $0)) }
		DispatchQueue.concurrentPerform(iterations: 64) { index in
			let (input, date) = expected[index % expected.count]
			XCTAssertEqual(DesktopFormat.timestamp(input), date, input)
		}
	}

	func testSchemaCopyResolvesVersionPairsAndDroppedCount() throws {
		for language in ["en", "zh-Hans"] {
			let localized = try bundle(language)
			let cause = localized.localizedString(forKey: DesktopCopy.schemaSignalCause, value: nil, table: nil)
			let text = String(format: cause, locale: Locale(identifier: language), arguments: [Int64(99), Int64(23)])
			XCTAssertTrue(text.contains("99"))
			XCTAssertTrue(text.contains("23"))
			XCTAssertFalse(text.contains("%"))
			let recovery = localized.localizedString(forKey: DesktopCopy.schemaSignalRecovery, value: nil, table: nil)
			XCTAssertEqual(recovery, language == "en" ? "Upgrade AgentDeck to open this database" : "升级 AgentDeck 后才能打开该数据库")
		}
	}

	private func bundle(_ identifier: String) throws -> Bundle {
		let path = try XCTUnwrap(
			Bundle.main.path(forResource: identifier, ofType: "lproj"),
			"the app bundle must ship a \(identifier) localization"
		)
		return try XCTUnwrap(Bundle(path: path))
	}

	func testEveryKeyResolvesInBothShippedLanguages() throws {
		for identifier in ["en", "zh-Hans"] {
			let localized = try bundle(identifier)
			for key in DesktopCopy.allKeys {
				let value = localized.localizedString(forKey: key, value: "\u{0}missing", table: nil)
				XCTAssertNotEqual(value, "\u{0}missing", "\(identifier) is missing a translation for \(key)")
				if identifier == "zh-Hans" {
					XCTAssertFalse(value.isEmpty, "\(identifier) has an empty translation for \(key)")
				}
			}
		}
	}

	func testTheInventoryHasNoDuplicateKeys() {
		XCTAssertEqual(Set(DesktopCopy.allKeys).count, DesktopCopy.allKeys.count)
	}

	func testHealthRecoveryCopyAndPrerequisitesShipInBothLanguages() throws {
		XCTAssertEqual(DesktopCopy.healthPrerequisiteKeys.count, 7)
		XCTAssertEqual(Set(DesktopCopy.healthPrerequisiteKeys.keys), Set(DesktopCopy.healthPrerequisiteReasons.keys))
		for language in ["en", "zh-Hans"] {
			let localized = try bundle(language)
			let keys = [
				DesktopCopy.healthCopySync, DesktopCopy.healthCopyDiagnostic, DesktopCopy.healthCopySafety,
				DesktopCopy.healthCopied, DesktopCopy.healthEffectStale, DesktopCopy.healthEffectIncomplete,
				DesktopCopy.healthUnknownCheck, DesktopCopy.healthAffectedCount,
			] + Array(DesktopCopy.healthCheckNameKeys.values) + Array(DesktopCopy.healthReasonKeys.values) + Array(DesktopCopy.healthPrerequisiteKeys.values)
			for key in keys {
				let value = localized.localizedString(forKey: key, value: "missing", table: nil)
				XCTAssertNotEqual(value, "missing", "\(language): \(key)")
				XCTAssertFalse(value.isEmpty)
			}
			for key in DesktopCopy.healthPrerequisiteKeys.values {
				let value = localized.localizedString(forKey: key, value: nil, table: nil)
				XCTAssertFalse(value.hasPrefix("rm "))
				XCTAssertFalse(value.contains(" rm -"))
			}
			XCTAssertTrue(localized.localizedString(forKey: DesktopCopy.healthEffectIncomplete, value: nil, table: nil).contains(language == "en" ? "not rolled back" : "没有回滚"))
		}
	}

	func testPeriodicRefreshNoteMatchesReviewedOneMinuteCopy() throws {
		let expected = [
			"en": "Refreshes about once a minute while AgentDeck is running; when off, startup and manual refresh still update data",
			"zh-Hans": "AgentDeck 运行时约每分钟刷新一次；关闭后，启动与手动刷新仍会更新数据",
		]
		for (language, value) in expected {
			let localized = try bundle(language)
			XCTAssertEqual(
				localized.localizedString(forKey: DesktopCopy.settingsPeriodicRefreshNote, value: nil, table: nil),
				value
			)
		}
	}

	func testRefreshPresentationCopyMatchesReviewedEnglishAndChinese() throws {
		let expected: [String: [String: String]] = [
			"en": [
				DesktopCopy.refreshAction: "Refresh",
				DesktopCopy.refreshingAction: "Refreshing…",
				DesktopCopy.updatedAction: "Updated",
				DesktopCopy.refreshFailedAction: "Refresh failed. Retry",
				DesktopCopy.refreshFailedShowingPrevious: "Refresh failed · showing previous data",
				DesktopCopy.firstRefreshFailed: "First refresh failed · no data is available yet",
				DesktopCopy.firstRefreshEmpty: "No data available yet",
				DesktopCopy.widgetPublicationFailed: "Menu-bar data is current · Widgets may be out of date",
			],
			"zh-Hans": [
				DesktopCopy.refreshAction: "刷新",
				DesktopCopy.refreshingAction: "刷新中…",
				DesktopCopy.updatedAction: "已更新",
				DesktopCopy.refreshFailedAction: "刷新失败。重试",
				DesktopCopy.refreshFailedShowingPrevious: "刷新失败 · 正在显示上次数据",
				DesktopCopy.firstRefreshFailed: "首次刷新失败 · 还没有可显示的数据",
				DesktopCopy.firstRefreshEmpty: "还没有可显示的数据",
				DesktopCopy.widgetPublicationFailed: "菜单栏数据已更新 · 小组件可能仍是旧数据",
			],
		]
		for (language, values) in expected {
			let localized = try bundle(language)
			for (key, value) in values {
				XCTAssertEqual(localized.localizedString(forKey: key, value: nil, table: nil), value)
			}
		}
	}

	/// The version withdrew the update check entirely, so no shipped string may
	/// offer one. "Updated <relative>" is freshness, not an update check, which
	/// is why the assertion names phrases rather than the word.
	func testNoStringOffersAnUpdateCheck() throws {
		let forbidden = [
			"check for update", "checking for update", "latest version",
			"new version", "release page", "download page",
			"检查更新", "更新检查", "新版本", "最新版本", "下载页",
		]
		for identifier in ["en", "zh-Hans"] {
			let localized = try bundle(identifier)
			for key in DesktopCopy.allKeys {
				let value = localized.localizedString(forKey: key, value: key, table: nil).lowercased()
				for phrase in forbidden {
					XCTAssertFalse(value.contains(phrase), "\(key) offers an update check in \(identifier)")
				}
			}
		}
	}
}
