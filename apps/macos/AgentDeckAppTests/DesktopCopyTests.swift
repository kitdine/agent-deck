import XCTest
@testable import AgentDeck

final class DesktopCopyTests: XCTestCase {
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
