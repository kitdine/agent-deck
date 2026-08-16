import XCTest
@testable import AgentDeckShared

final class DesktopLogPolicyTests: XCTestCase {
    func testLogPolicyUsesClassificationsInsteadOfSnapshotContent() throws {
        let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))

        XCTAssertEqual(DesktopLogPolicy.snapshotEvent(for: complete), "desktop_snapshot_complete")
        XCTAssertEqual(DesktopLogPolicy.helperFailureEvent(.nonZeroExit(23)), "embedded_helper_failed")
        XCTAssertFalse(DesktopLogPolicy.helperFailureEvent(.nonZeroExit(23)).contains("23"))
    }
}
