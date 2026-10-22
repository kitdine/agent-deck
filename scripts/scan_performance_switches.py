"""Independent P03 visible-publication transition probe, exported Apps only.

This module does not change the P01/P02 exporter. Call patch_switch_probe only
AFTER the existing matrix patch, then collect in an already isolated fixture.
Client AX presses and period/panel window mouse events are App-local synthetic
input, not physical mouse input. Background control anchors add instrumentation
layout cost; the measured endpoint includes that cost and bitmap capture.
"""
import hashlib
import json
import math
from datetime import datetime, time, timedelta, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

from scan_performance_serving_matrix import replace_once


def patch_switch_probe(repository, digest):
    controller = repository / 'apps/macos/AgentDeckApp/MenuBarItemController.swift'
    delegate = repository / 'apps/macos/AgentDeckApp/AgentDeckApp.swift'
    surface = repository / 'apps/macos/AgentDeckApp/MenuBarSurfaceView.swift'
    panels = repository / 'apps/macos/AgentDeckApp/MenuBarPanelViews.swift'
    shared = repository / 'apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift'
    c = replace_once(controller.read_text().replace('import AppKit\n', 'import AppKit\nimport CryptoKit\n'), '\tprivate enum BaselineFailure: String, Error {',
                     SWIFT + '\n\tprivate enum BaselineFailure: String, Error {') + ACTION
    d = replace_once(delegate.read_text(),
        'if ProcessInfo.processInfo.environment["AGENTDECK_SERVING_MEASUREMENT"] == "model" {',
        'if ProcessInfo.processInfo.environment["AGENTDECK_SERVING_MEASUREMENT"] == "switches" {\n'
        '\t\t\t\t\tawait itemController.captureP03(home: home)\n'
        '\t\t\t\t} else if ProcessInfo.processInfo.environment["AGENTDECK_SERVING_MEASUREMENT"] == "model" {')
    s = surface.read_text()
    s = replace_once(s, 'selection: $model.selectedClient)',
        'selection: Binding(get: { model.selectedClient }, set: { value in\n'
        '\t\t\tP03Action.begin("client", value, model: model)\n\t\t\tmodel.selectedClient = value\n\t\t}))')
    for kind, field, value in [('period', 'selectedPeriod', 'option.id'), ('panel', 'selectedPanel', 'tab.id')]:
        s = replace_once(s, f'model.{field} = {value}',
            f'P03Action.begin("{kind}", {value}' + ('.rawValue' if kind == 'panel' else '') + ', model: model)\n'
            f'\t\t\t\t\tmodel.{field} = {value}')
    for kind, expression in [('period', 'model.selectedPeriod == option.id'),
                             ('panel', 'model.presentedPanel == tab.id')]:
        value = 'option.id' if kind == 'period' else 'tab.id.rawValue'
        anchor = f'.desktopSelectedSegment({expression})'
        s = replace_once(s, anchor, anchor + '\n'
            f'\t\t\t\t.background(P03ControlAnchor(key: "{kind}." + {value}, selected: {expression}))')
    s = replace_once(s, '\t\t\t\tpanel\n', '\t\t\t\tpanel\n'
        '\t\t\t\t\t.p03BodyGeometry(model.presentedPanel.rawValue)\n')
    for anchor, key in [('\t\t\t\t\thero\n', 'hero'),
                        ('NoticeStripView(model: model)\n', 'notices'),
                        ('RhythmBlockView(block: model.rhythmBlock, expansion: sectionExpansion)\n', 'rhythm')]:
        s = replace_once(s, anchor, anchor.rstrip('\n') + f'.p03BodyGeometry("{key}")\n')
    s = replace_once(s, '.modifier(AcceptanceAppearance())', '.modifier(AcceptanceAppearance())\n'
        '\t\t.coordinateSpace(name: "p03.surface")\n'
        '\t\t.onPreferenceChange(P03BodyFrameKey.self) { P03BodyFrames.frames = $0 }')
    h = shared.read_text()
    if h.count('try process.run()') != 2:
        raise RuntimeError('p03_process_launch_anchor_mismatch')
    h = h.replace('try process.run()', 'try process.run()\n            P03HelperLaunches.record()') + HELPER
    v = replace_once(panels.read_text(), 'ShareRowsView(rows: panel.tiers)',
                     'ShareRowsView(rows: panel.tiers).p03BodyGeometry("attribution.numeric")')
    v = replace_once(v, '\t\t\t}\n\t\t\tif !panel.providerGroups.isEmpty {',
                     '\t\t\t}.p03BodyGeometry("attribution.quality")\n\t\t\tif !panel.providerGroups.isEmpty {')
    v = replace_once(v, '\t\t\t\t}\n\t\t\t}\n\t\t\tCollapsibleSection(title: t(DesktopCopy.pricingTitle)',
                     '\t\t\t\t}.p03BodyGeometry("attribution.providers")\n\t\t\t}\n\t\t\tCollapsibleSection(title: t(DesktopCopy.pricingTitle)')
    # All anchors validate before any write. No production files are edited.
    for path, text in [(controller, c), (delegate, d), (surface, s), (shared, h), (panels, v)]:
        path.write_text(text)
    return {str(path.relative_to(repository)): digest(path) for path in (controller, delegate, surface, shared, panels)}


def selection_window(publication_window, period):
    """Calendar-day bounds match the producer, including DST transitions."""
    days = {'today': 0, '7d': 6, '30d': 29}.get(period)
    if days is None or not isinstance(publication_window, dict) or set(publication_window) != {'timezone', 'start', 'end'}:
        raise ValueError('invalid_publication_window')
    zone = ZoneInfo(publication_window['timezone'])
    start = datetime.fromisoformat(publication_window['start'].replace('Z', '+00:00')).astimezone(zone)
    end = datetime.fromisoformat(publication_window['end'].replace('Z', '+00:00')).astimezone(zone)
    if start.time() != time() or end.time() != time() or end.date() != start.date() + timedelta(days=1):
        raise ValueError('invalid_publication_day')
    selected = datetime.combine(start.date() - timedelta(days=days), time(), zone)
    stamp = lambda value: value.astimezone(timezone.utc).isoformat(timespec='seconds').replace('+00:00', 'Z')
    return {'timezone': publication_window['timezone'], 'start': stamp(selected), 'end': stamp(end), 'period': period}


def validate_sample(sample):
    """Fail closed on action identity, immutable publication and bitmap proof."""
    required = ('source', 'target', 'action_source', 'observed', 'publication_before',
                'publication_after', 'window', 'frame_sha256', 'previous_frame_sha256')
    if any(not sample.get(key) for key in required):
        return 'missing_transition_evidence'
    source, target = sample['source'], sample['target']
    if set(source) != {'client', 'period', 'panel'} or set(target) != set(source):
        return 'unknown_selection_shape'
    changed = [key for key in source if source[key] != target[key]]
    if changed != [sample.get('kind')]:
        return 'not_one_actual_transition'
    if sample['action_source'] != source or sample['observed'] != target:
        return 'wrong_or_premutated_selection'
    if sample['publication_before'] != sample['publication_after']:
        return 'publication_changed'
    window = sample['window']
    if not isinstance(window, dict) or set(window) != {'timezone', 'start', 'end', 'period'} or window['period'] != target['period'] or any(not isinstance(v, str) or not v for v in window.values()):
        return 'wrong_window'
    try:
        if window != selection_window(sample.get('publication_window'), target['period']):
            return 'wrong_window'
    except (ValueError, TypeError, KeyError, AttributeError):
        return 'wrong_window'
    if sample.get('period_oracle_sha256') != sample.get('observed_period_sha256'):
        return 'wrong_period_totals'
    for key in ('period_oracle_sha256', 'observed_period_sha256'):
        value = sample.get(key)
        if not isinstance(value, str) or len(value) != 64 or any(c not in '0123456789abcdef' for c in value):
            return 'missing_period_oracle'
    for key in ('control_frame', 'selected_control_frame'):
        rect = sample.get(key)
        if not isinstance(rect, list) or len(rect) != 4 or any(type(v) not in (int, float) or not math.isfinite(v) for v in rect) or rect[2] <= 0 or rect[3] <= 0:
            return 'missing_control_geometry'
    for key in ('frame_sha256', 'previous_frame_sha256'):
        value = sample[key]
        if not isinstance(value, str) or len(value) != 64 or any(c not in '0123456789abcdef' for c in value):
            return 'missing_bitmap_identity'
    if sample.get('action_target') != target[sample['kind']]:
        return 'wrong_action'
    issued, action, frame = (sample.get(key) for key in ('issued_ns', 'action_ns', 'frame_ns'))
    if any(type(v) is not int or v <= 0 for v in (issued, action, frame)) or not issued <= action <= frame:
        return 'invalid_action_clock'
    if sample.get('elapsed_ns') != frame - action:
        return 'wrong_clock_origin'
    if sample.get('panel_oracle_target') != target or type(sample.get('panel_oracle_prepared_ns')) is not int or not 0 < sample['panel_oracle_prepared_ns'] <= issued:
        return 'unbound_panel_oracle'
    state, count = sample.get('panel_expected_state'), sample.get('panel_expected_numeric_count')
    if state not in ('available', 'unavailable', 'empty') or type(count) is not int or (state == 'available' and count < 1) or (state != 'available' and count != 0):
        return 'missing_panel_semantics'
    oracle = sample.get('panel_oracle_fields')
    if not isinstance(oracle, dict) or set(oracle) != {'target', 'state', 'labels', 'values'} or oracle['target'] != target or oracle['state'] != state:
        return 'unbound_panel_oracle'
    if any(not isinstance(oracle[key], list) or any(not isinstance(v, str) or not v for v in oracle[key]) for key in ('labels', 'values')) or not oracle['labels'] or len(oracle['values']) != count:
        return 'missing_panel_semantics'
    oracle_hash = hashlib.sha256(json.dumps(oracle, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
    if oracle_hash != sample.get('panel_oracle_sha256'):
        return 'panel_oracle_digest_mismatch'
    matches = sample.get('panel_numeric_matches')
    if not isinstance(matches, list) or len(matches) != count or any(type(v) is not bool for v in matches):
        return 'missing_panel_evidence'
    if not all(matches):
        return 'wrong_panel_numbers'
    geometry = sample.get('body_geometry')
    if not isinstance(geometry, dict) or any(geometry.get('excludes_' + key) is not True for key in ('hero', 'notices', 'rhythm')):
        return 'panel_crop_contaminated'
    region = 'attribution.quality' if target['panel'] == 'attribution' else target['panel']
    if geometry.get('semantic_region') != region or (target['panel'] == 'attribution' and geometry.get('excludes_provider_quality') is not True):
        return 'panel_crop_contaminated'
    if sample.get('hero_field_matches') != [True, True, True]:
        return 'wrong_hero_crop'
    for key in ('panel_oracle_sha256', 'panel_body_sha256', 'previous_panel_body_sha256'):
        value = sample.get(key)
        if not isinstance(value, str) or len(value) != 64 or any(c not in '0123456789abcdef' for c in value):
            return 'missing_panel_evidence'
    if sample.get('helper_launches_before') != sample.get('helper_launches_after') or type(sample.get('helper_launches_before')) is not int:
        return 'helper_launched'
    if sample['frame_sha256'] == sample['previous_frame_sha256']:
        return 'stale_frame'
    if not all(sample.get(key) is True for key in ('visible', 'selected_control', 'scope_matches', 'numeric_frame', 'panel_frame')):
        return 'wrong_native_frame'
    return None


def summarize(samples, required=100):
    if required < 100:
        raise ValueError('p03_requires_at_least_100_per_exact_transition')
    groups = {}
    rejected = []
    for sample in samples:
        failure = validate_sample(sample)
        if failure or not sample.get('cohort_sha256'):
            rejected.append(failure or 'missing_cohort')
            continue
        identity = {key: sample[key] for key in ('cohort_sha256', 'source', 'target', 'publication_before', 'window')}
        key = hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest()
        group = groups.setdefault(key, {'identity': identity, 'elapsed_ns': []})
        group['elapsed_ns'].append(sample['elapsed_ns'])
    for group in groups.values():
        values = sorted(group.pop('elapsed_ns'))
        group.update(count=len(values), p95_ns=values[math.ceil(.95 * len(values)) - 1] if len(values) >= required else None)
        group['budget_pass'] = group['p95_ns'] <= 100_000_000 if group['p95_ns'] is not None else None
    return {'measurement': 'P03', 'groups': groups, 'rejected': rejected,
            'coverage': 'Only listed exact transitions; no P01/P02 or whole-task verdict'}


HELPER = r'''
public enum P03HelperLaunches {
    private static let value = Counter()
    public static func record() { value.record() }
    public static func count() -> Int { value.count() }
    private final class Counter: @unchecked Sendable {
        private let lock = NSLock()
        private var launches = 0
        func record() { lock.lock(); launches += 1; lock.unlock() }
        func count() -> Int { lock.lock(); defer { lock.unlock() }; return launches }
    }
}
'''

SWIFT = r'''
    private func p03Selection() -> [String: String] {
        ["client": model.selectedClient, "period": model.selectedPeriod, "panel": model.presentedPanel.rawValue]
    }

    private func p03Attribute(_ element: NSObject, _ attribute: NSAccessibility.Attribute) -> Any? {
        guard element.accessibilityAttributeNames().contains(attribute) else { return nil }
        return element.accessibilityAttributeValue(attribute)
    }

    private func p03Role(_ element: NSObject) -> String {
        p03Attribute(element, .role) as? String ?? ""
    }

    private func p03Label(_ element: NSObject) -> String {
        let title = p03Attribute(element, .title) as? String ?? ""
        return title.isEmpty ? (p03Attribute(element, .description) as? String ?? "") : title
    }

    private func p03Rect(_ element: NSObject) -> CGRect {
        if let control = element as? P03ControlView, let window = control.window {
            return window.convertToScreen(control.convert(control.bounds, to: nil))
        }
        guard let position = p03Attribute(element, .position) as? NSValue,
            let size = p03Attribute(element, .size) as? NSValue else { return .zero }
        return CGRect(origin: position.pointValue, size: size.sizeValue)
    }

    private func p03Elements() -> [NSObject] {
        guard let view = popover.contentViewController?.view, let window = view.window else { return [] }
        var queue: [Any] = [window, view]
        var result = [NSObject]()
        var visited = Set<ObjectIdentifier>()
        while !queue.isEmpty, result.count < 4096 {
            guard let element = queue.removeFirst() as? NSObject,
                visited.insert(ObjectIdentifier(element)).inserted else { continue }
            result.append(element)
            queue.append(contentsOf: p03Attribute(element, .children) as? [Any] ?? [])
            if let native = element as? NSView { queue.append(contentsOf: native.subviews) }
        }
        return result
    }

    private func p03Labels(_ kind: String) -> [(String, String)] {
        switch kind {
        case "client": return model.clientTabs.map { ($0.id, "\($0.label) \($0.value ?? "")") }
        case "period": return model.periodOptions.map { ($0.id, $0.label) }
        case "panel": return model.panelTabs.map { ($0.id.rawValue, $0.accessibilityLabel) }
        default: return []
        }
    }

    private func p03Control(_ kind: String, _ value: String) throws -> NSObject {
        let labels = p03Labels(kind)
        guard let expected = labels.first(where: { $0.0 == value })?.1 else { throw P03Failure.controlMissing }
        if kind != "client" {
            guard let view = P03Controls.resolve(kind + "." + value),
                view.window === popover.contentViewController?.view.window,
                !view.isHiddenOrHasHiddenAncestor, !p03Rect(view).isEmpty else { throw P03Failure.controlMissing }
            return view
        }
        let role = kind == "client" ? "AXRadioButton" : "AXButton"
        let groupRole = kind == "client" ? "AXRadioGroup" : "AXGroup"
        let matches = p03Elements().filter { element in
            guard p03Role(element) == role, p03Label(element) == expected, !p03Rect(element).isEmpty,
                (p03Attribute(element, .enabled) as? NSNumber)?.boolValue == true,
                let parent = p03Attribute(element, .parent) as? NSObject, p03Role(parent) == groupRole else { return false }
            let peers = (p03Attribute(parent, .children) as? [NSObject] ?? []).filter {
                p03Role($0) == role && abs(p03Rect($0).midY - p03Rect(element).midY) < 3
            }
            // A unique exact label is insufficient: the native peer row must
            // be precisely the requested selector, with no duplicate labels.
            return peers.count == labels.count && Set(peers.map(p03Label)) == Set(labels.map { $0.1 })
        }
        guard matches.count == 1, let control = matches.first,
            control.accessibilityActionNames().contains(.press) else { throw P03Failure.controlMissing }
        return control
    }

    private func p03Selected(_ element: NSObject) -> Bool {
        if let control = element as? P03ControlView { return control.selected }
        return (p03Attribute(element, .selected) as? NSNumber)?.boolValue == true
            || (p03Attribute(element, .value) as? NSNumber)?.intValue == 1
    }

    private func p03Press(_ element: NSObject) throws {
        P03Action.inputTrace = ["popover_shown": popover.isShown,
            "window_visible": popover.contentViewController?.view.window?.isVisible ?? false,
            "window_key": popover.contentViewController?.view.window?.isKeyWindow ?? false,
            "app_active": NSApp.isActive, "surface_enabled": !model.switchPresentation.blocksSurface]
        guard popover.isShown, let dispatchWindow = popover.contentViewController?.view.window,
            dispatchWindow.isVisible, dispatchWindow.isKeyWindow, NSApp.isActive,
            !model.switchPresentation.blocksSurface else { throw P03Failure.inputPrecondition }
        guard let control = element as? P03ControlView else {
            P03Action.inputTrace["method"] = "AXPress"
            element.accessibilityPerformAction(.press)
            return
        }
        guard let window = control.window else { throw P03Failure.invisible }
        let rect = control.convert(control.bounds, to: nil)
        let point = NSPoint(x: rect.midX, y: rect.midY)
        P03Action.inputTrace["method"] = "window_mouse"
        P03Action.inputTrace["control_frame"] = NSStringFromRect(rect)
        var hit = window.contentView?.hitTest(point)
        var path = [String]()
        while let current = hit, path.count < 6 {
            path.append(NSStringFromClass(type(of: current)))
            if let native = current as? NSControl { P03Action.inputTrace["hit_enabled"] = native.isEnabled }
            hit = current.superview
        }
        P03Action.inputTrace["hit_path"] = path
        // Plain SwiftUI buttons can hit a hosting view instead of NSControl;
        // their enabled state is the exact surface .disabled binding above.
        // An AppKit-backed button must additionally report isEnabled itself.
        guard !path.isEmpty, P03Action.inputTrace["hit_enabled"] as? Bool != false
            else { throw P03Failure.inputPrecondition }
        var events = [NSEvent]()
        for type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
            guard let event = NSEvent.mouseEvent(with: type, location: point, modifierFlags: [],
                timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber,
                context: nil, eventNumber: 0, clickCount: 1, pressure: type == .leftMouseDown ? 1 : 0)
                else { throw P03Failure.actionNotRouted }
            events.append(event)
        }
        // AppKit's mouseDown enters a synchronous tracking loop. Its matching
        // mouseUp must already be queued so the ordinary Button action can run.
        NSApp.postEvent(events[1], atStart: true)
        window.sendEvent(events[0])
    }

    private func p03Recognize(_ image: CGImage) throws -> String {
        let request = VNRecognizeTextRequest()
        request.recognitionLevel = .accurate
        request.recognitionLanguages = ["en-US"]
        try VNImageRequestHandler(cgImage: image).perform([request])
        return (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }.joined(separator: " ")
    }

    private func p03Hash(_ image: CGImage) -> String {
        guard let data = image.dataProvider?.data else { return "" }
        return SHA256.hash(data: data as Data).map { String(format: "%02x", $0) }.joined()
    }

    private func p03PeriodHash(_ period: DesktopUsagePeriodV1) throws -> String {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys]
        return SHA256.hash(data: try encoder.encode(period)).map { String(format: "%02x", $0) }.joined()
    }

    private func p03Window(zoneName: String, start: String, end: String, period: String) throws -> [String: String] {
        guard let zone = TimeZone(identifier: zoneName), let date = DesktopFormat.timestamp(start),
            let days = ["today": 0, "7d": 6, "30d": 29][period] else { throw P03Failure.oracleUnavailable }
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = zone
        guard let lower = calendar.date(byAdding: .day, value: -days, to: date) else { throw P03Failure.oracleUnavailable }
        return ["timezone": zoneName, "start": ISO8601DateFormatter().string(from: lower), "end": end, "period": period]
    }

    private func p03Geometry(_ element: NSObject) -> [Double] {
        let rect = p03Rect(element)
        return [Double(rect.minX), Double(rect.minY), Double(rect.width), Double(rect.height)]
    }

    @MainActor private struct P03PanelOracle {
        let target: [String: String]
        let state: String
        let labels: [String]
        let values: [String]
        let prepared: UInt64

        func matches(_ text: String) -> Bool {
            let normalized = P03Action.normalized(text)
            let labelsMatch = labels.allSatisfy {
                let word = P03Action.normalized($0)
                return !word.isEmpty && normalized.contains(word)
            }
            return labelsMatch && valueMatches(text).allSatisfy { $0 }
        }
        func valueMatches(_ text: String) -> [Bool] {
            // Keep whitespace boundaries while ignoring punctuation/currency:
            // a required 12 must not be accepted merely because 312 is visible.
            let numericText = text.lowercased().filter { $0.isLetter || $0.isNumber || $0.isWhitespace }
            return values.map { value in
                let words = value.split(whereSeparator: \.isWhitespace).map { P03Action.normalized(String($0)) }.filter { !$0.isEmpty }
                guard !words.isEmpty else { return false }
                let pattern = "(?<![\\p{L}\\p{N}])" + words.map(NSRegularExpression.escapedPattern(for:)).joined(separator: "\\s+") + "(?![\\p{L}\\p{N}])"
                return numericText.range(of: pattern, options: .regularExpression) != nil
            }
        }
        var fields: [String: Any] {
            ["target": target, "state": state, "labels": labels, "values": values]
        }
        var semanticHash: String {
            let data = try! JSONSerialization.data(withJSONObject: fields, options: [.sortedKeys, .withoutEscapingSlashes])
            return SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
        }
        var numericHash: String {
            SHA256.hash(data: Data(values.joined(separator: "\n").utf8)).map { String(format: "%02x", $0) }.joined()
        }
    }

    // This is a bounded semantic oracle, not a second view model: only the
    // initially visible body figures are selected from the immutable wire DTO.
    // No expected value comes from the mutated model or from the captured frame.
    private func p03PanelOracle(_ snapshot: DesktopSnapshotV1, target: [String: String]) throws -> P03PanelOracle {
        guard let client = target["client"], let periodID = target["period"], let panel = target["panel"],
            let scope = snapshot.usage.presentation.scopes.first(where: { $0.client == client }),
            let period = scope.periods.items.first(where: { $0.period == periodID }) else { throw P03Failure.oracleUnavailable }
        func oracle(_ state: String, _ labels: [String], _ values: [String] = []) -> P03PanelOracle {
            P03PanelOracle(target: target, state: state, labels: labels, values: values, prepared: DispatchTime.now().uptimeNanoseconds)
        }
        func unavailable() -> P03PanelOracle {
            oracle("unavailable", [t(DesktopCopy.sectionUnavailable)])
        }
        switch panel {
        case "usage":
            guard scope.periods.available, scope.daily.available else { return unavailable() }
            if periodID == "today" {
                return oracle("available", [t(DesktopCopy.chipEvents), t(DesktopCopy.chipCacheHit)],
                    [DesktopFormat.count(period.totals.events), DesktopFormat.percent(period.cacheHitShare)])
            }
            return oracle("available", [t(DesktopCopy.chipAveragePerDay), t(DesktopCopy.chipPeak)],
                [DesktopFormat.decimalCompact(period.averagePerDay.tokens), DesktopFormat.tokens(period.peak.totals.tokens)])
        case "breakdown":
            guard scope.periods.available else { return unavailable() }
            guard let first = period.models.first else { return oracle("empty", [t(DesktopCopy.modelsEmpty)]) }
            return oracle("available", [t(DesktopCopy.modelsTitle), first.model],
                [DesktopFormat.tokens(first.value.tokens), DesktopFormat.percent(first.share)])
        case "attribution":
            guard scope.quality.available,
                let aggregate = scope.quality.items.first(where: { $0.period == periodID && $0.provider == nil })
                else { return unavailable() }
            guard !aggregate.tiers.isEmpty else { throw P03Failure.oracleUnavailable }
            var labels = [t(DesktopCopy.qualityTitle)], values = [String]()
            for tier in aggregate.tiers.prefix(3) {
                switch tier.quality {
                case "determinable": labels.append(t(DesktopCopy.qualityDeterminable))
                case "inferred": labels.append(t(DesktopCopy.qualityInferred))
                case "unattributed": labels.append(t(DesktopCopy.qualityUnattributed))
                default: labels.append(tier.quality)
                }
                values.append(DesktopFormat.cost(tier.value.providerCost, known: tier.value.providerCost, approximate: tier.value.costIncomplete))
                if tier.share != nil { values.append(DesktopFormat.percent(tier.share)) }
            }
            return oracle("available", labels, values)
        case "sessions":
            guard snapshot.sessions.available, snapshot.sessions.periods.available else { return unavailable() }
            guard let statistics = snapshot.sessions.periods.items.first(where: { $0.client == client && $0.period == periodID })
                else { throw P03Failure.oracleUnavailable }
            var labels = [t(DesktopCopy.sessionsCount), t(DesktopCopy.sessionsProjects)]
            var values = [DesktopFormat.count(Int64(statistics.sessions)), DesktopFormat.count(Int64(statistics.distinctProjects))]
            let average = statistics.sessions > 0 ? statistics.totalDurationSeconds / Int64(statistics.sessions) : 0
            // A missing duration renders an em dash, not a numeric zero. The
            // independent count/project fields still prove this scoped body.
            if average > 0 { labels.append(t(DesktopCopy.sessionsAverage)); values.append(DesktopFormat.duration(average)) }
            return oracle("available", labels, values)
        case "quota":
            // A hidden quota tab is never manufactured for a measurement.
            guard snapshot.subscription.reading else { throw P03Failure.oracleUnavailable }
            guard let first = snapshot.subscription.clients.first else { return oracle("empty", [t(DesktopCopy.quotaNoWindows)]) }
            if !first.applicable { return oracle("unavailable", [first.client.capitalized, t(DesktopCopy.quotaNotApplicable)]) }
            if first.failure == .probeDisabled { return oracle("unavailable", [first.client.capitalized, t(DesktopCopy.quotaNotRead)]) }
            guard !first.windows.isEmpty else { return oracle("unavailable", [first.client.capitalized, t(DesktopCopy.quotaUnavailable)]) }
            let formatting = QuotaPanelView(clients: [])
            return oracle("available", [first.client.capitalized] + first.windows.prefix(2).map { formatting.windowLabel($0) },
                first.windows.prefix(2).map { formatting.quotaPercentText($0.usedPercent) })
        default: throw P03Failure.oracleUnavailable
        }
    }

    private func p03ContentRect(_ image: CGImage) throws -> CGRect {
        let key = model.presentedPanel == .attribution ? "attribution.quality" : model.presentedPanel.rawValue
        guard let view = popover.contentViewController?.view, let window = view.window,
            let body = P03BodyFrames.frames[key]
            else { throw P03Failure.invisible }
        let scrollAreas = p03Elements().filter { p03Role($0) == "AXScrollArea" }
        guard scrollAreas.count == 1 else { throw P03Failure.invisible }
        let native = view.convert(window.convertFromScreen(p03Rect(scrollAreas[0])), from: nil)
        let viewport = CGRect(x: native.minX, y: view.isFlipped ? native.minY : view.bounds.height - native.maxY,
            width: native.width, height: native.height)
        let rect = body.intersection(viewport).intersection(view.bounds)
        let scale = CGFloat(image.height) / view.bounds.height
        let crop = CGRect(x: ceil(rect.minX * scale), y: ceil(rect.minY * scale),
            width: floor(rect.width * scale), height: floor(rect.height * scale))
        guard crop.width > 0, crop.height > 0 else { throw P03Failure.invisible }
        return crop
    }

    private func p03ContentImage(_ image: CGImage) throws -> CGImage {
        guard let result = image.cropping(to: try p03ContentRect(image)) else { throw P03Failure.invisible }
        return result
    }

    private func p03HeroImage(_ image: CGImage) throws -> CGImage {
        guard let view = popover.contentViewController?.view,
            let hero = P03BodyFrames.frames["hero"], !hero.isEmpty else { throw P03Failure.invisible }
        let scale = CGFloat(image.height) / view.bounds.height
        let rect = CGRect(x: hero.minX * scale, y: hero.minY * scale, width: hero.width * scale, height: hero.height * scale)
        guard let cropped = image.cropping(to: rect) else { throw P03Failure.invisible }
        return cropped
    }

    private func p03BodyGeometryEvidence(_ image: CGImage) throws -> [String: Any] {
        guard let view = popover.contentViewController?.view,
            let hero = P03BodyFrames.frames["hero"], !hero.isEmpty else { throw P03Failure.invisible }
        let scale = CGFloat(image.height) / view.bounds.height
        let pixels = try p03ContentRect(image)
        let crop = CGRect(x: pixels.minX / scale, y: pixels.minY / scale, width: pixels.width / scale, height: pixels.height / scale)
        let key = model.presentedPanel == .attribution ? "attribution.quality" : model.presentedPanel.rawValue
        var facts: [String: Any] = ["semantic_region": key, "crop": NSStringFromRect(crop), "body": NSStringFromRect(P03BodyFrames.frames[key] ?? .zero)]
        for key in ["hero", "notices", "rhythm"] {
            let region = P03BodyFrames.frames[key] ?? .zero
            facts[key] = NSStringFromRect(region)
            facts["excludes_" + key] = crop.intersection(region).isEmpty
        }
        if model.presentedPanel == .attribution {
            let providers = P03BodyFrames.frames["attribution.providers"] ?? .zero
            facts["provider_quality"] = NSStringFromRect(providers)
            facts["excludes_provider_quality"] = crop.intersection(providers).isEmpty
            guard crop.intersection(providers).isEmpty else { throw P03Failure.frameInvalid }
        }
        return facts
    }

    // Fault injection occurs after the timed capture. The new controls and hero
    // remain pixel-identical; only the panel body is replaced or blanked.
    private func p03ReplaceBody(_ image: CGImage, oldBody: CGImage?) throws -> CGImage {
        let crop = try p03ContentRect(image)
        guard let context = CGContext(data: nil, width: image.width, height: image.height, bitsPerComponent: 8,
            bytesPerRow: 0, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
            else { throw P03Failure.invisible }
        context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        var replacement = crop
        if oldBody == nil {
            guard model.presentedPanel == .attribution, let rows = P03BodyFrames.frames["attribution.numeric"],
                let view = popover.contentViewController?.view else { throw P03Failure.negativeNotDistinct }
            let scale = CGFloat(image.height) / view.bounds.height
            let top = max(crop.minY, ceil(rows.minY * scale))
            replacement = CGRect(x: crop.minX, y: top, width: crop.width, height: crop.maxY - top)
        }
        let destination = CGRect(x: replacement.minX, y: CGFloat(image.height) - replacement.maxY, width: replacement.width, height: replacement.height)
        if let oldBody { context.draw(oldBody, in: destination) }
        else { context.setFillColor(CGColor(gray: 1, alpha: 1)); context.fill(destination) }
        guard let result = context.makeImage() else { throw P03Failure.invisible }
        return result
    }

    func captureP03(home: URL) async {
        var result: [String: Any] = ["version": 1, "measurement": "P03",
            "stimulus": "App-local AXPress for client; NSWindow mouse events at rendered period/panel control anchors; not physical mouse input",
            "samples": [[String: Any]](), "unknownTargetsAreRejected": true]
        var samples = [[String: Any]]()
        var stage = "plan"
        do {
            guard let data = ProcessInfo.processInfo.environment["AGENTDECK_P03_PLAN"]?.data(using: .utf8),
                let plan = try JSONSerialization.jsonObject(with: data) as? [[String: String]],
                !plan.isEmpty, plan.count <= 32 else { throw P03Failure.planInvalid }
            let deadline = DispatchTime.now().uptimeNanoseconds + 150_000_000_000
            stage = "publication_ready"
            while model.coordinator.servingHeader?.sessionPrepared != true || model.hero == nil || model.isRefreshing {
                guard DispatchTime.now().uptimeNanoseconds < deadline else { throw P03Failure.notReady }
                try await Task.sleep(for: .milliseconds(10))
            }
            guard let button = statusItem.button else { throw P03Failure.controlMissing }
            stage = "popover_open"
            await Task.yield()
            togglePopover(button)
            NSApp.activate(ignoringOtherApps: true)
            Self.activatePopoverWindow(popover.contentViewController?.view.window)
            _ = try baselineFrame()
            result["accessibilityNodeCount"] = p03Elements().count
            result["accessibilityRoles"] = Array(Set(p03Elements().map(p03Role))).sorted()
            stage = "control_inventory"
            result["rendered_controls"] = try ["client", "period", "panel"].flatMap { kind in
                try p03Labels(kind).map { value, _ -> [String: Any] in
                    ["kind": kind, "value": value, "frame": p03Geometry(try p03Control(kind, value))]
                }
            }
            for (stepIndex, step) in plan.enumerated() {
                stage = "target_oracle"
                guard let kind = step["kind"], let value = step["value"], ["client", "period", "panel"].contains(kind),
                    step.count == 2 else { throw P03Failure.planInvalid }
                let source = p03Selection()
                guard source[kind] != value else { throw P03Failure.noTransition }
                var target = source; target[kind] = value
                guard let publication = model.coordinator.latestSnapshot, let header = model.coordinator.servingHeader,
                    let scope = publication.data.usage.presentation.scopes.first(where: { $0.client == target["client"] }),
                    let period = scope.periods.items.first(where: { $0.period == target["period"] }),
                    let periodLabel = model.periodOptions.first(where: { $0.id == target["period"] })?.label,
                    model.panelTabs.contains(where: { $0.id.rawValue == target["panel"] }) else { throw P03Failure.oracleUnavailable }
                // Oracle comes from the frozen publication and requested target, before action.
                let amount = DesktopFormat.cost(period.totals.providerCost, known: period.totals.knownProviderCost, approximate: period.totals.providerCost == nil)
                let tokens = DesktopFormat.tokens(period.totals.tokens)
                let scopeLine = "\(periodLabel) · \(DesktopFormat.day(publication.generatedAt))"
                let window = try p03Window(zoneName: header.timezone, start: header.windowStart, end: header.windowEnd, period: period.period)
                let periodHash = try p03PeriodHash(period)
                let panelOracle = try p03PanelOracle(publication.data, target: target)
                let sourcePanelOracle = try p03PanelOracle(publication.data, target: source)
                guard panelOracle.state != "available" || (!panelOracle.values.isEmpty && panelOracle.values.allSatisfy { $0.contains(where: \.isNumber) })
                    else { throw P03Failure.oracleUnavailable }
                let negative = ProcessInfo.processInfo.environment["AGENTDECK_P03_NEGATIVE"] ?? ""
                let negativeIndex = Int(ProcessInfo.processInfo.environment["AGENTDECK_P03_NEGATIVE_INDEX"] ?? "0") ?? -1
                let inject = stepIndex == negativeIndex
                let distinctBody = source["panel"] == target["panel"] && panelOracle.numericHash != sourcePanelOracle.numericHash
                if inject && negative == "old-panel-body" {
                    guard distinctBody, sourcePanelOracle.state == "available", panelOracle.state == "available"
                        else { throw P03Failure.negativeNotDistinct }
                }
                stage = "previous_frame"
                let previous = try baselineFrame()
                let previousBody = try p03ContentImage(previous.image)
                var negativePreflight = false
                if inject && negative == "old-panel-body" {
                    let oldText = try p03Recognize(previousBody)
                    guard sourcePanelOracle.matches(oldText), !panelOracle.matches(oldText)
                        else { throw P03Failure.negativeNotDistinct }
                    negativePreflight = true
                }
                // Layout/capture (and negative preflight OCR) can reconcile
                // SwiftUI views. Never retain an input target across them.
                stage = "target_control"
                let control = try p03Control(kind, value)
                let controlFrame = p03Geometry(control)
                let launches = P03HelperLaunches.count()
                P03Action.last = nil
                let issued = DispatchTime.now().uptimeNanoseconds
                stage = "action_dispatch"
                try p03Press(control)
                P03Action.inputTrace["after_dispatch_selection"] = p03Selection()
                let actionDeadline = issued + 2_000_000_000
                while P03Action.last == nil {
                    guard DispatchTime.now().uptimeNanoseconds < actionDeadline else {
                        P03Action.inputTrace["timeout_selection"] = p03Selection()
                        P03Action.inputTrace["timeout_visible"] = popover.isShown && popover.contentViewController?.view.window?.isVisible == true
                        throw P03Failure.actionNotRouted
                    }
                    try await Task.sleep(for: .milliseconds(1))
                }
                guard let action = P03Action.last, action.kind == kind, action.target == value,
                    action.source == source, action.uptime >= issued else { throw P03Failure.actionIdentity }
                result["last_action"] = ["kind": action.kind, "target": action.target,
                    "source": action.source, "issued_ns": issued, "action_ns": action.uptime]
                stage = "target_frame"
                await Task.yield()
                let frame = try baselineFrame()
                let observed = p03Selection()
                stage = "selected_control"
                let selectedControl = try p03Control(kind, value)
                let selected = p03Selected(selectedControl)
                let launchesAfter = P03HelperLaunches.count()
                guard model.coordinator.latestSnapshot == publication, model.coordinator.servingHeader == header,
                    observed == target, model.activeScope?.client == target["client"], model.activePeriod == period,
                    action.uptime <= frame.end else { throw P03Failure.publicationChanged }
                // Capture endpoint and state before synchronous Vision work. No OCR in the clock.
                stage = "panel_oracle"
                var image = inject && negative == "old-frame" ? previous.image : frame.image
                if inject && ["old-panel-body", "blank-panel-body"].contains(negative) {
                    image = try p03ReplaceBody(frame.image, oldBody: negative == "old-panel-body" ? previousBody : nil)
                }
                let heroImage = try p03HeroImage(image)
                let recognized = P03Action.normalized(try p03Recognize(heroImage))
                stage = "panel_crop"
                let bodyImage = try p03ContentImage(image)
                let content = try p03Recognize(bodyImage)
                let heroMatches = [amount, tokens, scopeLine].map { recognized.contains(P03Action.normalized($0)) }
                let numeric = heroMatches.allSatisfy { $0 }
                let panelValid = panelOracle.matches(content)
                let geometry = try p03BodyGeometryEvidence(image)
                guard ["hero", "notices", "rhythm"].allSatisfy({ geometry["excludes_" + $0] as? Bool == true })
                    else { throw P03Failure.frameInvalid }
                let nativeVisible = popover.isShown && popover.contentViewController?.view.window?.isVisible == true
                let sample: [String: Any] = ["kind": kind, "source": source, "target": target,
                    "action_source": action.source, "action_target": action.target, "observed": observed,
                    "publication_before": header.publicationID, "publication_after": model.coordinator.servingHeader?.publicationID ?? "",
                    "publication_window": ["timezone": header.timezone, "start": header.windowStart, "end": header.windowEnd],
                    "window": window, "period_oracle_sha256": periodHash,
                    "observed_period_sha256": try p03PeriodHash(model.activePeriod!),
                    "control_frame": controlFrame, "selected_control_frame": p03Geometry(selectedControl),
                    "panel_oracle_target": panelOracle.target, "panel_oracle_prepared_ns": panelOracle.prepared,
                    "panel_oracle_fields": panelOracle.fields,
                    "panel_oracle_sha256": panelOracle.semanticHash, "panel_expected_state": panelOracle.state,
                    "panel_expected_numeric_count": panelOracle.values.count,
                    "panel_numeric_matches": panelOracle.valueMatches(content),
                    "panel_header_present": panelOracle.labels.first.map { P03Action.normalized(content).contains(P03Action.normalized($0)) } ?? false,
                    "body_geometry": geometry, "body_negative_preflight_passed": negativePreflight,
                    "input_dispatch": P03Action.inputTrace,
                    "hero_field_matches": heroMatches, "hero_crop_sha256": p03Hash(heroImage),
                    "panel_source_numeric_sha256": sourcePanelOracle.numericHash,
                    "panel_target_numeric_sha256": panelOracle.numericHash, "panel_numeric_distinct_from_source": distinctBody,
                    "panel_body_sha256": p03Hash(bodyImage), "previous_panel_body_sha256": p03Hash(previousBody),
                    "body_fault_injected": inject && ["old-panel-body", "blank-panel-body"].contains(negative),
                    "body_crop": NSStringFromRect(try p03ContentRect(image)),
                    "issued_ns": issued, "action_ns": action.uptime, "frame_ns": frame.end, "elapsed_ns": frame.end - action.uptime,
                    "helper_launches_before": launches, "helper_launches_after": launchesAfter,
                    "frame_sha256": p03Hash(image), "previous_frame_sha256": p03Hash(previous.image),
                    "visible": nativeVisible, "selected_control": selected, "scope_matches": model.activeScope?.client == target["client"],
                    "numeric_frame": numeric, "panel_frame": panelValid]
                samples.append(sample)
                stage = "frame_validation"
                guard nativeVisible, selected, numeric, panelValid, launches == launchesAfter,
                    p03Hash(image) != p03Hash(previous.image) else { throw P03Failure.frameInvalid }
            }
        } catch let failure as P03Failure { result["failure"] = failure.rawValue }
        catch { result["failure"] = "p03_native_collection_failed" }
        if result["failure"] != nil { result["failure_stage"] = stage }
        result["last_input_dispatch"] = P03Action.inputTrace
        result["samples"] = samples
        let destination = home.appendingPathComponent("baseline-result.json")
        try? JSONSerialization.data(withJSONObject: result).write(to: destination, options: [.atomic])
        try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
        closePopover()
    }

    private enum P03Failure: String, Error {
        case planInvalid, notReady, controlMissing, noTransition, oracleUnavailable, actionNotRouted
        case actionIdentity, publicationChanged, frameInvalid, invisible, negativeNotDistinct, inputPrecondition
    }
'''
# File-scope action recorder belongs only to the exported app, and only records
# normal control callbacks. It never performs a selection mutation itself.
SWIFT += r'''
'''
ACTION = r'''
extension View {
    func p03BodyGeometry(_ key: String) -> some View {
        background(GeometryReader { proxy in
            Color.clear.preference(key: P03BodyFrameKey.self, value: [key: proxy.frame(in: .named("p03.surface"))])
        }.allowsHitTesting(false))
    }
}

struct P03BodyFrameKey: PreferenceKey {
    static let defaultValue: [String: CGRect] = [:]
    static func reduce(value: inout [String: CGRect], nextValue: () -> [String: CGRect]) {
        value.merge(nextValue(), uniquingKeysWith: { _, new in new })
    }
}

@MainActor enum P03BodyFrames {
    static var frames: [String: CGRect] = [:]
}

// A background view takes the Button's rendered size without adding layout or
// intercepting input. Only the normal SwiftUI Button callback changes selection.
@MainActor
struct P03ControlAnchor: NSViewRepresentable {
    let key: String
    let selected: Bool
    func makeNSView(context: Context) -> P03ControlView { P03ControlView() }
    func updateNSView(_ view: P03ControlView, context: Context) {
        view.selected = selected
        P03Controls.register(view, key: key)
    }
}

@MainActor
final class P03ControlView: NSView {
    var selected = false
    override func hitTest(_ point: NSPoint) -> NSView? { nil }
    override func isAccessibilityElement() -> Bool { false }
}

@MainActor
enum P03Controls {
    private final class Reference {
        weak var view: P03ControlView?
        init(_ view: P03ControlView) { self.view = view }
    }
    private static var views: [String: [Reference]] = [:]
    static func register(_ view: P03ControlView, key: String) {
        var existing = (views[key] ?? []).filter { $0.view != nil }
        if !existing.contains(where: { $0.view === view }) { existing.append(Reference(view)) }
        views[key] = existing
    }
    static func resolve(_ key: String) -> P03ControlView? {
        let live = (views[key] ?? []).compactMap { $0.view }.filter { $0.window != nil }
        return live.count == 1 ? live[0] : nil
    }
}

@MainActor
enum P03Action {
    struct Event {
        let kind: String
        let target: String
        let source: [String: String]
        let uptime: UInt64
    }
    static var last: Event?
    static var inputTrace: [String: Any] = [:]
    static func normalized(_ value: String) -> String { value.lowercased().filter { $0.isLetter || $0.isNumber } }
    static func begin(_ kind: String, _ target: String, model: MenuBarViewModel) {
        last = Event(kind: kind, target: target,
            source: ["client": model.selectedClient, "period": model.selectedPeriod, "panel": model.presentedPanel.rawValue],
            uptime: DispatchTime.now().uptimeNanoseconds)
        if ProcessInfo.processInfo.environment["AGENTDECK_P03_NEGATIVE"] == "delayed-action" {
            Thread.sleep(forTimeInterval: 0.15)
        }
    }
}
'''


def collect(app, home, plan, baseline, probe, negative_control=None, negative_index=0):
    """One visible sequence; caller owns exact-cache restore between launches.

    Uses the existing real reader, process cleanup and worker-idle guards. The
    starting identity is taken after reader/schema checks (SQLite sidecars count).
    No frozen input/checkpoint is changed or regenerated by this function.
    """
    from scan_performance_serving_matrix import (validate_cache_in_place, database_schema_versions,
        state_identity, wait_for_private_worker, launch_model, CACHE)
    app, home = Path(app).resolve(), Path(home).resolve()
    if home.name != 'home' or not home.parent.name.startswith('agentdeck-macos-xctest.serving.') or not str(home).startswith('/private/tmp/'):
        raise ValueError('p03_private_home_required')
    if not str(app).startswith('/private/tmp/agentdeck-macos-xctest.serving.') or app.suffix != '.app':
        raise ValueError('p03_private_app_required')
    if not isinstance(plan, list) or not 1 <= len(plan) <= 32 or any(
            not isinstance(step, dict) or set(step) != {'kind', 'value'} or step['kind'] not in ('client', 'period', 'panel')
            or not isinstance(step['value'], str) or not step['value'] for step in plan):
        raise ValueError('p03_explicit_bounded_plan_required')
    if negative_control not in (None, 'old-frame', 'delayed-action', 'old-panel-body', 'blank-panel-body'):
        raise ValueError('p03_unknown_negative_control')
    if type(negative_index) is not int or not 0 <= negative_index < len(plan):
        raise ValueError('p03_negative_index_outside_plan')
    helper = app / 'Contents/Helpers/agentdeck'
    wait_for_private_worker(home)
    validation = validate_cache_in_place(baseline, helper, home)
    schemas = database_schema_versions(home)
    inventory_path = home.parent / 'private-inventory.json'
    inventory = json.loads(inventory_path.read_text())
    for item in inventory:
        relative = Path(item['relative'])
        source = home / relative
        if relative.is_absolute() or '..' in relative.parts or source.is_symlink() or not source.resolve().is_relative_to(home):
            raise ValueError('p03_frozen_source_boundary')
        if source.stat().st_size != item['bytes'] or source.stat().st_mtime_ns != item['mtime_ns'] or probe.sha(source) != item['sha256']:
            raise ValueError('p03_frozen_source_changed')
    binaries = [app / 'Contents/MacOS/AgentDeck', helper,
                app / 'Contents/Frameworks/AgentDeckShared.framework/Versions/A/AgentDeckShared']
    identity = {'binaries': {str(p.relative_to(app)): probe.sha(p) for p in binaries},
        'starting_database_sha256': state_identity(home, probe.sha), 'starting_schema_versions': schemas,
        'starting_cache_sha256': probe.sha(home / '.agentdeck' / CACHE), 'cache_validation': validation,
        'frozen_inventory_sha256': probe.sha(inventory_path), 'collector_sha256': probe.sha(Path(__file__)),
        'plan': plan, 'negative_control': negative_control, 'negative_index': negative_index}
    (home / 'baseline-result.json').unlink(missing_ok=True)
    environment = {'AGENTDECK_SERVING_CONDITION': 'restore', 'AGENTDECK_SERVING_MEASUREMENT': 'switches',
                   'AGENTDECK_P03_PLAN': json.dumps(plan, separators=(',', ':'))}
    if negative_control:
        environment['AGENTDECK_P03_NEGATIVE'] = negative_control
        environment['AGENTDECK_P03_NEGATIVE_INDEX'] = str(negative_index)
    native = None
    try:
        native = launch_model(baseline, app, home, environment)
    finally:
        cleanup = probe.stop_private_app(app)
        wait_for_private_worker(home)
    if {str(p.relative_to(app)): probe.sha(p) for p in binaries} != identity['binaries']:
        raise RuntimeError('p03_artifact_changed_during_collection')
    samples = native.get('samples', [])
    for index, sample in enumerate(samples):
        # Prior control history affects mounted SwiftUI state; keep it in cohort.
        cohort = {**identity, 'transition_index': index}
        sample['cohort_sha256'] = hashlib.sha256(json.dumps(cohort, sort_keys=True).encode()).hexdigest()
        sample['validation_failure'] = validate_sample(sample)
    return {'identity': identity, 'native': native, 'cleanup': cleanup,
            'summary': summarize(samples), 'negative_control': negative_control}
