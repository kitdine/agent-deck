package scanruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/platform"
)

const (
	receiptJournalFilename = "scan-receipts.json"
	receiptJournalVersion  = 1
	maxReceipts            = 256
	maxExpiredReceipts     = maxReceipts
	receiptRetention       = 15 * time.Minute
)

var ErrReceiptJournalFull = errors.New("scan receipt journal is full")

type receiptState string

const (
	receiptAccepted receiptState = "accepted"
	receiptTerminal receiptState = "terminal"
	receiptExpired  receiptState = "expired"
)

// scanReceipt is intentionally private and bounded. It records only local
// coordination identity and domain outcomes; it never contains source text,
// paths from scan progress, tool bodies, or credentials.
type scanReceipt struct {
	ID            string       `json:"id"`
	StateID       string       `json:"state_id"`
	Home          string       `json:"home"`
	Scope         Scope        `json:"scope"`
	RoundID       string       `json:"round_id"`
	Observation   uint64       `json:"observation"`
	State         receiptState `json:"state"`
	RoundTerminal bool         `json:"round_terminal"`
	AcceptedAt    time.Time    `json:"accepted_at"`
	TerminalAt    *time.Time   `json:"terminal_at,omitempty"`
	Result        *Result      `json:"result,omitempty"`
}

type receiptJournal struct {
	path    string
	entries map[string]scanReceipt
}

type receiptJournalDocument struct {
	Version int           `json:"version"`
	Entries []scanReceipt `json:"entries"`
}

func openReceiptJournal(stateRoot string) (*receiptJournal, error) {
	journal := &receiptJournal{
		path:    filepath.Join(stateRoot, receiptJournalFilename),
		entries: map[string]scanReceipt{},
	}
	info, err := os.Lstat(journal.path)
	if errors.Is(err, os.ErrNotExist) {
		return journal, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("scan receipt journal is not a regular private file")
	}
	if err = os.Chmod(journal.path, platform.FileMode); err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(journal.path)
	if err != nil {
		return nil, err
	}
	var document receiptJournalDocument
	if err = json.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("decode scan receipt journal: %w", err)
	}
	if document.Version != receiptJournalVersion {
		return nil, fmt.Errorf("unsupported scan receipt journal version %d", document.Version)
	}
	for _, receipt := range document.Entries {
		if receipt.ID == "" || receipt.StateID == "" || receipt.RoundID == "" || !receipt.Scope.valid() {
			return nil, errors.New("invalid scan receipt journal entry")
		}
		if _, exists := journal.entries[receipt.ID]; exists {
			return nil, errors.New("duplicate scan receipt journal entry")
		}
		journal.entries[receipt.ID] = receipt
	}
	return journal, nil
}

func (j *receiptJournal) get(id string) (scanReceipt, bool) {
	receipt, found := j.entries[id]
	return receipt, found
}

func (j *receiptJournal) accept(receipt scanReceipt) error {
	if receipt.ID == "" || receipt.StateID == "" || receipt.RoundID == "" || !receipt.Scope.valid() {
		return errors.New("invalid accepted scan receipt")
	}
	if _, exists := j.entries[receipt.ID]; exists {
		return errors.New("scan receipt already exists")
	}
	before := cloneReceiptEntries(j.entries)
	if err := j.makeRoom(); err != nil {
		return err
	}
	if receipt.AcceptedAt.IsZero() {
		receipt.AcceptedAt = time.Now().UTC()
	}
	receipt.State = receiptAccepted
	j.entries[receipt.ID] = receipt
	if err := j.save(); err != nil {
		j.entries = before
		return err
	}
	return nil
}

func (j *receiptJournal) terminal(roundID string, result Result) error {
	before := cloneReceiptEntries(j.entries)
	changed := false
	now := time.Now().UTC()
	for id, receipt := range j.entries {
		if receipt.RoundID != roundID || receipt.RoundTerminal {
			continue
		}
		if receipt.State == receiptAccepted {
			copy := result
			receipt.State = receiptTerminal
			receipt.TerminalAt = &now
			receipt.Result = &copy
		}
		receipt.RoundTerminal = true
		j.entries[id] = receipt
		changed = true
	}
	if !changed {
		return nil
	}
	if err := j.save(); err != nil {
		j.entries = before
		return err
	}
	return nil
}

func (j *receiptJournal) terminalReceipt(id string, result Result) error {
	receipt, found := j.entries[id]
	if !found {
		return errors.New("scan receipt not found")
	}
	if receipt.State == receiptTerminal {
		return nil
	}
	if receipt.State != receiptAccepted {
		return errors.New("scan receipt cannot become terminal")
	}
	before := cloneReceiptEntries(j.entries)
	now := time.Now().UTC()
	copy := result
	receipt.State = receiptTerminal
	receipt.TerminalAt = &now
	receipt.Result = &copy
	j.entries[id] = receipt
	if err := j.save(); err != nil {
		j.entries = before
		return err
	}
	return nil
}

func (j *receiptJournal) recoverable() []scanReceipt {
	ret := make([]scanReceipt, 0)
	for _, receipt := range j.entries {
		if !receipt.RoundTerminal {
			ret = append(ret, receipt)
		}
	}
	sort.Slice(ret, func(i, k int) bool {
		if !ret[i].AcceptedAt.Equal(ret[k].AcceptedAt) {
			return ret[i].AcceptedAt.Before(ret[k].AcceptedAt)
		}
		return ret[i].ID < ret[k].ID
	})
	return ret
}

func cloneReceiptEntries(entries map[string]scanReceipt) map[string]scanReceipt {
	copy := make(map[string]scanReceipt, len(entries))
	for id, receipt := range entries {
		copy[id] = receipt
	}
	return copy
}

func (j *receiptJournal) activeCount() int {
	count := 0
	for _, receipt := range j.entries {
		if receipt.State != receiptExpired {
			count++
		}
	}
	return count
}

func (j *receiptJournal) expire(id string) {
	receipt := j.entries[id]
	receipt.State = receiptExpired
	receipt.TerminalAt = nil
	receipt.Result = nil
	j.entries[id] = receipt
}

func (j *receiptJournal) trimExpired() {
	expired := make([]scanReceipt, 0)
	for _, receipt := range j.entries {
		if receipt.State == receiptExpired {
			expired = append(expired, receipt)
		}
	}
	sort.Slice(expired, func(i, k int) bool {
		if !expired[i].AcceptedAt.Equal(expired[k].AcceptedAt) {
			return expired[i].AcceptedAt.Before(expired[k].AcceptedAt)
		}
		return expired[i].ID < expired[k].ID
	})
	for len(expired) > maxExpiredReceipts {
		delete(j.entries, expired[0].ID)
		expired = expired[1:]
	}
}

func (j *receiptJournal) makeRoom() error {
	if j.activeCount() < maxReceipts {
		return nil
	}
	terminal := make([]scanReceipt, 0)
	for _, receipt := range j.entries {
		if receipt.State == receiptTerminal && receipt.RoundTerminal {
			terminal = append(terminal, receipt)
		}
	}
	sort.Slice(terminal, func(i, k int) bool {
		if !terminal[i].AcceptedAt.Equal(terminal[k].AcceptedAt) {
			return terminal[i].AcceptedAt.Before(terminal[k].AcceptedAt)
		}
		return terminal[i].ID < terminal[k].ID
	})
	if len(terminal) == 0 {
		return ErrReceiptJournalFull
	}
	j.expire(terminal[0].ID)
	j.trimExpired()
	return nil
}

func (j *receiptJournal) prune(now time.Time) error {
	before := cloneReceiptEntries(j.entries)
	changed := false
	for id, receipt := range j.entries {
		if receipt.State != receiptTerminal || !receipt.RoundTerminal || receipt.TerminalAt == nil || now.Sub(*receipt.TerminalAt) <= receiptRetention {
			continue
		}
		j.expire(id)
		changed = true
	}
	if !changed {
		return nil
	}
	j.trimExpired()
	if err := j.save(); err != nil {
		j.entries = before
		return err
	}
	return nil
}

func (j *receiptJournal) save() error {
	entries := make([]scanReceipt, 0, len(j.entries))
	for _, receipt := range j.entries {
		entries = append(entries, receipt)
	}
	sort.Slice(entries, func(i, k int) bool {
		if !entries[i].AcceptedAt.Equal(entries[k].AcceptedAt) {
			return entries[i].AcceptedAt.Before(entries[k].AcceptedAt)
		}
		return entries[i].ID < entries[k].ID
	})
	contents, err := json.Marshal(receiptJournalDocument{Version: receiptJournalVersion, Entries: entries})
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(j.path), ".scan-receipts-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(platform.FileMode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = temporary.Write(append(contents, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, j.path); err != nil {
		return err
	}
	return os.Chmod(j.path, platform.FileMode)
}
