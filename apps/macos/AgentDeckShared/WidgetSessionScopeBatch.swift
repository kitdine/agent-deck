import Foundation

/// Reads one explicit session-scope selection for the Widget batch. The host
/// passes the batch's pinned generation (nil for the first read) and the one
/// instant every selection of the batch is answered at.
public protocol WidgetSessionScopeReading: Sendable {
	func widgetSessionScopeView(
		_ selection: DesktopSessionScopeSelectionV1,
		generationID: String?,
		asOf: String
	) async throws -> DesktopSessionScopeViewV1

	/// The whole batch in one read: `scope` for every client and period, in
	/// `WidgetSessionScopeBatchCollector.selections` order, at one instant and
	/// pinned by the producer to one generation. nil means this reader answers
	/// one selection at a time and the collector reads them itself.
	func widgetSessionScopeBatch(
		scope: DesktopSessionScopeV1,
		asOf: String
	) async throws -> [DesktopSessionScopeViewV1]?

	/// The batch the process that produced `publication` already answered and
	/// handed over with it, or nil when there is none: the collector then
	/// reads one itself. It carries the instant it was answered at.
	func widgetSessionScopeDelivered(publication: String) async -> WidgetSessionScopeDelivery?
}

/// A whole batch answered by the producer of a publication, with its instant.
public struct WidgetSessionScopeDelivery: Equatable, Sendable {
	public let asOf: String
	public let views: [DesktopSessionScopeViewV1]

	public init(asOf: String, views: [DesktopSessionScopeViewV1]) {
		self.asOf = asOf
		self.views = views
	}
}

public extension WidgetSessionScopeReading {
	func widgetSessionScopeDelivered(publication _: String) async -> WidgetSessionScopeDelivery? { nil }

	func widgetSessionScopeBatch(
		scope _: DesktopSessionScopeV1,
		asOf _: String
	) async throws -> [DesktopSessionScopeViewV1]? { nil }
}

public enum WidgetSessionScopeBatchOutcome: Equatable, Sendable {
	/// Every supported client × period answered at one confirmed default, one
	/// committed generation and one time.
	case accepted(AppGroupSessionScopeSummarySetV1)
	/// No complete batch: the caller keeps the previous accepted set as stale,
	/// or publishes the reason as unavailable when there is none.
	case rejected(reason: String)
}

/// Builds the Widget's session-scope summaries (Task 7's production path).
///
/// The host never counts sessions itself: every figure is Go's answer for an
/// explicit selection whose scope is the confirmed persistent default — never
/// the Popover's temporary choice. The production reader answers the whole
/// batch from one helper process, so a publication starts one helper rather
/// than nine; a reader without that form is read one selection at a time.
/// When the producer of the publication being published already handed its
/// batch over, that one is checked first and no helper is started at all.
/// Either way the first view (all × today) fixes the batch's generation and
/// every later one is pinned to it, so Go answers from that generation or
/// reports it unavailable instead of silently answering from a newer one. A changed default, a moved generation, a mismatched
/// selection, identity or time, or any missing combination discards the whole
/// candidate; consistency failures retry at most `maximumRetries` times.
public struct WidgetSessionScopeBatchCollector: Sendable {
	public static let maximumRetries = 2
	/// all × today first: it is the read that fixes the batch identity.
	public static let selections: [(client: String, period: String)] = DesktopSessionScopeSelectionV1.clients.flatMap { client in
		DesktopSessionScopeSelectionV1.periods.map { (client: client, period: $0) }
	}

	private let reader: any WidgetSessionScopeReading
	private let now: @Sendable () -> Date

	public init(reader: any WidgetSessionScopeReading, now: @escaping @Sendable () -> Date = Date.init) {
		self.reader = reader
		self.now = now
	}

	public func collect(default initial: DesktopSessionScopeDefaultV1, publication: String? = nil) async -> WidgetSessionScopeBatchOutcome {
		await collectWithViews(default: initial, publication: publication).outcome
	}

	/// The batch and, when it is accepted, the complete views it was built
	/// from: the confirmed default scope for every client and period at one
	/// generation. The Popover shows them when the user changes client or
	/// period, instead of starting a read of its own for a view this batch
	/// already holds.
	public func collectWithViews(
		default initial: DesktopSessionScopeDefaultV1,
		publication: String? = nil
	) async -> (outcome: WidgetSessionScopeBatchOutcome, views: [DesktopSessionScopeViewV1]) {
		var expected = initial
		var reason = AppGroupSessionScopeSummaryReason.batchInconsistent
		// A handed-over batch passes the same checks as a read one. Anything
		// short of acceptance falls through to the reads below, with their
		// full retry budget.
		if expected.confirmed, let scope = expected.value, let publication,
			let delivery = await reader.widgetSessionScopeDelivered(publication: publication),
			delivery.views.count == Self.selections.count,
			case let .accepted(set, views) = await attempt(scope: scope, default: expected, delivery: delivery) {
			return (.accepted(set), views)
		}
		for _ in 0 ... Self.maximumRetries {
			guard expected.confirmed, let scope = expected.value else {
				// An unconfirmed default starts no batch; it is never Work.
				return (.rejected(reason: AppGroupSessionScopeSummaryReason.defaultUnconfirmed), [])
			}
			switch await attempt(scope: scope, default: expected) {
			case let .accepted(set, views):
				return (.accepted(set), views)
			case let .retry(failure, adopted):
				reason = failure
				if let adopted { expected = adopted }
			case let .final(failure):
				return (.rejected(reason: failure), [])
			}
		}
		return (.rejected(reason: reason), [])
	}

	private enum Attempt {
		case accepted(AppGroupSessionScopeSummarySetV1, [DesktopSessionScopeViewV1])
		/// A consistency failure worth another full batch; `adopted` is a newer
		/// confirmed default the reads reported.
		case retry(String, adopted: DesktopSessionScopeDefaultV1?)
		case final(String)
	}

	private func attempt(
		scope: DesktopSessionScopeV1, default expected: DesktopSessionScopeDefaultV1,
		delivery: WidgetSessionScopeDelivery? = nil
	) async -> Attempt {
		let asOf = delivery?.asOf ?? Self.timestamp(now())
		guard let asOfDate = Self.date(asOf) else { return .final(AppGroupSessionScopeSummaryReason.selectionUnavailable) }
		var generation: String?
		var items = [AppGroupSessionScopeSummaryV1]()
		var views = [DesktopSessionScopeViewV1]()
		let batch: [DesktopSessionScopeViewV1]?
		if let delivery {
			batch = delivery.views
		} else {
			do {
				batch = try await reader.widgetSessionScopeBatch(scope: scope, asOf: asOf)
			} catch {
				return .final(AppGroupSessionScopeSummaryReason.selectionUnavailable)
			}
		}
		if let batch, batch.count != Self.selections.count {
			return .retry(AppGroupSessionScopeSummaryReason.batchInconsistent, adopted: nil)
		}
		for (index, (client, period)) in Self.selections.enumerated() {
			let selection = DesktopSessionScopeSelectionV1(scope: scope, client: client, period: period)
			let view: DesktopSessionScopeViewV1
			if let batch {
				// Checked like a single read below: selection, default,
				// generation and time must all agree.
				view = batch[index]
			} else {
				do {
					view = try await reader.widgetSessionScopeView(selection, generationID: generation, asOf: asOf)
				} catch {
					return .final(AppGroupSessionScopeSummaryReason.selectionUnavailable)
				}
			}
			guard view.default == expected else {
				let adopted = view.default.confirmed ? view.default : nil
				return .retry(AppGroupSessionScopeSummaryReason.defaultChanged, adopted: adopted)
			}
			guard view.available else {
				let failure = AppGroupSessionScopeSummaryReason.safe(view.reason ?? AppGroupSessionScopeSummaryReason.selectionUnavailable)
				if failure == AppGroupSessionScopeSummaryReason.generationUnavailable || failure == "selection_stale" {
					return .retry(failure, adopted: nil)
				}
				return .final(failure)
			}
			guard view.selection == selection, let counts = view.counts, let coverage = view.originCoverage,
				let publication = view.generationID, !publication.isEmpty,
				generation == nil || generation == publication,
				let generatedAt = view.generatedAt, Self.date(generatedAt) == asOfDate
			else {
				return .retry(AppGroupSessionScopeSummaryReason.batchInconsistent, adopted: nil)
			}
			generation = publication
			views.append(view)
			guard let unknown = Self.unknownCount(scope: scope, sessions: counts.sessions, coverage: coverage) else {
				return .final(AppGroupSessionScopeSummaryReason.batchInconsistent)
			}
			items.append(AppGroupSessionScopeSummaryV1(
				scope: scope, count: counts.sessions, unknownCount: unknown, client: client, period: period,
				publicationID: publication, generatedAt: asOf
			))
		}
		guard items.count == Self.selections.count else {
			return .final(AppGroupSessionScopeSummaryReason.batchInconsistent)
		}
		return .accepted(AppGroupSessionScopeSummarySetV1(stale: false, reason: nil, items: items), views)
	}

	/// Unknown sessions of the selected set, or nil (outer) when the answer is
	/// inconsistent. Coverage that is not complete leaves the figure unknown
	/// (`.some(nil)`, shown as —), never a guessed 0. Auxiliary holds only
	/// confirmed auxiliary sessions, so anything but 0 there rejects the batch.
	private static func unknownCount(
		scope: DesktopSessionScopeV1, sessions: Int, coverage: DesktopSessionOriginCoverageV1
	) -> Int?? {
		if let count = coverage.unknownCount, count < 0 || count > sessions { return nil }
		if scope == .auxiliary {
			return coverage.unknownCount == nil || coverage.unknownCount == 0 ? .some(0) : nil
		}
		guard coverage.complete, let count = coverage.unknownCount else { return .some(nil) }
		return .some(count)
	}

	static func timestamp(_ date: Date) -> String {
		let formatter = ISO8601DateFormatter()
		formatter.formatOptions = [.withInternetDateTime]
		formatter.timeZone = TimeZone(secondsFromGMT: 0)
		return formatter.string(from: date)
	}

	static func date(_ value: String) -> Date? {
		let fractional = ISO8601DateFormatter()
		fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		return fractional.date(from: value) ?? ISO8601DateFormatter().date(from: value)
	}
}
