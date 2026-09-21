import Darwin
import Foundation

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
