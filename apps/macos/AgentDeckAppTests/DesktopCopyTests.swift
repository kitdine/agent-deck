import XCTest
@testable import AgentDeck

final class DesktopCopyTests: XCTestCase {
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
