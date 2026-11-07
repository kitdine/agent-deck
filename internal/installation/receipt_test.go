package installation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
)

func TestReceiptValidation(t *testing.T) {
	r, _, self := fixture(t)
	receipt := register(t, r, self, "make")
	valid, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := r.AccountHome()
	path := receiptPath(home)
	cases := map[string]string{
		"version":      strings.Replace(string(valid), `"receipt_version":1`, `"receipt_version":2`, 1),
		"relative":     strings.Replace(string(valid), self, "relative/agentdeck", 1),
		"unclean":      strings.Replace(string(valid), self, filepath.Dir(self)+"/../bin/agentdeck", 1),
		"channel":      strings.Replace(string(valid), `"make"`, `"env"`, 1),
		"id-short":     strings.Replace(string(valid), receipt.InstallationID, "a", 1),
		"id-uppercase": strings.Replace(string(valid), receipt.InstallationID, strings.Repeat("A", 32), 1),
		"id-nonhex":    strings.Replace(string(valid), receipt.InstallationID, strings.Repeat("z", 32), 1),
		"unknown":      strings.TrimSuffix(string(valid), "}") + `,"state":"active"}`,
		"duplicate":    strings.TrimSuffix(string(valid), "}") + `,"channel":"make"}`,
		"case":         strings.Replace(string(valid), `"channel"`, `"Channel"`, 1),
		"trailing":     string(valid) + ` {}`,
		"oversize":     string(valid) + strings.Repeat(" ", maxReceiptBytes),
		"null":         "null", "array": "[]", "empty": "{}",
		"missing-time": strings.Replace(string(valid), `"installed_at":"`+receipt.InstalledAt.Format("2006-01-02T15:04:05Z07:00")+`",`, "", 1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if got := r.Resolve(); got.Reason != "receipt_invalid" {
				t.Fatalf("identity=%+v data=%s", got, data)
			}
		})
	}
	// Exactly 4 KiB is allowed; one byte more must fail, including whitespace.
	data := append(valid, []byte(strings.Repeat(" ", maxReceiptBytes-len(valid)))...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(); !got.Installed {
		t.Fatalf("4 KiB receipt: %+v", got)
	}
}

func TestReceiptNoFollowAndPhysicalRegistration(t *testing.T) {
	r, root, self := fixture(t)
	register(t, r, self, "make")
	home, _ := r.AccountHome()
	path := receiptPath(home)
	backing := filepath.Join(root, "receipt.json")
	if err := os.Rename(path, backing); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(backing, path); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(); got.Reason != "receipt_invalid" {
		t.Fatalf("symlink receipt: %+v", got)
	}
	lock, err := platform.AcquireOwnershipLock(context.Background(), home, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	link := filepath.Join(root, "executable-link")
	if err := os.Symlink(self, link); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{link, filepath.Dir(self), "relative"} {
		if _, err := r.WriteReceipt(lock, candidate, "make"); err == nil {
			t.Fatalf("registered nonphysical file %s", candidate)
		}
	}
	if _, err := r.WriteReceipt(nil, self, "make"); err == nil {
		t.Fatal("write without lock succeeded")
	}
	// Atomic publication replaces the receipt link itself; its target is intact.
	before, err := os.ReadFile(backing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.WriteReceipt(lock, self, "local_app"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(backing)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("registration followed receipt symlink")
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.WriteReceipt(lock, self, "make"); err == nil {
		t.Fatal("write with released lock succeeded")
	}
}
