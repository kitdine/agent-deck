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
	static func schemaSignal(refusals: Bool = false, sessionsUnavailable: Bool = false, numbers: Bool = true, extraWarnings: [String] = []) -> DesktopWireEnvelopeV1 {
		var checks: [[String: Any]] = [["name": "database", "status": "error", "code": "schema_ahead", "count": numbers ? 99 : NSNull(), "supported_count": numbers ? 23 : NSNull()]]
		if refusals { checks.append(["name": "hook_deliveries", "status": "warning", "code": "hook_deliveries_dropped", "count": 2]) }
		return envelope(scopes: [], clientSubtotalsAvailable: false, presentationAvailable: false,
			sessionsAvailable: !sessionsUnavailable, sessionsPeriodsAvailable: !sessionsUnavailable,
			health: ["available": true, "status": "unhealthy", "healthy": false, "problems": checks.count, "warnings": refusals ? 1 : 0, "errors": 1, "checks": checks],
			warnings: ["provider_unavailable", "usage_unavailable"] + (sessionsUnavailable ? ["sessions_unavailable"] : []) + extraWarnings,
			partial: true, candidates: [], routes: [])
	}

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

	/// How a fixture snapshot carries `session_scope_view`: derived from the
	/// fixture's own sessions for the default selection (what the current
	/// producer publishes), absent (a legacy publication), or given verbatim.
	enum SessionScopeFixture {
		case derived
		case absent
		case custom(Any)
	}

	static func confirmedDefault(_ value: String = "work", revision: Int = 0) -> [String: Any] {
		["confirmed": true, "value": value, "revision": revision, "reason": NSNull()]
	}

	static let unconfirmedDefault: [String: Any] = ["confirmed": false, "value": NSNull(), "revision": NSNull(), "reason": "read_failed"]

	static func scopeRecent(
		client: String = "codex", id: String = "session-secret-1", project: Any = "agent-deck", model: String = "gpt-5",
		first: Any = "2026-08-13T09:40:00Z", last: Any = "2026-08-13T09:58:00Z",
		effective: String = "unknown", automatic: String = "unknown", reason: String = "origin_metadata_missing",
		rule: Any = "session-origin-reducer-v1", override: Any = NSNull(), revision: Int = 0
	) -> [String: Any] {
		[
			"client": client, "session_id": id, "project": project, "model": model, "first_at": first, "last_at": last,
			"origin": [
				"automatic": automatic, "effective": effective, "reason_code": reason, "rule_version": rule,
				"override": override, "revision": revision,
			] as [String: Any],
		]
	}

	/// The families of a `work_signals` payload restricted to one selection,
	/// as a view carries them.
	static func filteredWorkSignals(_ payload: [String: Any], period: String, client: String) -> [String: Any] {
		var filtered: [String: Any] = [:]
		for family in ["activity", "workflow", "tooling"] {
			let value = payload[family] as? [String: Any] ?? [:]
			let items = (value["items"] as? [[String: Any]] ?? []).filter {
				$0["period"] as? String == period && $0["client"] as? String == client
			}
			filtered[family] = ["available": value["available"] as? Bool ?? false, "items": items] as [String: Any]
		}
		return filtered
	}

	static func sessionScopeView(
		scope: String = "work", client: String = "all", period: String = "today",
		defaults: [String: Any] = confirmedDefault(),
		sessions: Int = 2, projects: Int = 2, knownDurationMS: Any = 7_200_000, missing: Int = 0,
		unknownCount: Any = 1, complete: Bool = true, reasons: [String] = [],
		recent: [[String: Any]] = [scopeRecent()],
		workSignals: [String: Any]? = nil,
		publicationID: String? = nil,
		generatedAt: String = "2026-08-13T10:00:00Z"
	) -> [String: Any] {
		[
			"available": true, "reason": NSNull(), "default": defaults,
			"selection": ["scope": scope, "client": client, "period": period],
			"publication_id": publicationID ?? "pub-\(scope)-\(client)-\(period)",
			"generated_at": generatedAt,
			"origin_coverage": ["complete": complete, "unknown_count": unknownCount, "reasons": reasons] as [String: Any],
			"counts": ["sessions": sessions, "projects": projects, "known_duration_ms": knownDurationMS, "missing_duration_count": missing] as [String: Any],
			"recent": recent,
			"work_signals": workSignals ?? filteredWorkSignals(WireFixture.workSignals(), period: period, client: client),
		]
	}

	static func unavailableSessionScopeView(
		reason: String, defaults: [String: Any] = confirmedDefault(), selection: [String: Any]? = ["scope": "work", "client": "all", "period": "today"]
	) -> [String: Any] {
		[
			"available": false, "reason": reason, "default": defaults, "selection": selection ?? NSNull(),
			"publication_id": NSNull(), "generated_at": NSNull(), "origin_coverage": NSNull(), "counts": NSNull(),
			"recent": [[String: Any]](), "work_signals": workSignals(available: false),
		]
	}

	static func decodedSessionScopeView(_ payload: [String: Any]) -> DesktopSessionScopeViewV1 {
		try! JSONDecoder().decode(DesktopSessionScopeViewV1.self, from: JSONSerialization.data(withJSONObject: payload))
	}

	/// A view for any selection, built from the same synthetic sessions: two
	/// unknown Work sessions and one maintenance session per populated client;
	/// Claude has none. Auxiliary never includes an unknown session.
	static func fixtureScopeView(_ selection: DesktopSessionScopeSelectionV1, defaults: [String: Any] = confirmedDefault()) -> DesktopSessionScopeViewV1 {
		let populated = selection.client != "claude"
		let work = [scopeRecent(), scopeRecent(client: "codex", id: "child-1", project: "ai-tools", reason: "task_child_unconfirmed", rule: "session-origin-reducer-v1")]
		let auxiliary = [scopeRecent(client: "codex", id: "maint-1", project: "agent-deck", effective: "auxiliary", automatic: "auxiliary", reason: "maintenance_origin", rule: "codex-otel-background-v1")]
		let rows: [[String: Any]]
		switch selection.scope {
		case .work: rows = populated ? work : []
		case .auxiliary: rows = populated ? auxiliary : []
		case .all: rows = populated ? work + auxiliary : []
		}
		let unknown = rows.filter { ($0["origin"] as? [String: Any])?["effective"] as? String == "unknown" }.count
		let projects = Set(rows.compactMap { $0["project"] as? String }).count
		let minutes = rows.count * 18
		return decodedSessionScopeView(sessionScopeView(
			scope: selection.scope.rawValue, client: selection.client, period: selection.period, defaults: defaults,
			sessions: rows.count, projects: projects, knownDurationMS: minutes * 60_000,
			unknownCount: unknown, recent: rows
		))
	}

	/// Every supported selection's fixture view, precomputed so a nonisolated
	/// reader can answer from it.
	static func fixtureScopeViews(defaults: [String: Any] = confirmedDefault()) -> [DesktopSessionScopeSelectionV1: DesktopSessionScopeViewV1] {
		var views: [DesktopSessionScopeSelectionV1: DesktopSessionScopeViewV1] = [:]
		for scope in DesktopSessionScopeV1.allCases {
			for client in DesktopSessionScopeSelectionV1.clients {
				for period in DesktopSessionScopeSelectionV1.periods {
					let selection = DesktopSessionScopeSelectionV1(scope: scope, client: client, period: period)
					views[selection] = fixtureScopeView(selection, defaults: defaults)
				}
			}
		}
		return views
	}

	static func envelope(
		sessionScope: SessionScopeFixture = .derived,
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

		var snapshot: [String: Any] = [
			"wire_version": 1,
			"generated_at": generatedAt,
			"next_refresh_at": "2026-08-13T10:05:00Z",
			"provider": provider,
			"usage": usage,
			"sessions": sessions,
			"health": health,
		]
		switch sessionScope {
		case .absent:
			break
		case let .custom(view):
			snapshot["session_scope_view"] = view
		case .derived:
			if !sessionsAvailable {
				snapshot["session_scope_view"] = unavailableSessionScopeView(reason: "sessions_unavailable")
			} else {
				let signals = includeWorkSignals
					? filteredWorkSignals(workSignalsPayload ?? workSignals(available: workSignalsAvailable), period: "today", client: "all")
					: workSignals(available: false)
				let rows = sessionItems.map { item in
					scopeRecent(client: item["client"] as? String ?? "codex", id: item["session_id"] as? String ?? "",
						project: item["project"] ?? NSNull(), model: item["model"] as? String ?? "",
						first: item["first_at"] ?? NSNull(), last: item["last_at"] ?? NSNull())
				}
				snapshot["session_scope_view"] = sessionScopeView(
					sessions: max(2, rows.count), unknownCount: max(2, rows.count), recent: rows, workSignals: signals,
					generatedAt: generatedAt
				)
			}
		}

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
		case suspendedFailure(any Error)
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
		case let .suspendedFailure(error):
			await withCheckedContinuation { continuation = $0 }
			throw error
		}
	}

	func refresh(recentLimit: Int, progress: @escaping @Sendable (DesktopScanProgress) -> Void) async throws -> DesktopWireEnvelopeV1 {
		progress(.waiting)
		if case .suspendedEnvelope = behavior {
			progress(DesktopScanProgress(
				sequence: 1,
				stage: .importing,
				usage: DesktopScanDomainProgress(state: "processing", committed: 3, total: 8, skipped: 0),
				session: DesktopScanDomainProgress(state: "processing", committed: 2, total: 8, skipped: 1)
			))
		} else if case .suspendedFailure = behavior {
			progress(DesktopScanProgress(
				sequence: 1,
				stage: .importing,
				usage: DesktopScanDomainProgress(state: "processing", committed: 3, total: 8, skipped: 0),
				session: DesktopScanDomainProgress(state: "processing", committed: 2, total: 8, skipped: 1)
			))
		}
		if case .failure = behavior {
			progress(DesktopScanProgress(
				sequence: 2,
				stage: .completed,
				usage: DesktopScanDomainProgress(state: "completed", committed: 8, total: 8, skipped: 0),
				session: DesktopScanDomainProgress(state: "failed", committed: 0, total: 0, skipped: 0)
			))
		}
		return try await refresh(recentLimit: recentLimit)
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

/// A fake `QuotaSettingsTransport` that behaves like the real CLI round trip
/// without spawning a subprocess: `applyQuotaSettings`/`setQuotaStatusLine`
/// update the same in-memory record `loadQuotaSettings` next reads, matching
/// core state's own read-your-writes behavior.
actor StubQuotaSettingsTransport: QuotaSettingsTransport {
	private var settings: DesktopQuotaSettingsValuesV1
	private var statuslineOutcome: DesktopUsageHookOutcomeV1
	private(set) var applyCalls = [DesktopQuotaSettingsDesiredV1]()
	private(set) var statuslineCalls = [Bool]()

	static let defaultSettings = DesktopQuotaSettingsValuesV1(
		reading: false, interval: .fiveMinutes, alerts: false, thresholds: [], resetNotice: false, statusline: false
	)

	init(
		settings: DesktopQuotaSettingsValuesV1 = StubQuotaSettingsTransport.defaultSettings,
		statuslineOutcome: DesktopUsageHookOutcomeV1 = .configured
	) {
		self.settings = settings
		self.statuslineOutcome = statuslineOutcome
	}

	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		.decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil))
	}

	func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		applyCalls.append(desired)
		let wasReading = settings.reading
		let disabledClaude = settings.clients.contains("claude") && !desired.clients.contains("claude")
		settings = DesktopQuotaSettingsValuesV1(
			reading: desired.reading, clients: desired.clients, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice,
			// Mirrors runDesktopQuotaSettings: turning reading off or disabling Claude clears consent.
			statusline: (wasReading && !desired.reading) || disabledClaude ? false : settings.statusline
		)
		let restore: DesktopUsageHookResultV1? = ((wasReading && !desired.reading) || disabledClaude) && statuslineOutcome != .configured
			? DesktopUsageHookResultV1(outcome: statuslineOutcome)
			: nil
		return .decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: restore))
	}

	func setQuotaStatusLine(enabled: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> {
		statuslineCalls.append(enabled)
		let succeeded = statuslineOutcome != .failed
		let consent = enabled ? succeeded : false
		settings = DesktopQuotaSettingsValuesV1(
			reading: settings.reading, clients: settings.clients, interval: settings.interval, alerts: settings.alerts,
			thresholds: settings.thresholds, resetNotice: settings.resetNotice, statusline: consent
		)
		return .decoded(DesktopQuotaStatusLineResultV1(consent: consent, result: DesktopUsageHookResultV1(outcome: statuslineOutcome)))
	}
}

/// A trivial async-safe counter for asserting how many times an injected
/// closure (e.g. a refresh trigger) fired, without racing a plain `var`.
actor ActorCounter {
	private(set) var count = 0
	func increment() { count += 1 }
}

/// Every call fails to decode, as a real launch failure (missing helper,
/// timeout, or non-JSON stdout) would — as distinct from `StubQuotaSettingsTransport`
/// deliberately returning a decoded `failed`/`restore_incomplete` sub-result.
struct AlwaysUndecodableQuotaSettingsTransport: QuotaSettingsTransport {
	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> { .undecodable }
	func applyQuotaSettings(_: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> { .undecodable }
	func setQuotaStatusLine(enabled _: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> { .undecodable }
}

@MainActor
func makeQuotaSettingsController(
	preferences: DesktopPreferences? = nil,
	transport: any QuotaSettingsTransport = StubQuotaSettingsTransport(),
	claudeSettingsURL: URL = URL(fileURLWithPath: "/nonexistent/agentdeck-test/.claude/settings.json"),
	notifications: any NotificationPermissionChecking = StubNotificationPermission(granted: true),
	refreshQuotaSnapshot: (() async -> Void)? = nil
) -> QuotaSettingsController {
	QuotaSettingsController(
		preferences: preferences ?? DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar()),
		transport: transport,
		claudeSettingsURL: claudeSettingsURL,
		notifications: notifications,
		refreshQuotaSnapshot: refreshQuotaSnapshot
	)
}

/// Stands in for the notification service so hosted tests never reach it or
/// its permission prompt.
actor StubNotificationPermission: NotificationPermissionChecking {
	private let granted: Bool
	private(set) var requestCount = 0
	private(set) var checkCount = 0

	init(granted: Bool) {
		self.granted = granted
	}

	func authorizationGranted() async -> Bool {
		checkCount += 1
		return granted
	}

	func requestAuthorization() async -> Bool {
		requestCount += 1
		return granted
	}
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

/// Explicit session-scope reads under test control. Every request is recorded;
/// a held request waits until the test releases it, so late answers and
/// overlapping requests are ordered by the test, not by timing.
actor StubSessionScopeReader: DesktopSessionScopeReading {
	enum Answer: Sendable {
		case view(DesktopSessionScopeViewV1)
		case failure(HelperExecutionError)
	}

	private(set) var requests: [DesktopSessionScopeSelectionV1] = []
	private var holding = false
	private var held: [Int: CheckedContinuation<Void, Never>] = [:]
	private var overrides: [Int: Answer] = [:]
	private let answer: @Sendable (DesktopSessionScopeSelectionV1) -> Answer

	init(answer: @escaping @Sendable (DesktopSessionScopeSelectionV1) -> Answer) {
		self.answer = answer
	}

	func hold(_ value: Bool) { holding = value }

	/// Answers the request at `index` with `answer` instead of its selection's
	/// own, e.g. the cancellation a superseded read ends with.
	func answer(request index: Int, with answer: Answer) { overrides[index] = answer }

	func release(_ index: Int) {
		held.removeValue(forKey: index)?.resume()
	}

	func heldCount() -> Int { held.count }

	func sessionScopeView(_ selection: DesktopSessionScopeSelectionV1, recentLimit: Int) async throws -> DesktopSessionScopeViewV1 {
		requests.append(selection)
		let index = requests.count - 1
		if holding {
			await withCheckedContinuation { held[index] = $0 }
		}
		switch overrides.removeValue(forKey: index) ?? answer(selection) {
		case let .view(view): return view
		case let .failure(error): throw error
		}
	}
}

/// Builds a view model over a coordinator already in the requested state.
@MainActor
func makeModel(
	host: StubDesktopHost,
	preferences: DesktopPreferences? = nil,
	transport: any ProviderSwitching = StubSwitchTransport(),
	snapshotStore: AppGroupSnapshotStore? = nil,
	sessionScopeReader: (any DesktopSessionScopeReading)? = nil,
	// One minute after the fixtures' `generated_at`. A clock further out makes
	// every derived state additionally `aged`, which silently changes the empty
	// copy from "today" to "this snapshot" and would mask a real regression in
	// that rule.
	now: @escaping () -> Date = { Date(timeIntervalSince1970: 1_786_615_260) }
) async -> MenuBarViewModel {
	let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: snapshotStore)
	let controller = SwitchController(transport: transport, refreshCoordinator: coordinator)
	let resolvedPreferences = preferences ?? DesktopPreferences(
		defaults: isolatedDefaults(),
		registrar: StubLoginItemRegistrar()
	)
	return MenuBarViewModel(
		coordinator: coordinator,
		switchController: controller,
		preferences: resolvedPreferences,
		sessionScopeReader: sessionScopeReader,
		now: now
	)
}

func isolatedDefaults() -> UserDefaults {
	let suite = "com.kitdine.agentdeck.tests.\(UUID().uuidString)"
	let defaults = UserDefaults(suiteName: suite) ?? .standard
	defaults.removePersistentDomain(forName: suite)
	return defaults
}

/// Records telemetry helper calls. Status answers come from `statuses` in
/// order, repeating the last one; enable/disable answers come from the
/// matching closures, nil meaning a refused (undecodable) helper result.
final class StubTelemetryTransport: TelemetryTransport, @unchecked Sendable {
	private let lock = NSLock()
	private var statuses: [DesktopTelemetryStatusV1]
	private let enableResult: (Bool) -> DesktopTelemetryReceiverResultV1?
	private let disableResult: () -> DesktopTelemetryReceiverResultV1?
	private(set) var enableCalls: [Bool] = []
	private(set) var disableCalls = 0
	private(set) var previewLoads = 0

	init(
		statuses: [DesktopTelemetryStatusV1] = [DesktopTelemetryStatusV1(receiver: "disabled", listener: "stopped", metering: "disabled")],
		enableResult: @escaping (Bool) -> DesktopTelemetryReceiverResultV1? = { write in DesktopTelemetryReceiverResultV1(receiver: "enabled", codexConfig: write ? "written" : "unchanged") },
		disableResult: @escaping () -> DesktopTelemetryReceiverResultV1? = { DesktopTelemetryReceiverResultV1(receiver: "disabled") }
	) {
		self.statuses = statuses
		self.enableResult = enableResult
		self.disableResult = disableResult
	}

	func loadTelemetryStatus() async -> DesktopQuotaTransportOutcome<DesktopTelemetryStatusV1> {
		.decoded(lock.withLock { statuses.count > 1 ? statuses.removeFirst() : statuses[0] })
	}

	func enableTelemetryReceiver(writeCodexConfig: Bool) async -> DesktopQuotaTransportOutcome<DesktopTelemetryReceiverResultV1> {
		lock.withLock { enableCalls.append(writeCodexConfig) }
		return enableResult(writeCodexConfig).map { .decoded($0) } ?? .undecodable
	}

	func disableTelemetryReceiver() async -> DesktopQuotaTransportOutcome<DesktopTelemetryReceiverResultV1> {
		lock.withLock { disableCalls += 1 }
		return disableResult().map { .decoded($0) } ?? .undecodable
	}

	func loadTelemetryPreview() async -> DesktopQuotaTransportOutcome<DesktopTelemetryPreviewV1> {
		lock.withLock { previewLoads += 1 }
		return .decoded(DesktopTelemetryPreviewV1(snippet: "[otel]\n<redacted>"))
	}
}

@MainActor
final class StubTelemetryListenerSupervisor: TelemetryListenerSupervising {
	private(set) var isRunning = false
	private(set) var starts = 0
	private(set) var stops = 0

	func start() {
		starts += 1
		isRunning = true
	}

	func stop() {
		stops += 1
		isRunning = false
	}
}

@MainActor
func makeTelemetrySettingsController(
	transport: any TelemetryTransport = StubTelemetryTransport(),
	supervisor: any TelemetryListenerSupervising = StubTelemetryListenerSupervisor()
) -> TelemetrySettingsController {
	TelemetrySettingsController(transport: transport, supervisor: supervisor, settleDelay: .zero)
}
