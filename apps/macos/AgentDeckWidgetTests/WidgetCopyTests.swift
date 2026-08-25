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
