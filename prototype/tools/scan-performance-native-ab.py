#!/usr/bin/env python3
"""Install a bounded native A/B experiment in an exported private HEAD copy.

Never patches the product checkout. Real inputs and rendered bitmaps stay private;
the XCTest writes only timings, hashes and boolean correctness checks.
"""
import pathlib
import sys

root = pathlib.Path(sys.argv[1]).resolve()
if not str(root).startswith('/private/tmp/agentdeck-macos-xctest.native-ab.'):
    raise SystemExit('Requires a private native experiment root')
repo = root / 'repo'

controller = repo / 'apps/macos/AgentDeckApp/MenuBarItemController.swift'
text = controller.read_text()
marker = '\tprivate func togglePopover(_ sender: NSStatusBarButton) {'
assert text.count(marker) == 1
text = text.replace(marker, '''
    // Experimental accessors only in the exported copy; presentation is unchanged.
    func proofClick() {
        guard let button = statusItem.button, let window = button.window else { return }
        let location = button.convert(NSPoint(x: button.bounds.midX, y: button.bounds.midY), to: nil)
        for type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
            if let event = NSEvent.mouseEvent(with: type, location: location, modifierFlags: [],
                timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber,
                context: nil, eventNumber: 1, clickCount: 1, pressure: 1) {
                NSApp.postEvent(event, atStart: false)
            }
        }
    }
    var proofView: NSView? { popover.contentViewController?.view }
    var proofShown: Bool { popover.isShown }
    func proofDispose() {
        closePopover()
        NSStatusBar.system.removeStatusItem(statusItem)
    }
''' + marker)
controller.write_text(text)

coordinator = repo / 'apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift'
text = coordinator.read_text()
marker = '\tpublic func setFullAttemptTerminalHandler('
assert text.count(marker) == 1
text = text.replace(marker, '''
    // Candidate mechanism: restore a validated complete envelope before refreshing.
    public func proofRestore(_ envelope: DesktopWireEnvelopeV1) {
        latestSnapshot = envelope
        state = .ready(envelope)
    }
''' + marker)
coordinator.write_text(text)

tests = repo / 'apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift'
text = tests.read_text().replace('import XCTest\n', 'import XCTest\nimport CryptoKit\nimport SQLite3\nimport Vision\n')
marker = '\tprivate func renderedViewPNG<Content: View>'
assert text.count(marker) == 1
test = r'''
    private struct ProofSaved: Decodable {
        let core: String
        let sessions: String
        let sha256: String
        let payload: Data
    }
    private func proofEpoch(_ file: URL, _ table: String) throws -> String {
        var db: OpaquePointer?
        guard sqlite3_open_v2(file.path, &db, SQLITE_OPEN_READONLY, nil) == SQLITE_OK else {
            throw NSError(domain: "proof-readonly-db", code: 1)
        }
        defer { sqlite3_close(db) }
        var statement: OpaquePointer?
        guard sqlite3_prepare_v2(db, "SELECT CAST(epoch AS TEXT) FROM \(table) WHERE singleton=1", -1, &statement, nil) == SQLITE_OK else {
            throw NSError(domain: "proof-epoch-query", code: 1)
        }
        defer { sqlite3_finalize(statement) }
        guard sqlite3_step(statement) == SQLITE_ROW, let value = sqlite3_column_text(statement, 0) else {
            throw NSError(domain: "proof-epoch-row", code: 1)
        }
        return String(cString: value)
    }
    private func proofSaved(_ home: URL) throws -> DesktopWireEnvelopeV1 {
        let bytes = try Data(contentsOf: home.appendingPathComponent("proof-saved.json"))
        let saved = try JSONDecoder().decode(ProofSaved.self, from: bytes)
        let digest = SHA256.hash(data: saved.payload).map { String(format: "%02x", $0) }.joined()
        let state = home.appendingPathComponent(".agentdeck")
        guard digest == saved.sha256,
            try proofEpoch(state.appendingPathComponent("agentdeck.sqlite3"), "derived_snapshot_generation") == saved.core,
            try proofEpoch(state.appendingPathComponent("sessions.sqlite3"), "session_index_generation") == saved.sessions else {
            throw NSError(domain: "proof-invalid-saved-identity", code: 1)
        }
        return try decodeDesktopWireEnvelopeV1(saved.payload)
    }
    private func proofFrame(_ item: MenuBarItemController, _ hero: MenuBarHero) throws -> Bool {
        guard item.proofShown, let view = item.proofView, let window = view.window,
            window.isVisible, window.windowNumber > 0 else {
            throw NSError(domain: "proof-popover-not-visible", code: 1)
        }
        view.layoutSubtreeIfNeeded()
        window.displayIfNeeded()
        view.displayIfNeeded()
        guard let bitmap = view.bitmapImageRepForCachingDisplay(in: view.bounds) else {
            throw NSError(domain: "proof-no-native-bitmap", code: 1)
        }
        view.cacheDisplay(in: view.bounds, to: bitmap)
        // This time is captured before OCR: AppKit completed the usable data draw.
        proofFrameEnd = DispatchTime.now().uptimeNanoseconds
        guard let image = bitmap.cgImage else { throw NSError(domain: "proof-no-image", code: 1) }
        let request = VNRecognizeTextRequest()
        request.recognitionLevel = .accurate
        request.recognitionLanguages = ["en-US"]
        try VNImageRequestHandler(cgImage: image).perform([request])
        let visible = (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }.joined(separator: " ")
        func normalize(_ s: String) -> String { s.lowercased().filter { $0.isLetter || $0.isNumber } }
        // Do not save bitmap or recognized private values in the result bundle.
        return normalize(visible).contains(normalize(hero.tokens)) && normalize(visible).contains(normalize(hero.amount))
    }
    private var proofFrameEnd: UInt64 {
        get { Self.proofEnd }
        set { Self.proofEnd = newValue }
    }
    private static var proofEnd: UInt64 = 0

    func testNativeDataPreparationAB() async throws {
        guard let path = ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"],
            path.hasPrefix("/private/tmp/agentdeck-macos-xctest.native-ab.") else {
            throw NSError(domain: "proof-no-isolated-home", code: 1)
        }
        setenv("AGENTDECK_TEST_LOCALE", "en", 1)
        let home = URL(fileURLWithPath: path)
        let expected = try proofSaved(home)
        let runner = EmbeddedHelperRunner(appBundleURL: Bundle.main.bundleURL,
            environment: ["HOME": path, "CODEX_HOME": home.appendingPathComponent(".codex").path,
                "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin"])
        var samples = [[String: Any]]()
        for pair in -1..<12 {
            for variant in (pair % 2 == 0 ? ["A", "B"] : ["B", "A"]) {
                let coordinator = DesktopRefreshCoordinator(host: DesktopHost(runner: runner), snapshotStore: nil)
                let switching = SwitchController(transport: StubSwitchTransport(), refreshCoordinator: coordinator)
                let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
                let model = MenuBarViewModel(coordinator: coordinator, switchController: switching, preferences: preferences)
                let start = DispatchTime.now().uptimeNanoseconds
                if variant == "A" { await coordinator.refresh(manualQuota: false) }
                else { coordinator.proofRestore(try proofSaved(home)) }
                guard let actual = coordinator.latestSnapshot, model.hero != nil,
                    actual.data.usage.presentation == expected.data.usage.presentation,
                    actual.data.sessions.periods == expected.data.sessions.periods,
                    model.selectedClient == "all", model.selectedPeriod == "today" else {
                    throw NSError(domain: "proof-logical-data-mismatch", code: 1)
                }
                let ms = Double(DispatchTime.now().uptimeNanoseconds - start) / 1_000_000
                if pair >= 0 { samples.append(["pair": pair, "variant": variant,
                    "empty_memory_to_validated_model_ms": ms, "usage_projection_equal": true,
                    "session_periods_equal": true, "hero_available": true]) }
            }
        }
        let output: [String: Any] = ["samples": samples, "pairs": 12, "warmup_pairs_excluded": 1,
            "scope": "native Swift coordinator + wire decoder + MenuBarViewModel; no popover/frame/OS launch measurement",
            "quota": "off; no network", "page_cache": "warm; not purged"]
        try JSONSerialization.data(withJSONObject: output, options: [.prettyPrinted, .sortedKeys])
            .write(to: home.appendingPathComponent("data-preparation-ab-results.json"), options: .atomic)
    }

    func testNativeScanPerformanceAB() async throws {
        guard let path = ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"],
            path.hasPrefix("/private/tmp/agentdeck-macos-xctest.native-ab.") else {
            throw NSError(domain: "proof-no-isolated-home", code: 1)
        }
        setenv("AGENTDECK_TEST_LOCALE", "en", 1)
        let home = URL(fileURLWithPath: path)
        let expected = try proofSaved(home)
        let runner = EmbeddedHelperRunner(appBundleURL: Bundle.main.bundleURL,
            environment: ["HOME": path, "CODEX_HOME": home.appendingPathComponent(".codex").path,
                "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin"])
        var samples = [[String: Any]]()
        // An untimed first opening warms AppKit/SwiftUI/font/OCR caches equally.
        for pair in -1..<12 {
            for variant in (pair % 2 == 0 ? ["A", "B"] : ["B", "A"]) {
                for scenario in ["empty_memory", "warm_click"] {
                    let coordinator = DesktopRefreshCoordinator(host: DesktopHost(runner: runner), snapshotStore: nil)
                    let switching = SwitchController(transport: StubSwitchTransport(), refreshCoordinator: coordinator)
                    let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
                    let model = MenuBarViewModel(coordinator: coordinator, switchController: switching, preferences: preferences)
                    if scenario == "warm_click" { coordinator.proofRestore(expected) }
                    let item = MenuBarItemController(model: model, openSettings: {})
                    // Allow NSStatusItem attachment before timing the button action.
                    try await Task.sleep(for: .milliseconds(20))
                    let start = DispatchTime.now().uptimeNanoseconds
                    item.proofClick()
                    if scenario == "empty_memory" {
                        if variant == "A" { await coordinator.refresh(manualQuota: false) }
                        else { coordinator.proofRestore(try proofSaved(home)) }
                    }
                    // Give SwiftUI's observation transaction one main-loop opportunity.
                    try await Task.sleep(for: .milliseconds(1))
                    guard let actual = coordinator.latestSnapshot, let hero = model.hero else {
                        throw NSError(domain: "proof-no-usable-data", code: 1)
                    }
                    guard actual.data.usage.presentation == expected.data.usage.presentation,
                        actual.data.sessions.periods == expected.data.sessions.periods,
                        model.selectedClient == "all", model.selectedPeriod == "today" else {
                        throw NSError(domain: "proof-logical-data-mismatch", code: 1)
                    }
                    let ocrOK = try proofFrame(item, hero)
                    let milliseconds = Double(proofFrameEnd - start) / 1_000_000
                    item.proofDispose()
                    guard ocrOK else { throw NSError(domain: "proof-displayed-data-mismatch", code: 1) }
                    if pair >= 0 {
                        samples.append(["pair": pair, "variant": variant, "scenario": scenario,
                            "action_to_validated_native_draw_ms": milliseconds, "visible_popover": true,
                            "ocr_hero_verified": true, "usage_projection_equal": true, "session_periods_equal": true])
                    }
                }
            }
        }
        let output: [String: Any] = ["samples": samples, "scope": "hosted-XCTest real status-button action and NSPopover; excludes OS process launch and physical display presentation",
            "pairs": 12, "warmup_pairs_excluded": 1, "quota": "off; no network", "selected_scope": "all/today; full native usage surface"]
        try JSONSerialization.data(withJSONObject: output, options: [.prettyPrinted, .sortedKeys])
            .write(to: home.appendingPathComponent("native-ab-results.json"), options: .atomic)
    }
'''
tests.write_text(text.replace(marker, test + '\n' + marker))
