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

private final class WidgetFixtureBundleMarker {}
