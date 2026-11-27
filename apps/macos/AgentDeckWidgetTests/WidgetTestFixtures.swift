import Foundation
import XCTest

func widgetFixture(_ name: String) throws -> WidgetDesktopSnapshotV1 {
	let bundle = Bundle(for: WidgetFixtureBundleMarker.self)
	let url = try XCTUnwrap(bundle.url(forResource: name, withExtension: "json"))
	let envelope = try decodeDesktopWireEnvelopeV1(Data(contentsOf: url))
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONEncoder().encode(AppGroupDesktopSnapshotV1(envelope: envelope))
	)
}

func snapshotWithoutClient(_ snapshot: WidgetDesktopSnapshotV1, client: String) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	let scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	presentation["scopes"] = scopes.filter { ($0["client"] as? String) != client }
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

func snapshotWithEmptyToday(_ snapshot: WidgetDesktopSnapshotV1, client: String) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	let index = try XCTUnwrap(scopes.firstIndex { ($0["client"] as? String) == client })
	var periods = try XCTUnwrap(scopes[index]["periods"] as? [String: Any])
	var items = try XCTUnwrap(periods["items"] as? [[String: Any]])
	let today = try XCTUnwrap(items.firstIndex { ($0["period"] as? String) == "today" })
	var totals = try XCTUnwrap(items[today]["totals"] as? [String: Any])
	for key in ["tokens", "input_tokens", "output_tokens", "cached_read_tokens", "cache_write_tokens", "events", "sessions"] {
		totals[key] = 0
	}
	items[today]["totals"] = totals
	periods["items"] = items
	scopes[index]["periods"] = periods
	presentation["scopes"] = scopes
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

func snapshotWithPartial(_ snapshot: WidgetDesktopSnapshotV1) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	object["partial"] = true
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

/// Sets both the client-level observed_at and, matching the real wire
/// (BuildSubscription stamps every window from the same probe cycle with the
/// same instant in the ordinary case), each of that client's windows' own
/// observed_at -- quotaFooterObservedAt reads from the windows actually
/// displayed, not the client-level aggregate.
func snapshotWithQuotaObservedAt(_ snapshot: WidgetDesktopSnapshotV1, values: [String: String]) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
	var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
	for index in clients.indices {
		guard let client = clients[index]["client"] as? String, let value = values[client] else { continue }
		clients[index]["observed_at"] = value
		if var windows = clients[index]["windows"] as? [[String: Any]] {
			for windowIndex in windows.indices {
				windows[windowIndex]["observed_at"] = value
			}
			clients[index]["windows"] = windows
		}
	}
	subscription["clients"] = clients
	object["subscription"] = subscription
	return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
}

/// Sets one client's own `stale` flag, leaving every other field (including
/// the snapshot-wide generatedAt) untouched -- for proving the quota
/// qualifier derives from this producer-computed flag rather than the
/// generic snapshot-age ladder.
func snapshotWithQuotaStale(_ snapshot: WidgetDesktopSnapshotV1, client: String, stale: Bool) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
	var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
	let clientIndex = try XCTUnwrap(clients.firstIndex { ($0["client"] as? String) == client })
	clients[clientIndex]["stale"] = stale
	subscription["clients"] = clients
	object["subscription"] = subscription
	return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
}

/// Sets one specific window's own observed_at, leaving the client-level
/// aggregate and every other window untouched -- for exercising the mixed-age
/// case a partial update leaves behind.
func snapshotWithWindowObservedAt(_ snapshot: WidgetDesktopSnapshotV1, client: String, windowKey: String, value: String) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
	var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
	let clientIndex = try XCTUnwrap(clients.firstIndex { ($0["client"] as? String) == client })
	var windows = try XCTUnwrap(clients[clientIndex]["windows"] as? [[String: Any]])
	let windowIndex = try XCTUnwrap(windows.firstIndex { ($0["key"] as? String) == windowKey })
	windows[windowIndex]["observed_at"] = value
	clients[clientIndex]["windows"] = windows
	subscription["clients"] = clients
	object["subscription"] = subscription
	return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
}

/// Every client with reading off: no windows, and failure == probe_disabled,
/// matching the desktop wire's real shape for that state (subscription.go).
func snapshotWithQuotaReadingOff(_ snapshot: WidgetDesktopSnapshotV1) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var subscription = try XCTUnwrap(object["subscription"] as? [String: Any])
	var clients = try XCTUnwrap(subscription["clients"] as? [[String: Any]])
	for index in clients.indices {
		clients[index]["windows"] = []
		clients[index]["failure"] = "probe_disabled"
		clients[index]["tightest_window_key"] = NSNull()
	}
	subscription["clients"] = clients
	object["subscription"] = subscription
	return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: object))
}

func snapshotWithPricedToday(_ snapshot: WidgetDesktopSnapshotV1, client: String) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	let index = try XCTUnwrap(scopes.firstIndex { ($0["client"] as? String) == client })

	var quality = try XCTUnwrap(scopes[index]["quality"] as? [String: Any])
	var qualityItems = try XCTUnwrap(quality["items"] as? [[String: Any]])
	let aggregate = try XCTUnwrap(qualityItems.firstIndex {
		($0["period"] as? String) == "today" && ($0["provider"] == nil || $0["provider"] is NSNull)
	})
	var tiers = try XCTUnwrap(qualityItems[aggregate]["tiers"] as? [[String: Any]])
	let shares = ["determinable": "62.50", "inferred": "25.00", "unattributed": "12.50"]
	let costs = ["determinable": "1.250000000", "inferred": "0.500000000", "unattributed": "0.250000000"]
	for position in tiers.indices {
		let name = try XCTUnwrap(tiers[position]["quality"] as? String)
		tiers[position]["share"] = try XCTUnwrap(shares[name])
		var value = try XCTUnwrap(tiers[position]["value"] as? [String: Any])
		value["provider_cost"] = try XCTUnwrap(costs[name])
		value["cost_incomplete"] = false
		tiers[position]["value"] = value
	}
	qualityItems[aggregate]["tiers"] = tiers
	quality["items"] = qualityItems
	scopes[index]["quality"] = quality

	var pricing = try XCTUnwrap(scopes[index]["pricing"] as? [String: Any])
	var pricingItems = try XCTUnwrap(pricing["items"] as? [[String: Any]])
	let today = try XCTUnwrap(pricingItems.firstIndex { ($0["period"] as? String) == "today" })
	pricingItems[today]["priced_events"] = pricingItems[today]["unpriced_events"]
	pricingItems[today]["unpriced_events"] = 0
	pricingItems[today]["coverage"] = "100.00"
	pricingItems[today]["unpriced_identifiers"] = [String]()
	pricing["items"] = pricingItems
	scopes[index]["pricing"] = pricing

	presentation["scopes"] = scopes
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

func snapshotWithModels(
	_ snapshot: WidgetDesktopSnapshotV1,
	modelsByScopeAndPeriod: [String: [String: [(client: String?, model: String, tokens: Int64, share: String)]]]
) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	for scopeIndex in scopes.indices {
		guard let client = scopes[scopeIndex]["client"] as? String,
			let byPeriod = modelsByScopeAndPeriod[client]
		else { continue }
		var periods = try XCTUnwrap(scopes[scopeIndex]["periods"] as? [String: Any])
		var items = try XCTUnwrap(periods["items"] as? [[String: Any]])
		for periodIndex in items.indices {
			guard let periodName = items[periodIndex]["period"] as? String,
				let entries = byPeriod[periodName]
			else { continue }
			let totalTokens = entries.reduce(Int64(0)) { $0 + $1.tokens }
			if var totals = items[periodIndex]["totals"] as? [String: Any] {
				totals["tokens"] = totalTokens
				items[periodIndex]["totals"] = totals
			}
			items[periodIndex]["models"] = entries.map { entry in
				[
					"client": entry.client ?? NSNull(),
					"model": entry.model,
					"value": [
						"tokens": entry.tokens,
						"events": Int64(1),
						"provider_cost": String(format: "%.6f", Double(entry.tokens) / 1_000),
						"cost_incomplete": false,
					] as [String: Any],
					"share": entry.share,
				] as [String: Any]
			}
		}
		periods["items"] = items
		scopes[scopeIndex]["periods"] = periods
	}
	presentation["scopes"] = scopes
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

private final class WidgetFixtureBundleMarker {}

func identityWidgetFixture(_ edit: (inout [[String: Any]]) -> Void = { _ in }) throws -> WidgetDesktopSnapshotV1 {
	let original = try widgetFixture("snapshot-complete")
	var root = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
	var sub = try XCTUnwrap(root["subscription"] as? [String: Any])
	var clients = try XCTUnwrap(sub["clients"] as? [[String: Any]])
	for i in clients.indices {
		clients[i]["identity"] = [
			"state": "identified", "freshness": "current", "current_until": "2026-08-13T10:00:30Z",
			"email_masked": "a***e@\(String(repeating: "long.", count: 20))example.test",
		]
	}
	edit(&clients)
	sub["clients"] = clients
	root["subscription"] = sub
	return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: JSONSerialization.data(withJSONObject: root))
}

/// Parts of one scope+period's totals: (tokens, input, output, cacheRead, cacheWrite).
typealias WidgetTokenParts = (tokens: Int64, input: Int64, output: Int64, cacheRead: Int64, cacheWrite: Int64)

func snapshotWithTokenParts(_ snapshot: WidgetDesktopSnapshotV1, _ overrides: [String: [String: WidgetTokenParts]]) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	for (client, periods) in overrides {
		let index = try XCTUnwrap(scopes.firstIndex { ($0["client"] as? String) == client })
		var scopePeriods = try XCTUnwrap(scopes[index]["periods"] as? [String: Any])
		var items = try XCTUnwrap(scopePeriods["items"] as? [[String: Any]])
		for (period, parts) in periods {
			let at = try XCTUnwrap(items.firstIndex { ($0["period"] as? String) == period })
			var totals = try XCTUnwrap(items[at]["totals"] as? [String: Any])
			totals["tokens"] = parts.tokens
			totals["input_tokens"] = parts.input
			totals["output_tokens"] = parts.output
			totals["cached_read_tokens"] = parts.cacheRead
			totals["cache_write_tokens"] = parts.cacheWrite
			items[at]["totals"] = totals
		}
		scopePeriods["items"] = items
		scopes[index]["periods"] = scopePeriods
	}
	presentation["scopes"] = scopes
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}

func snapshotWithUnavailablePeriods(_ snapshot: WidgetDesktopSnapshotV1, client: String) throws -> WidgetDesktopSnapshotV1 {
	var object = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)) as? [String: Any])
	var usage = try XCTUnwrap(object["usage"] as? [String: Any])
	var presentation = try XCTUnwrap(usage["presentation"] as? [String: Any])
	var scopes = try XCTUnwrap(presentation["scopes"] as? [[String: Any]])
	let index = try XCTUnwrap(scopes.firstIndex { ($0["client"] as? String) == client })
	var periods = try XCTUnwrap(scopes[index]["periods"] as? [String: Any])
	periods["available"] = false
	scopes[index]["periods"] = periods
	presentation["scopes"] = scopes
	usage["presentation"] = presentation
	object["usage"] = usage
	return try JSONDecoder().decode(
		WidgetDesktopSnapshotV1.self,
		from: JSONSerialization.data(withJSONObject: object)
	)
}
