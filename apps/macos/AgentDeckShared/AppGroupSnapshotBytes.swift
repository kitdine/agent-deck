import Darwin
import Foundation

// Shared by the writer and the read-only extension. These types deliberately
// have no storage for installation stamps, organization, or identity diagnostics.
public struct AppGroupSubscriptionV1: Codable, Equatable, Sendable {
	let available, reading: Bool
	let clients: [AppGroupQuotaClient]

	static let unavailable = Self(DesktopSubscriptionSnapshotV1.unavailable)

	init(_ source: DesktopSubscriptionSnapshotV1) {
		available = source.available
		reading = source.reading
		clients = source.clients.map(AppGroupQuotaClient.init)
	}
	enum CodingKeys: CodingKey { case available, reading, clients }
	public init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		available = try c.decode(Bool.self, forKey: .available)
		reading = try c.decodeIfPresent(Bool.self, forKey: .reading) ?? true
		clients = try c.decode([AppGroupQuotaClient].self, forKey: .clients)
	}
}

struct AppGroupQuotaClient: Codable, Equatable, Sendable {
	let client: String
	let applicable: Bool
	let applicableReason: DesktopQuotaReasonV1?
	let source: DesktopQuotaSourceV1?
	let observedAt: String?
	let stale, attributionConfirmed: Bool
	let plan: String?
	let planReason: DesktopQuotaReasonV1?
	let windows: [DesktopSubscriptionWindowV1]
	let tightestWindowKey: String?
	let resetAllowance: DesktopResetAllowanceV1?
	let resetAllowanceReason: DesktopQuotaReasonV1?
	let observedResetAt: String?
	let failure: DesktopQuotaReasonV1?
	let planDisplay: WireField<Plan>
	let quotaContext: WireField<Context>
	let identity: WireField<Identity>

	struct Plan: Codable, Equatable, Sendable {
		let state: DesktopPlanDisplayV1.State
		let shortName: WireField<String>
		enum CodingKeys: String, CodingKey { case state, shortName = "short_name" }
	}
	struct Context: Codable, Equatable, Sendable {
		let reason: DesktopQuotaContextV1.Reason
	}
	struct Identity: Codable, Equatable, Sendable {
		let state: DesktopQuotaIdentityV1.State
		let freshness: DesktopQuotaIdentityV1.Freshness
		let currentUntil, emailMasked: WireField<String>
		enum CodingKeys: String, CodingKey {
			case state, freshness, currentUntil = "current_until", emailMasked = "email_masked"
		}

		func safeForStorage() -> Self {
			var email = emailMasked
			if let value = email.value {
				// Reject a full address mistakenly supplied in email_masked. Never
				// attempt to construct a mask or restore historical identity.
				let parts = value.split(separator: "@", omittingEmptySubsequences: false)
				let local = parts.first ?? ""
				let valid = value.rangeOfCharacter(from: .whitespacesAndNewlines.union(.controlCharacters)) == nil &&
					parts.count == 2 && parts[1].contains(".") && !parts[1].hasPrefix(".") && !parts[1].hasSuffix(".") &&
					(local == "***" || local.dropFirst() == "***" || local.dropFirst().dropLast() == "***")
				if state != .identified || freshness != .current || !valid { email = .null }
			}
			return Self(state: state, freshness: freshness, currentUntil: currentUntil, emailMasked: email)
		}
	}

	init(_ source: DesktopSubscriptionClientV1) {
		client = source.client
		applicable = source.applicable
		applicableReason = source.applicableReason
		self.source = source.source
		observedAt = source.observedAt
		stale = source.stale
		attributionConfirmed = source.attributionConfirmed
		plan = source.plan
		planReason = source.planReason
		windows = source.windows
		tightestWindowKey = source.tightestWindowKey
		resetAllowance = source.resetAllowance
		resetAllowanceReason = source.resetAllowanceReason
		observedResetAt = source.observedResetAt
		failure = source.failure
		planDisplay = source.planDisplay.project { Plan(state: $0.state, shortName: $0.shortName) }
		quotaContext = source.quotaContext.project { Context(reason: $0.reason) }
		identity = source.identity.project {
			Identity(state: $0.state, freshness: $0.freshness, currentUntil: $0.currentUntil,
				emailMasked: $0.emailMasked).safeForStorage()
		}
	}

	enum CodingKeys: String, CodingKey {
		case client, applicable, source, stale, plan, windows, failure, identity
		case applicableReason = "applicable_reason", observedAt = "observed_at", attributionConfirmed = "attribution_confirmed"
		case planReason = "plan_reason", tightestWindowKey = "tightest_window_key", resetAllowance = "reset_allowance"
		case resetAllowanceReason = "reset_allowance_reason", observedResetAt = "observed_reset_at"
		case planDisplay = "plan_display", quotaContext = "quota_context"
	}

	static func timestamp(_ raw: String) -> Date? {
		guard raw.range(of: #"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})\z"#,
			options: .regularExpression) != nil else { return nil }
		return DesktopTimestamp.date(from: raw)
	}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		let name = try c.decode(String.self, forKey: .client)
		guard name == "claude" || name == "codex" else {
			throw DecodingError.dataCorruptedError(forKey: .client, in: c, debugDescription: "Unknown quota client")
		}
		client = name
		applicable = try c.decode(Bool.self, forKey: .applicable)
		applicableReason = try c.decodeIfPresent(DesktopQuotaReasonV1.self, forKey: .applicableReason)
		source = try c.decodeIfPresent(DesktopQuotaSourceV1.self, forKey: .source)
		func observation(_ key: CodingKeys) throws -> String? {
			guard let raw = try c.decodeIfPresent(String.self, forKey: key) else { return nil }
			guard Self.timestamp(raw) != nil else {
				throw DecodingError.dataCorruptedError(forKey: key, in: c, debugDescription: "Invalid quota timestamp")
			}
			return raw
		}
		observedAt = try observation(.observedAt)
		stale = try c.decode(Bool.self, forKey: .stale)
		attributionConfirmed = try c.decode(Bool.self, forKey: .attributionConfirmed)
		plan = try c.decodeIfPresent(String.self, forKey: .plan)
		planReason = try c.decodeIfPresent(DesktopQuotaReasonV1.self, forKey: .planReason)
		windows = try c.decode([DesktopSubscriptionWindowV1].self, forKey: .windows)
		tightestWindowKey = try c.decodeIfPresent(String.self, forKey: .tightestWindowKey)
		if let key = tightestWindowKey, !windows.contains(where: { $0.key == key }) {
			throw DecodingError.dataCorruptedError(forKey: .tightestWindowKey, in: c, debugDescription: "Unknown window")
		}
		resetAllowance = try c.decodeIfPresent(DesktopResetAllowanceV1.self, forKey: .resetAllowance)
		resetAllowanceReason = try c.decodeIfPresent(DesktopQuotaReasonV1.self, forKey: .resetAllowanceReason)
		observedResetAt = try observation(.observedResetAt)
		failure = try c.decodeIfPresent(DesktopQuotaReasonV1.self, forKey: .failure)
		planDisplay = try c.decode(WireField<Plan>.self, forKey: .planDisplay)
		quotaContext = try c.decode(WireField<Context>.self, forKey: .quotaContext)
		identity = try c.decode(WireField<Identity>.self, forKey: .identity).project { $0.safeForStorage() }
	}
}

private extension WireField {
	func project<U>(_ transform: (T) -> U) -> WireField<U> {
		switch self {
		case .absent: .absent
		case .null: .null
		case let .value(v): .value(transform(v))
		}
	}
}

// Codable synthesis uses these overloads even for optional members inside
// the compact plan and identity objects: a missing key is not JSON null.
private extension KeyedDecodingContainer {
	func decode<T>(_ type: WireField<T>.Type, forKey key: Key) throws -> WireField<T> {
		guard contains(key) else { return .absent }
		return try decodeNil(forKey: key) ? .null : .value(decode(T.self, forKey: key))
	}
}
private extension KeyedEncodingContainer {
	mutating func encode<T>(_ field: WireField<T>, forKey key: Key) throws {
		switch field {
		case .absent: break
		case .null: try encodeNil(forKey: key)
		case let .value(value): try encode(value, forKey: key)
		}
	}
}

public enum AppGroupSnapshotByteReadError: Error, Equatable, Sendable {
	case missing
	case unsafeFile
	case oversized
	case unreadable
}

public enum AppGroupSnapshotBytes {
	public static let maximumBytes = 8 * 1024 * 1024
	private static let readChunkBytes = 64 * 1024

	public static func readBounded(at url: URL) throws -> Data {
		try readBounded(at: url, afterLstat: {})
	}

	static func readBounded(
		at url: URL,
		afterLstat: () throws -> Void
	) throws -> Data {
		var pathStatus = stat()
		guard lstat(url.path, &pathStatus) == 0 else {
			throw errno == ENOENT ? AppGroupSnapshotByteReadError.missing : .unreadable
		}
		guard isRegular(pathStatus), pathStatus.st_size <= off_t(maximumBytes) else {
			throw isRegular(pathStatus) ? AppGroupSnapshotByteReadError.oversized : .unsafeFile
		}
		try afterLstat()

		let descriptor = open(url.path, O_RDONLY | O_NOFOLLOW)
		guard descriptor >= 0 else {
			throw errno == ENOENT ? AppGroupSnapshotByteReadError.missing : .unreadable
		}
		defer { close(descriptor) }

		var openedStatus = stat()
		guard fstat(descriptor, &openedStatus) == 0 else {
			throw AppGroupSnapshotByteReadError.unreadable
		}
		guard isRegular(openedStatus), openedStatus.st_size <= off_t(maximumBytes) else {
			throw isRegular(openedStatus) ? AppGroupSnapshotByteReadError.oversized : .unsafeFile
		}
		guard pathStatus.st_dev == openedStatus.st_dev,
			pathStatus.st_ino == openedStatus.st_ino
		else {
			throw AppGroupSnapshotByteReadError.unsafeFile
		}

		var bytes = Data()
		bytes.reserveCapacity(min(Int(openedStatus.st_size), maximumBytes))
		var buffer = [UInt8](repeating: 0, count: readChunkBytes)
		while bytes.count <= maximumBytes {
			let remaining = maximumBytes + 1 - bytes.count
			let count = buffer.withUnsafeMutableBytes { rawBuffer in
				Darwin.read(descriptor, rawBuffer.baseAddress, min(rawBuffer.count, remaining))
			}
			guard count >= 0 else {
				throw AppGroupSnapshotByteReadError.unreadable
			}
			guard count > 0 else { break }
			bytes.append(buffer, count: count)
			if bytes.count > maximumBytes {
				throw AppGroupSnapshotByteReadError.oversized
			}
		}

		var finalStatus = stat()
		guard fstat(descriptor, &finalStatus) == 0 else {
			throw AppGroupSnapshotByteReadError.unreadable
		}
		guard isRegular(finalStatus) else {
			throw AppGroupSnapshotByteReadError.unsafeFile
		}
		guard finalStatus.st_size <= off_t(maximumBytes) else {
			throw AppGroupSnapshotByteReadError.oversized
		}
		guard finalStatus.st_size == openedStatus.st_size,
			finalStatus.st_size == off_t(bytes.count)
		else {
			throw AppGroupSnapshotByteReadError.unreadable
		}
		return bytes
	}

	private static func isRegular(_ status: stat) -> Bool {
		(status.st_mode & S_IFMT) == S_IFREG
	}
}
