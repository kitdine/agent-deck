// Package hookrefusal owns the bounded diagnostic for refused Hook deliveries.
package hookrefusal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

const Filename = "hook-refusals.json"
const maxRecordBytes = 2048

type Record struct {
	SchemaVersion int       `json:"schema_version"`
	Code          string    `json:"code"`
	Stored        int       `json:"stored"`
	Supported     int       `json:"supported"`
	FirstAt       time.Time `json:"first_at"`
	LastAt        time.Time `json:"last_at"`
	Count         int       `json:"count"`
}

// Read ignores absent, incompatible and malformed diagnostics without repairing
// them. A size limit also bounds reads of files not produced by this package.
func Read(root string) (Record, bool) {
	path := filepath.Join(root, Filename)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxRecordBytes {
		return Record{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return Record{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxRecordBytes+1))
	if err != nil || len(data) > maxRecordBytes {
		return Record{}, false
	}
	var record Record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF {
		return Record{}, false
	}
	if record.SchemaVersion != 1 || record.Code != "schema_ahead" || record.Supported < 0 || record.Stored <= record.Supported || record.Count < 1 || record.FirstAt.IsZero() || record.LastAt.Before(record.FirstAt) {
		return Record{}, false
	}
	return record, true
}

// Live suppresses history after an upgrade without deleting or modifying it.
func Live(root string, supported int) (Record, bool) {
	r, ok := Read(root)
	return r, ok && r.Stored > supported
}

// Write atomically replaces one private record without acquiring the state lock.
// Concurrent read-modify-replace operations may lose increments; this is a
// diagnostic count, not an exact ledger. Callers deliberately ignore failures.
func Write(root string, stored, supported int) error {
	if supported < 0 || stored <= supported {
		return errors.New("invalid schema refusal")
	}
	now := time.Now().UTC()
	r, ok := Read(root)
	if !ok {
		r = Record{SchemaVersion: 1, Code: "schema_ahead", FirstAt: now}
	}
	r.Stored, r.Supported = stored, supported
	if now.Before(r.LastAt) {
		now = r.LastAt
	}
	r.LastAt = now
	if r.Count < math.MaxInt {
		r.Count++
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".hook-refusals-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(append(data, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(root, Filename))
}

func Clear(root string) error {
	err := os.Remove(filepath.Join(root, Filename))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
