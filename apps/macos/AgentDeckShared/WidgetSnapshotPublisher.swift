import Foundation
import WidgetKit

public struct WidgetTimelineReloader: Sendable {
	private let reloadKinds: @Sendable (Set<AppGroupWidgetKind>) -> Void

	public init(reloadKinds: @escaping @Sendable (Set<AppGroupWidgetKind>) -> Void) {
		self.reloadKinds = reloadKinds
	}

	public func reload(kinds: Set<AppGroupWidgetKind>) {
		reloadKinds(kinds)
	}

	public static let live = WidgetTimelineReloader { kinds in
		for kind in kinds {
			WidgetCenter.shared.reloadTimelines(ofKind: kind.widgetKitKindIdentifier)
		}
	}
}

public enum WidgetSnapshotPublisherIssue: Equatable, Sendable {
	case unsafeOutput
	case oversizedOutput
	case storageUnavailable
}

public enum WidgetSnapshotPublicationResult: Equatable, Sendable {
	case published(affectedKinds: Set<AppGroupWidgetKind>)
	case superseded
	case failedBeforeCommit(retainedVerifiedGeneration: UInt64?, issue: WidgetSnapshotPublisherIssue)
	case indeterminateAfterCommit(attemptedGeneration: UInt64, issue: WidgetSnapshotPublisherIssue)
}

public actor WidgetSnapshotPublisher {
	public private(set) var highestAcceptedGeneration: UInt64 = 0
	public private(set) var lastVerifiedGeneration: UInt64?

	private let store: AppGroupSnapshotStore
	private let reloader: WidgetTimelineReloader
	private let beforeCommit: @Sendable () async throws -> Void
	private var baseline: AppGroupSnapshotExisting?

	public init(
		store: AppGroupSnapshotStore,
		reloader: WidgetTimelineReloader = .live,
		beforeCommit: @escaping @Sendable () async throws -> Void = {}
	) {
		self.store = store
		self.reloader = reloader
		self.beforeCommit = beforeCommit
	}

	public func publish(
		_ snapshot: AppGroupDesktopSnapshotV1,
		generation: UInt64
	) async -> WidgetSnapshotPublicationResult {
		guard generation > highestAcceptedGeneration else { return .superseded }
		highestAcceptedGeneration = generation

		let existing = baseline ?? store.readExisting()
		baseline = existing
		let affectedKinds: Set<AppGroupWidgetKind>
		do {
			let next = try WidgetSemanticProjection(snapshot: snapshot)
			switch existing {
			case let .known(previous):
				affectedKinds = next.affectedKinds(comparedTo: try WidgetSemanticProjection(snapshot: previous))
			case .unknown:
				affectedKinds = Set(AppGroupWidgetKind.allCases)
			}
		} catch {
			return .failedBeforeCommit(
				retainedVerifiedGeneration: lastVerifiedGeneration,
				issue: .storageUnavailable
			)
		}

		let prepared: AppGroupSnapshotPreparedWrite
		do {
			prepared = try store.prepareWrite(snapshot)
		} catch {
			return .failedBeforeCommit(
				retainedVerifiedGeneration: lastVerifiedGeneration,
				issue: issue(for: error)
			)
		}

		do {
			try await beforeCommit()
		} catch {
			store.discard(prepared)
			guard generation == highestAcceptedGeneration else { return .superseded }
			return .failedBeforeCommit(
				retainedVerifiedGeneration: lastVerifiedGeneration,
				issue: .storageUnavailable
			)
		}
		guard generation == highestAcceptedGeneration else {
			store.discard(prepared)
			return .superseded
		}

		do {
			try store.commit(prepared)
		} catch AppGroupSnapshotCommitError.failedBeforeCommit {
			store.discard(prepared)
			return .failedBeforeCommit(
				retainedVerifiedGeneration: lastVerifiedGeneration,
				issue: .storageUnavailable
			)
		} catch AppGroupSnapshotCommitError.indeterminateAfterCommit {
			baseline = .unknown(.unreadable)
			return .indeterminateAfterCommit(attemptedGeneration: generation, issue: .storageUnavailable)
		} catch {
			store.discard(prepared)
			return .failedBeforeCommit(
				retainedVerifiedGeneration: lastVerifiedGeneration,
				issue: .storageUnavailable
			)
		}

		lastVerifiedGeneration = generation
		baseline = .known(snapshot)
		if !affectedKinds.isEmpty {
			reloader.reload(kinds: affectedKinds)
		}
		return .published(affectedKinds: affectedKinds)
	}

	private func issue(for error: Error) -> WidgetSnapshotPublisherIssue {
		switch error {
		case AppGroupSnapshotStoreError.oversizedSnapshot:
			.oversizedOutput
		case AppGroupSnapshotStoreError.insecureDirectory, AppGroupSnapshotStoreError.insecureFile:
			.unsafeOutput
		default:
			.storageUnavailable
		}
	}
}
