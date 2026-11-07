package installation

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
	"golang.org/x/sys/unix"
)

const maxReceiptBytes = 4 * 1024

var installationIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Receipt struct {
	ReceiptVersion int       `json:"receipt_version"`
	Executable     string    `json:"executable"`
	Channel        string    `json:"channel"`
	InstalledAt    time.Time `json:"installed_at"`
	InstallationID string    `json:"installation_id"`
}

func receiptPath(account string) string {
	return filepath.Join(account, "Library", "Application Support", "AgentDeck", "installation.json")
}

func (r Receipt) validate() error {
	if r.ReceiptVersion != 1 || !filepath.IsAbs(r.Executable) || filepath.Clean(r.Executable) != r.Executable || !installationIDPattern.MatchString(r.InstallationID) || r.InstalledAt.IsZero() {
		return errors.New("invalid installation receipt")
	}
	switch r.Channel {
	case "homebrew_cask", "homebrew_formula", "make", "local_app":
		return nil
	}
	return errors.New("invalid installation channel")
}

func (r *Resolver) ReadReceipt() (Receipt, error) {
	home, err := r.AccountHome()
	if err != nil {
		return Receipt{}, err
	}
	return readReceipt(home)
}

func readReceipt(account string) (Receipt, error) {
	fd, err := unix.Open(receiptPath(account), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return Receipt{}, err
	}
	file := os.NewFile(uintptr(fd), "installation.json")
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Receipt{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxReceiptBytes {
		return Receipt{}, errors.New("invalid receipt file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxReceiptBytes+1))
	if err != nil {
		return Receipt{}, err
	}
	if len(data) > maxReceiptBytes {
		return Receipt{}, errors.New("installation receipt exceeds 4 KiB")
	}
	// Check exact field names and duplicates, rather than accepting encoding/json's
	// case-insensitive struct matching or last-value-wins duplicate handling.
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return Receipt{}, errors.New("invalid receipt object")
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Receipt{}, err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return Receipt{}, errors.New("duplicate receipt field")
		}
		switch key {
		case "receipt_version", "executable", "channel", "installed_at", "installation_id":
		default:
			return Receipt{}, fmt.Errorf("unknown receipt field %q", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return Receipt{}, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return Receipt{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Receipt{}, errors.New("trailing receipt data")
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, receipt.validate()
}

// WriteReceipt must run inside the same exclusive lifetime as publication.
// It registers the physical executable, not a mutable symlink such as opt.
func (r *Resolver) WriteReceipt(lock *platform.OwnershipLock, executable, channel string) (Receipt, error) {
	home, err := r.AccountHome()
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	err = lock.WithExclusive(home, func() error {
		physical, err := filepath.EvalSymlinks(executable)
		if err != nil {
			return err
		}
		if physical != executable {
			return errors.New("receipt executable must be a physical absolute path")
		}
		info, err := os.Lstat(executable)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("receipt executable must be a regular file")
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return err
		}
		now := r.now
		if now == nil {
			now = time.Now
		}
		receipt = Receipt{1, executable, channel, now().UTC().Truncate(time.Second), hex.EncodeToString(id[:])}
		if err := receipt.validate(); err != nil {
			return err
		}
		data, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if len(data) > maxReceiptBytes {
			return errors.New("installation receipt exceeds 4 KiB")
		}
		path := receiptPath(home)
		file, err := os.CreateTemp(filepath.Dir(path), ".installation-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())
		defer file.Close()
		if err := file.Chmod(platform.FileMode); err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(file.Name(), path); err != nil {
			return err
		}
		dir, err := os.Open(filepath.Dir(path))
		if err != nil {
			return err
		}
		defer dir.Close()
		return dir.Sync()
	})
	return receipt, err
}
