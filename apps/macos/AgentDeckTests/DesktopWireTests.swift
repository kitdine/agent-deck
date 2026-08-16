import Foundation
import XCTest
@testable import AgentDeckShared

final class DesktopWireTests: XCTestCase {
    func testCanonicalCompleteAndPartialFixturesDecode() throws {
        let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
        let partial = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-partial.json"))

        XCTAssertFalse(complete.partial)
        XCTAssertTrue(complete.warnings.isEmpty)
        XCTAssertTrue(complete.data.provider.available)
        XCTAssertTrue(complete.data.sessions.available)

        XCTAssertTrue(partial.partial)
        XCTAssertFalse(partial.warnings.isEmpty)
        XCTAssertFalse(partial.data.provider.available)
        XCTAssertTrue(partial.data.health.available)
    }

	func testUnsupportedWireVersionIsRejected() throws {
        let complete = try desktopFixtureData("snapshot-complete.json")
        var object = try XCTUnwrap(JSONSerialization.jsonObject(with: complete) as? [String: Any])
        var data = try XCTUnwrap(object["data"] as? [String: Any])
        data["wire_version"] = 2
        object["data"] = data
        let unsupported = try JSONSerialization.data(withJSONObject: object)

        XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(unsupported)) { error in
            XCTAssertEqual(error as? DesktopWireError, .unsupportedWireVersion(2))
		}
	}

	func testRFC3339TimestampsAllowOffsetsAndFractionsAndRejectMalformedValues() throws {
		let fixture = try desktopFixtureData("snapshot-complete.json")
		let offset = "2026-08-13T18:00:00+08:00"
		let offsetData = try replacingTimestamps(
			in: fixture,
			generatedAt: offset,
			dataGeneratedAt: offset,
			nextRefreshAt: "2026-08-13T18:05:00+08:00"
		)
		XCTAssertEqual(try decodeDesktopWireEnvelopeV1(offsetData).generatedAt, offset)

		let fractional = "2026-08-13T10:00:00.123Z"
		let fractionalData = try replacingTimestamps(
			in: fixture,
			generatedAt: fractional,
			dataGeneratedAt: fractional,
			nextRefreshAt: "2026-08-13T10:05:00.123Z"
		)
		XCTAssertEqual(try decodeDesktopWireEnvelopeV1(fractionalData).data.generatedAt, fractional)

		for invalidData in [
			try replacingTimestamps(in: fixture, generatedAt: "not-a-timestamp"),
			try replacingTimestamps(in: fixture, dataGeneratedAt: "2026-08-13 10:00:00Z"),
		] {
			XCTAssertThrowsError(try decodeDesktopWireEnvelopeV1(invalidData)) { error in
				XCTAssertEqual(error as? DesktopWireError, .invalidTimestamp)
			}
		}
	}

	private func replacingTimestamps(
		in fixture: Data,
		generatedAt: String? = nil,
		dataGeneratedAt: String? = nil,
		nextRefreshAt: String? = nil
	) throws -> Data {
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: fixture) as? [String: Any])
		var data = try XCTUnwrap(object["data"] as? [String: Any])
		if let generatedAt {
			object["generated_at"] = generatedAt
		}
		if let dataGeneratedAt {
			data["generated_at"] = dataGeneratedAt
		}
		if let nextRefreshAt {
			data["next_refresh_at"] = nextRefreshAt
		}
		object["data"] = data
		return try JSONSerialization.data(withJSONObject: object)
	}
}
