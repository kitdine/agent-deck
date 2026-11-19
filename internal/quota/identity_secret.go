package quota

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

const (
	claudeIdentityKeyName   = "quota-identity.key"
	claudeIdentityKeyPrefix = "." + claudeIdentityKeyName + "-"
	claudeIdentityKeyMagic  = "AGENTDECK-QUOTA-IDENTITY-KEY\n"
	claudeIdentityKeyV1     = 1
	claudeIdentityKeySize   = 32
)

var errInvalidClaudeIdentityKey = errors.New("invalid quota identity key")

func deriveClaudeComparison(key [claudeIdentityKeySize]byte, kind, email, org string) *claudeComparison {
	sum := func(input string) []byte {
		mac := hmac.New(sha256.New, key[:])
		_, _ = io.WriteString(mac, input)
		return mac.Sum(nil)
	}
	var comparison claudeComparison
	copy(comparison.correlation[:], sum("agentdeck-quota-identity/v1\x00claude\x00"+kind+"\x00"+strings.ToLower(email)+"\x00"+org))
	copy(comparison.keyID[:], sum("key-id/v1"))
	return &comparison
}

func loadClaudeIdentityKey(stateRoot string) ([claudeIdentityKeySize]byte, error) {
	var zero [claudeIdentityKeySize]byte
	root, err := platform.GuardStateRoot(stateRoot)
	if err != nil {
		return zero, err
	}
	dir, err := openClaudeIdentityDir(root)
	if errors.Is(err, unix.ENOENT) {
		if err = platform.EnsureStateRoot(root); err != nil {
			return zero, err
		}
		dir, err = openClaudeIdentityDir(root)
	}
	if err != nil {
		return zero, err
	}
	defer dir.Close()
	fd := int(dir.Fd())
	key, err := readClaudeIdentityKey(fd)
	if !errors.Is(err, unix.ENOENT) {
		return key, err
	}
	cleanAbandonedClaudeKeys(dir, time.Now())
	if err := createClaudeIdentityKey(fd); err != nil {
		return zero, err
	}
	return readClaudeIdentityKey(fd)
}

func openClaudeIdentityDir(root string) (*os.File, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || stat.Uid != uint32(os.Geteuid()) || stat.Mode&07777 != 0700 {
		_ = unix.Close(fd)
		return nil, errInvalidClaudeIdentityKey
	}
	return os.NewFile(uintptr(fd), root), nil
}

func readClaudeIdentityKey(dirFD int) (key [claudeIdentityKeySize]byte, err error) {
	fd, err := unix.Openat(dirFD, claudeIdentityKeyName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return key, err
	}
	file := os.NewFile(uintptr(fd), claudeIdentityKeyName)
	defer file.Close()
	return readClaudeIdentityKeyFile(file)
}

func readClaudeIdentityKeyFile(file *os.File) (key [claudeIdentityKeySize]byte, err error) {
	const fileSize = len(claudeIdentityKeyMagic) + 1 + claudeIdentityKeySize
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil ||
		stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&07777 != 0600 ||
		stat.Uid != uint32(os.Geteuid()) || stat.Size != int64(fileSize) {
		return key, errInvalidClaudeIdentityKey
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(fileSize)+1))
	if err != nil || len(data) != fileSize ||
		string(data[:len(claudeIdentityKeyMagic)]) != claudeIdentityKeyMagic || data[len(claudeIdentityKeyMagic)] != claudeIdentityKeyV1 {
		return key, errInvalidClaudeIdentityKey
	}
	copy(key[:], data[len(claudeIdentityKeyMagic)+1:])
	return key, nil
}

func createClaudeIdentityKey(dirFD int) error {
	var random [claudeIdentityKeySize + 16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	name := claudeIdentityKeyPrefix + hex.EncodeToString(random[claudeIdentityKeySize:])
	fd, err := unix.Openat(dirFD, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(dirFD, name, 0)
	file := os.NewFile(uintptr(fd), name)
	data := append([]byte(claudeIdentityKeyMagic), claudeIdentityKeyV1)
	data = append(data, random[:claudeIdentityKeySize]...)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// Linking never overwrites a winner, including an invalid existing key.
	if err := unix.Linkat(dirFD, name, dirFD, claudeIdentityKeyName, 0); err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	return unix.Fsync(dirFD)
}

func cleanAbandonedClaudeKeys(dir *os.File, now time.Time) {
	fd := int(dir.Fd())
	names, _ := dir.Readdirnames(-1)
	for _, name := range names {
		if !strings.HasPrefix(name, claudeIdentityKeyPrefix) {
			continue
		}
		var stat unix.Stat_t
		if unix.Fstatat(fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW) == nil &&
			stat.Mode&unix.S_IFMT == unix.S_IFREG && stat.Uid == uint32(os.Geteuid()) &&
			now.Sub(time.Unix(stat.Mtim.Unix())) > 10*time.Minute {
			_ = unix.Unlinkat(fd, name, 0)
		}
	}
}
