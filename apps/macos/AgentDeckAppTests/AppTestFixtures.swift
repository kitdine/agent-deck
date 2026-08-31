import Foundation
import ServiceManagement
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

/// Synthetic wire payloads. Nothing here reads real AgentDeck, Codex, Claude,
/// App Group, or network state: every value is written by the test.
///
/// The type is `@MainActor` because its five static `[String: Any]` payloads are
/// shared mutable global state to Swift 6's concurrency checker, and every
/// consumer is already a main-actor test. Isolating the owner is what makes them
/// safe; `nonisolated(unsafe)` would only silence the checker, and no external
/// synchronization invariant exists here to justify that.
@MainActor
enum WireFixture {
	static func totals(
		tokens: Int64,
		events: Int64 = 1,
		sessions: Int64 = 1,
		priced: Bool = true,
		unpricedComponents: Int = 0
	) -> [String: Any] {
		// Cost tracks tokens so a period filter that fails to propagate is
		// visible as an unchanged amount rather than hidden by a constant.
		let known = String(format: "%.6f", Double(tokens) / 1000)
		return [
			"tokens": tokens,
			"input_tokens": tokens / 2,
			"output_tokens": tokens / 4,
			"cached_read_tokens": tokens / 8,
			"cache_write_tokens": tokens / 8,
			"events": events,
			"sessions": sessions,
			"catalog_base_cost": priced ? known : NSNull(),
			"provider_cost": priced ? known : NSNull(),
			"known_catalog_base_cost": known,
			"known_provider_cost": known,
			"pricing_complete": priced,
			"unpriced_components": unpricedComponents,
		]
	}

	static func displayValue(tokens: Int64, events: Int64 = 1, incomplete: Bool = false) -> [String: Any] {
		[
			"tokens": tokens,
			"events": events,
			"provider_cost": String(format: "%.6f", Double(tokens) / 1_000),
			"cost_incomplete": incomplete,
		]
	}

	static func period(
		_ name: String,
		tokens: Int64,
		pricingComplete: Bool = true,
		unpricedComponents: Int = 0,
		models: [String] = ["gpt-5"]
	) -> [String: Any] {
		[
			"period": name,
			"totals": totals(
				tokens: tokens,
				events: tokens == 0 ? 0 : 3,
				sessions: tokens == 0 ? 0 : 1,
				priced: pricingComplete,
				unpricedComponents: unpricedComponents
			),
			"average_per_day": ["tokens": "\(tokens).00", "provider_cost": "1.000000", "known_provider_cost": "1.000000"],
			"peak": ["date": "2026-08-13", "totals": totals(tokens: tokens)],
			"cache_hit_share": "66.10",
			"models": models.map { name in
				["client": NSNull(), "model": name, "value": displayValue(tokens: tokens), "share": "100.00"] as [String: Any]
			},
		]
	}

	static func scope(
		client: String,
		tokensByPeriod: [String: Int64] = ["today": 1440, "7d": 5000, "30d": 12000],
		qualityAvailable: Bool = true,
		pricingAvailable: Bool = true,
		rhythmAvailable: Bool = true,
		dailyAvailable: Bool = true,
		hourlyAvailable: Bool = true,
		throughHour: Int = 23,
		periodsAvailable: Bool = true,
		unpricedIdentifiers: [String: [String]] = [:]
	) -> [String: Any] {
		precondition((0 ... 23).contains(throughHour))
		let order = ["today", "7d", "30d"]
		let hourlyPattern: [Int64] = [0, 0, 0, 0, 0, 0, 40, 80, 120, 240, 320, 220, 180, 200, 400, 480, 240, 300, 120, 80, 40, 20, 10, 0]
		let hourlyItems: [[String: Any]] = hourlyPattern.prefix(throughHour + 1).enumerated().map { hour, base in
			let tokens = client == "claude" ? base / 5 : client == "codex" ? base * 4 / 5 : base
			return [
				"hour": hour,
				"value": displayValue(tokens: tokens, events: tokens == 0 ? 0 : max(1, tokens / 100)),
			]
		}
		let rhythmValues = (0 ..< 7 * 24).map { index -> (intensity: Int, tokens: Int64) in
			let weekday = index / 24
			let hour = index % 24
			let peak = weekday == 1 && hour == 15
			return (peak ? 100 : 10, peak ? 4_800 : Int64((weekday + 1) * (hour + 1)))
		}
		return [
			"client": client,
			"periods": [
				"available": periodsAvailable,
				"items": order.map { period($0, tokens: tokensByPeriod[$0] ?? 0) },
			],
			"daily": [
				"available": dailyAvailable,
				"items": (0 ..< 3).map { offset in
					["date": "2026-08-1\(1 + offset)", "value": displayValue(tokens: Int64(100 * (offset + 1)))] as [String: Any]
				},
			],
			"hourly": [
				"available": hourlyAvailable,
				"through_hour": throughHour,
				"items": hourlyAvailable ? hourlyItems : [],
			],
			"quality": [
				"available": qualityAvailable,
				"items": order.flatMap { name -> [[String: Any]] in
					[
						[
							"period": name,
							"provider": NSNull(),
							"tiers": [
								["quality": "determinable", "value": displayValue(tokens: tokensByPeriod[name] ?? 0), "share": "100.00"],
							],
						],
						[
							"period": name,
							"provider": "official",
							"tiers": [
								["quality": "determinable", "value": displayValue(tokens: tokensByPeriod[name] ?? 0), "share": "100.00"],
							],
						],
					]
				},
			],
			"pricing": [
				"available": pricingAvailable,
				"items": order.map { name in
					[
						"period": name,
						"priced_events": Int64(order.firstIndex(of: name) ?? 0) + 1,
						"unpriced_events": Int64(0),
						"coverage": "100.00",
						"unpriced_identifiers": unpricedIdentifiers[name] ?? [],
					] as [String: Any]
				},
			],
			"rhythm": [
				"available": rhythmAvailable,
				"intensities": rhythmValues.map(\.intensity),
				"tokens": rhythmValues.map(\.tokens),
				"provider_costs": rhythmValues.map { String(format: "%.6f", Double($0.tokens) / 1_000) },
				"cost_incomplete": rhythmValues.map { _ in false },
				"active_days": 27,
				"busiest_day": "tuesday",
				"quietest_day": "sunday",
			],
		]
	}

	static func workSignals(
		available: Bool = true,
		costBasis: String = "turn",
		missingWorkflowMetrics: Bool = false,
		emptyMaintenance: Bool = false,
		omitToolingForTodayAll: Bool = false,
		omitAllForTodayAll: Bool = false,
		toolingFamilyAvailable: Bool = true
	) -> [String: Any] {
		guard available else {
			return [
				"activity": ["available": false, "items": [[String: Any]]()] as [String: Any],
				"workflow": ["available": false, "items": [[String: Any]]()] as [String: Any],
				"tooling": ["available": false, "items": [[String: Any]]()] as [String: Any],
			]
		}
		let periods = ["today", "7d", "30d"]
		let clients = ["all", "codex"]
		let activityItems: [[String: Any]] = periods.flatMap { period in
			clients.compactMap { client -> [String: Any]? in
				guard !(omitAllForTodayAll && period == "today" && client == "all") else { return nil }
				return [
					"period": period,
					"client": client,
					"cost_basis": costBasis,
					"kinds": [
						[
							"kind": "coding", "share": 52.0, "cost": 2.74, "events": 21,
							"sub": [
								["kind": "feature", "share": 24.0, "cost": 1.25, "events": 10],
								["kind": "refactoring", "share": 13.0, "cost": 0.68, "events": 5],
								["kind": "testing", "share": 9.0, "cost": 0.47, "events": 4],
								[
									"kind": "maintenance",
									"share": emptyMaintenance ? 0.0 : 6.0,
									"cost": emptyMaintenance ? 0.0 : 0.34,
									"events": emptyMaintenance ? 0 : 2,
								],
							],
						],
						[
							"kind": "debugging", "share": 21.0, "cost": 1.11, "events": 10,
							"sub": [
								["kind": "investigation", "share": 12.0, "cost": 0.63, "events": 6],
								["kind": "repair", "share": 9.0, "cost": 0.48, "events": 4],
							],
						],
						[
							"kind": "conversation", "share": 17.0, "cost": 0.89, "events": 8,
							"sub": [
								["kind": "exploration", "share": 7.0, "cost": 0.37, "events": 3],
								["kind": "brainstorming", "share": 5.0, "cost": 0.26, "events": 2],
								["kind": "planning", "share": 5.0, "cost": 0.26, "events": 3],
							],
						],
						[
							"kind": "delegation", "share": 10.0, "cost": 0.53, "events": 4,
							"sub": [
								["kind": "subagent", "share": 6.0, "cost": 0.32, "events": 2],
								["kind": "workflow", "share": 4.0, "cost": 0.21, "events": 2],
							],
						],
					],
				] as [String: Any]
			}
		}
		let workflowItems: [[String: Any]] = periods.flatMap { period in
			clients.compactMap { client -> [String: Any]? in
				guard !(omitAllForTodayAll && period == "today" && client == "all") else { return nil }
				let periodIndex = periods.firstIndex(of: period) ?? 0
				let firstEdit: Any = missingWorkflowMetrics
					? NSNull()
					: (periodIndex + 1) * (client == "codex" ? 60 : 120)
				let filesTouched: Any = missingWorkflowMetrics ? NSNull() : periodIndex + 3
				let retries: Any = missingWorkflowMetrics ? NSNull() : periodIndex
				let editsPerSession: Any = missingWorkflowMetrics
					? NSNull()
					: Double(periodIndex + 1) + (client == "codex" ? 0.5 : 0)
				let topFile: Any = missingWorkflowMetrics ? NSNull() : "tasks.md"
				let topFileEdits: Any = missingWorkflowMetrics ? NSNull() : periodIndex + 4
				return [
					"period": period,
					"client": client,
					"first_edit_seconds": firstEdit,
					"files_touched": filesTouched,
					"retries": retries,
					"edits_per_session": editsPerSession,
					"top_file": topFile,
					"top_file_edits": topFileEdits,
				] as [String: Any]
			}
		}
		let toolingItems: [[String: Any]] = periods.flatMap { period in
			clients.compactMap { client -> [String: Any]? in
				let omitted = period == "today" && client == "all"
				guard !(omitted && (omitToolingForTodayAll || omitAllForTodayAll)) else { return nil }
				return [
					"period": period,
					"client": client,
					"calls": 181,
					"groups": 5,
					"rows": [
						["kind": "other", "calls": 99, "share": 54.7],
						["kind": "edit", "calls": 18, "share": 9.9],
						["kind": "bash", "calls": 32, "share": 17.7],
						["kind": "mcp", "calls": 10, "share": 5.5],
						["kind": "read", "calls": 22, "share": 12.2],
					],
					"top_mcp_server": "codegraph",
					"top_mcp_calls": 7,
				] as [String: Any]
			}
		}
		return [
			"activity": ["available": true, "items": activityItems] as [String: Any],
			"workflow": ["available": true, "items": workflowItems] as [String: Any],
			"tooling": [
				"available": toolingFamilyAvailable,
				"items": toolingFamilyAvailable ? toolingItems : [],
			] as [String: Any],
		]
	}

	static func envelope(
		scopes: [[String: Any]] = [scope(client: "all"), scope(client: "codex"), scope(client: "claude")],
		clientSubtotalsAvailable: Bool = true,
		presentationAvailable: Bool = true,
		sessionsAvailable: Bool = true,
		sessionsPeriodsAvailable: Bool = true,
		includeWorkSignals: Bool = true,
		workSignalsAvailable: Bool = true,
		workSignalsPayload: [String: Any]? = nil,
		sessionItems: [[String: Any]] = [defaultSessionItem],
		health: [String: Any] = healthyHealth,
		warnings: [String] = [],
		partial: Bool = false,
		candidates: [[String: Any]] = [defaultCandidate],
		routes: [[String: Any]] = [defaultRoute],
		generatedAt: String = "2026-08-13T10:00:00Z"
	) -> DesktopWireEnvelopeV1 {
		// Each sub-expression is named and explicitly typed. One heterogeneous
		// literal of this size makes the Swift type checker give up — it compiled
		// only until the module's other error was removed, at which point it became
		// "unable to type-check this expression in reasonable time".
		let periodNames = ["today", "7d", "30d"]
		let clientNames = ["all", "codex", "claude"]

		let subtotalItems: [[String: Any]] = periodNames.flatMap { name in
			clientNames.map { client -> [String: Any] in
				let tokens: Int64 = client == "claude" ? 100 : 1440
				return ["period": name, "client": client, "value": displayValue(tokens: tokens)]
			}
		}

		let sessionPeriodItems: [[String: Any]] = periodNames.flatMap { name in
			clientNames.map { client -> [String: Any] in
				let empty = client == "claude"
				let projects: [[String: Any]] = empty ? [] : [
					["project": "agent-deck", "sessions": 1, "duration_seconds": 4_200],
					["project": "ai-tools", "sessions": 1, "duration_seconds": 3_000],
				]
				return [
					"period": name,
					"client": client,
					"sessions": empty ? 0 : 2,
					"total_duration_seconds": empty ? 0 : 7200,
					"median_duration_seconds": empty ? 0 : 3600,
					"distinct_projects": empty ? 0 : 2,
					"projects": projects,
				]
			}
		}

		let presentation: [String: Any] = [
			"available": presentationAvailable,
			"scopes": scopes,
			"client_subtotals": ["available": clientSubtotalsAvailable, "items": subtotalItems] as [String: Any],
		]

		let usage: [String: Any] = [
			"available": true,
			"from": "2026-08-13T00:00:00Z",
			"to": generatedAt,
			"tokens": [String: Int64](),
			"counts": [String: Int64](),
			"catalog_base_cost": "1.000000",
			"provider_cost": "1.000000",
			"known_catalog_base_cost": "1.000000",
			"known_provider_cost": "1.000000",
			"pricing_complete": true,
			"unpriced_components": 0,
			"warnings": [String](),
			"presentation": presentation,
		]

		var sessions: [String: Any] = [
			"available": sessionsAvailable,
			"total": sessionItems.count,
			"periods": ["available": sessionsPeriodsAvailable, "items": sessionPeriodItems] as [String: Any],
			"items": sessionItems,
		]
		if includeWorkSignals {
			sessions["work_signals"] = workSignalsPayload ?? workSignals(available: workSignalsAvailable)
		}

		let provider: [String: Any] = [
			"available": true,
			"routes": routes,
			"candidates": candidates,
		]

		let snapshot: [String: Any] = [
			"wire_version": 1,
			"generated_at": generatedAt,
			"next_refresh_at": "2026-08-13T10:05:00Z",
			"provider": provider,
			"usage": usage,
			"sessions": sessions,
			"health": health,
		]

		let payload: [String: Any] = [
			"schema_version": 1,
			"command": "desktop.snapshot",
			"generated_at": generatedAt,
			"warnings": warnings,
			"partial": partial,
			"error": NSNull(),
			"data": snapshot,
		]
		let data = try! JSONSerialization.data(withJSONObject: payload)
		return try! decodeDesktopWireEnvelopeV1(data)
	}

	static let defaultRoute: [String: Any] = [
		"client": "codex",
		"provider": "official",
		"selected_at": "2026-08-13T09:55:00Z",
		"via_wrapper": false,
	]

	static let defaultCandidate: [String: Any] = [
		"provider": "aigocode",
		"built_in": false,
		"clients": ["codex"],
		"credentials": [["name": "work", "clients": ["codex"], "present": true]],
		"has_wrapper": true,
		"ready": true,
		"options": [
			[
				"client": "codex", "provider": "aigocode", "credential": "work",
				"via_wrapper": true, "ready": true, "reason_code": NSNull(),
			],
			[
				"client": "codex", "provider": "aigocode", "credential": "work",
				"via_wrapper": false, "ready": false, "reason_code": "wrapper_not_configured",
			],
		],
	]

	static var multiTargetCandidate: [String: Any] {
		var candidate = defaultCandidate
		candidate["options"] = [
			[
				"client": "codex", "provider": "aigocode", "credential": "work",
				"via_wrapper": true, "ready": true, "reason_code": NSNull(),
			],
			[
				"client": "codex", "provider": "aigocode", "credential": "work",
				"via_wrapper": false, "ready": true, "reason_code": NSNull(),
			],
		] as [[String: Any]]
		return candidate
	}

	static let defaultSessionItem: [String: Any] = [
		"client": "codex",
		"session_id": "session-secret-1",
		"project": "agent-deck",
		"model": "gpt-5",
		"first_at": "2026-08-13T09:40:00Z",
		"last_at": "2026-08-13T09:58:00Z",
	]

	static let healthyHealth: [String: Any] = [
		"available": true, "status": "ok", "healthy": true,
		"problems": 0, "warnings": 0, "errors": 0,
		"checks": [["name": "schema", "status": "ok", "code": NSNull(), "count": NSNull(), "recovery_command": NSNull()]],
	]

	static let warningHealth: [String: Any] = [
		"available": true, "status": "warning", "healthy": false,
		"problems": 2, "warnings": 2, "errors": 0,
		"checks": [
			["name": "schema", "status": "ok", "code": NSNull(), "count": NSNull(), "recovery_command": NSNull()],
			["name": "usage", "status": "warning", "code": "usage_stale", "count": NSNull(), "recovery_command": "agentdeck usage scan"],
			["name": "prices", "status": "warning", "code": "prices_stale", "count": NSNull(), "recovery_command": "agentdeck usage price update"],
		],
	]

	static let failingHealth: [String: Any] = [
		"available": true, "status": "error", "healthy": false,
		"problems": 2, "warnings": 1, "errors": 1,
		"checks": [
			["name": "schema", "status": "ok", "code": NSNull(), "count": NSNull(), "recovery_command": NSNull()],
			["name": "usage", "status": "warning", "code": "usage_stale", "count": NSNull(), "recovery_command": "agentdeck usage scan"],
			["name": "prices", "status": "error", "code": "prices_missing", "count": NSNull(), "recovery_command": "agentdeck usage price update"],
		],
	]
}

/// Drives the coordinator into an exact state. `refresh` either returns the
/// prepared envelope, throws the prepared issue, or suspends so a test can hold
/// the coordinator in `refreshing`.
@MainActor
final class StubDesktopHost: DesktopSnapshotRefreshing {
	enum Behavior {
		case envelope(DesktopWireEnvelopeV1)
		case failure(any Error)
		case suspendedEnvelope(DesktopWireEnvelopeV1)
	}

	var behavior: Behavior
	private(set) var refreshCount = 0
	private var continuation: CheckedContinuation<Void, Never>?

	init(behavior: Behavior) {
		self.behavior = behavior
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		refreshCount += 1
		switch behavior {
		case let .envelope(envelope):
			return envelope
		case let .failure(error):
			throw error
		case let .suspendedEnvelope(envelope):
			await withCheckedContinuation { continuation = $0 }
			return envelope
		}
	}

	func resume() {
		continuation?.resume()
		continuation = nil
	}
}

actor StubSwitchTransport: ProviderSwitching {
	private(set) var targets = [ProviderSwitchTarget]()
	private var outcome: ProviderSwitchTransportOutcome = .succeeded

	init(outcome: ProviderSwitchTransportOutcome = .succeeded) {
		self.outcome = outcome
	}

	func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome {
		targets.append(target)
		return outcome
	}

	func recordedTargets() -> [ProviderSwitchTarget] { targets }
}

final class StubLoginItemRegistrar: LoginItemRegistering {
	var status: SMAppService.Status = .notRegistered
	var registerError: (any Error)?
	var unregisterError: (any Error)?
	private(set) var registerCount = 0
	private(set) var unregisterCount = 0

	func register() throws {
		registerCount += 1
		if let registerError { throw registerError }
		status = .enabled
	}

	func unregister() throws {
		unregisterCount += 1
		if let unregisterError { throw unregisterError }
		status = .notRegistered
	}
}

/// Builds a view model over a coordinator already in the requested state.
@MainActor
func makeModel(
	host: StubDesktopHost,
	preferences: DesktopPreferences? = nil,
	transport: any ProviderSwitching = StubSwitchTransport(),
	// One minute after the fixtures' `generated_at`. A clock further out makes
	// every derived state additionally `aged`, which silently changes the empty
	// copy from "today" to "this snapshot" and would mask a real regression in
	// that rule.
	now: @escaping () -> Date = { Date(timeIntervalSince1970: 1_786_615_260) }
) async -> MenuBarViewModel {
	let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
	let controller = SwitchController(transport: transport, refreshCoordinator: coordinator)
	let resolvedPreferences = preferences ?? DesktopPreferences(
		defaults: isolatedDefaults(),
		registrar: StubLoginItemRegistrar()
	)
	return MenuBarViewModel(
		coordinator: coordinator,
		switchController: controller,
		preferences: resolvedPreferences,
		now: now
	)
}

func isolatedDefaults() -> UserDefaults {
	let suite = "com.kitdine.agentdeck.tests.\(UUID().uuidString)"
	let defaults = UserDefaults(suiteName: suite) ?? .standard
	defaults.removePersistentDomain(forName: suite)
	return defaults
}
