import XCTest

final class WidgetCopyTests: XCTestCase {
	func testEveryKeyResolvesInBothShippedLanguages() throws {
		let bundle = Bundle(for: WidgetCopyTests.self)
		for identifier in ["en", "zh-Hans"] {
			let path = try XCTUnwrap(bundle.path(forResource: identifier, ofType: "lproj"))
			let localized = try XCTUnwrap(Bundle(path: path))
			for key in WidgetCopy.allKeys {
				let missing = "\u{0}missing"
				let value = localized.localizedString(forKey: key, value: missing, table: nil)
				XCTAssertNotEqual(value, missing, "\(identifier) is missing \(key)")
				XCTAssertFalse(value.isEmpty, "\(identifier) has an empty value for \(key)")
			}
		}
	}

	func testReviewedFailureCopyIsExactInBothShippedLanguages() throws {
		let expected: [String: [String: String]] = [
			"en": [
				"No Widget data yet": "No Widget data yet",
				"Open AgentDeck to refresh": "Open AgentDeck to refresh",
				"Widget storage unavailable": "Widget storage unavailable",
				"Open AgentDeck to retry": "Open AgentDeck to retry",
				"Widget data could not be read": "Widget data could not be read",
				"Will retry on the next refresh": "Will retry on the next refresh",
				"Widget data is from a newer AgentDeck": "Widget data is from a newer AgentDeck",
				"Upgrade AgentDeck to refresh": "Upgrade AgentDeck to refresh",
			],
			"zh-Hans": [
				"No Widget data yet": "还没有小组件数据",
				"Open AgentDeck to refresh": "打开 AgentDeck 以刷新",
				"Widget storage unavailable": "小组件存储不可用",
				"Open AgentDeck to retry": "打开 AgentDeck 以重试",
				"Widget data could not be read": "无法读取小组件数据",
				"Will retry on the next refresh": "等待下次刷新重试",
				"Widget data is from a newer AgentDeck": "小组件数据来自较新版本的 AgentDeck",
				"Upgrade AgentDeck to refresh": "升级 AgentDeck 后刷新",
			],
		]
		let bundle = Bundle(for: WidgetCopyTests.self)
		for (identifier, values) in expected {
			let path = try XCTUnwrap(bundle.path(forResource: identifier, ofType: "lproj"))
			let localized = try XCTUnwrap(Bundle(path: path))
			for (key, value) in values {
				XCTAssertEqual(WidgetCopy.text(key, bundle: localized), value, "\(identifier): \(key)")
			}
		}
	}

	func testInventoryContainsNoDuplicateKeys() {
		XCTAssertEqual(Set(WidgetCopy.allKeys).count, WidgetCopy.allKeys.count)
	}

	func testDefaultLookupUsesTheWidgetResourceBundle() {
		XCTAssertEqual(
			WidgetCopy.text("Usage"),
			WidgetCopy.text("Usage", bundle: WidgetCopy.resourceBundle)
		)
	}

	func testFooterUsesOneLocalizedFreshnessValue() throws {
		let bundle = Bundle(for: WidgetCopyTests.self)
		let cases = [
			("en", "6 hours ago", "Last updated 6 hours ago", "20 minutes ago", "Updated 20 minutes ago"),
			("zh-Hans", "6 小时前", "上次更新于6 小时前", "20 分钟前", "20 分钟前更新"),
		]

		for (identifier, oldRelative, oldExpected, agingRelative, agingExpected) in cases {
			let path = try XCTUnwrap(bundle.path(forResource: identifier, ofType: "lproj"))
			let localized = try XCTUnwrap(Bundle(path: path))
			let old = WidgetFooterPresentation(
				qualifiers: [.partial, .old],
				relativeTime: oldRelative,
				bundle: localized
			)
			let aging = WidgetFooterPresentation(
				qualifiers: [.aging],
				relativeTime: agingRelative,
				bundle: localized
			)

			XCTAssertEqual(old.updateText, oldExpected)
			XCTAssertEqual(old.qualifierText, WidgetCopy.text("Some data unavailable", bundle: localized))
			XCTAssertTrue(old.isOld)
			XCTAssertEqual(aging.updateText, agingExpected)
			XCTAssertEqual(aging.qualifierText, "")
			XCTAssertFalse(aging.isOld)
		}
	}
}
