// Package watch coordinates foreground incremental scans.
package watch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const EventSchemaVersion = 1

type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Type          string    `json:"type"`
	Domain        string    `json:"domain"`
	GeneratedAt   time.Time `json:"generated_at"`
	Changes       int       `json:"changes"`
	Skipped       bool      `json:"skipped"`
	Reason        string    `json:"reason,omitempty"`
}

type Source struct {
	Domain   string
	Snapshot func(context.Context) (string, error)
	Scan     func(context.Context) (int, error)
}

type LockFunc func(context.Context) (release func() error, err error)

type Service struct {
	Sources             SourceSet
	Lock                LockFunc
	Now                 func() time.Time
	InitialFingerprints map[string]string
	PersistFingerprint  func(context.Context, string, string) error

	fingerprints map[string]string
}

type SourceSet []Source

func (s *Service) Poll(ctx context.Context) ([]Event, error) {
	if s.fingerprints == nil {
		s.fingerprints = make(map[string]string)
		for domain, fingerprint := range s.InitialFingerprints {
			s.fingerprints[domain] = fingerprint
		}
	}
	type changedSource struct {
		source      Source
		fingerprint string
	}
	changed := make([]changedSource, 0, len(s.Sources))
	for _, source := range s.Sources {
		fingerprint, err := source.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", source.Domain, err)
		}
		if previous, ok := s.fingerprints[source.Domain]; !ok || previous != fingerprint {
			changed = append(changed, changedSource{source: source, fingerprint: fingerprint})
		}
	}
	if len(changed) == 0 {
		return []Event{}, nil
	}
	release, err := s.Lock(ctx)
	if errors.Is(err, store.ErrStateBusy) {
		return []Event{s.event("scan_skipped", "all", 0, true, store.ErrStateBusy.Code)}, nil
	}
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(changed))
	for _, item := range changed {
		changes, err := item.source.Scan(ctx)
		if err != nil {
			if releaseErr := release(); releaseErr != nil {
				return nil, errors.Join(fmt.Errorf("scan %s: %w", item.source.Domain, err), fmt.Errorf("release scan lock: %w", releaseErr))
			}
			return nil, fmt.Errorf("scan %s: %w", item.source.Domain, err)
		}
		if s.PersistFingerprint != nil {
			if err = s.PersistFingerprint(ctx, item.source.Domain, item.fingerprint); err != nil {
				if releaseErr := release(); releaseErr != nil {
					return nil, errors.Join(fmt.Errorf("persist scan fingerprint: %w", err), fmt.Errorf("release scan lock: %w", releaseErr))
				}
				return nil, fmt.Errorf("persist scan fingerprint: %w", err)
			}
		}
		s.fingerprints[item.source.Domain] = item.fingerprint
		events = append(events, s.event("scan_completed", item.source.Domain, changes, false, ""))
	}
	if err = release(); err != nil {
		return nil, fmt.Errorf("release scan lock: %w", err)
	}
	return events, nil
}

func (s *Service) Run(ctx context.Context, interval time.Duration, emit func(Event) error) error {
	if interval <= 0 {
		interval = time.Minute
	}
	for {
		events, err := s.Poll(ctx)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err = emit(event); err != nil {
				return err
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (s *Service) event(kind, domain string, changes int, skipped bool, reason string) Event {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return Event{SchemaVersion: EventSchemaVersion, Type: kind, Domain: domain, GeneratedAt: now().UTC(), Changes: changes, Skipped: skipped, Reason: reason}
}

// FingerprintRoots hashes source metadata without reading source contents.
// Missing roots are represented consistently and are not errors.
func FingerprintRoots(roots ...string) (string, error) {
	records := make([]string, 0)
	for _, root := range roots {
		root = filepath.Clean(root)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			records = append(records, strings.Join([]string{root, relative, fmt.Sprint(info.Mode()), fmt.Sprint(info.Size()), fmt.Sprint(info.ModTime().UnixNano())}, "\x00"))
			// WalkDir intentionally does not follow links.  For extension roots we
			// still need a linked skill's target changes to trigger a scan, while
			// avoiding recursive link traversal and cycles.
			if entry.Type()&fs.ModeSymlink != 0 {
				target, linkErr := filepath.EvalSymlinks(path)
				if linkErr != nil {
					records = append(records, root+"\x00"+relative+"\x00broken-link")
					return nil
				}
				linkErr = filepath.WalkDir(target, func(targetPath string, targetEntry fs.DirEntry, targetWalkErr error) error {
					if targetWalkErr != nil {
						return targetWalkErr
					}
					targetInfo, infoErr := targetEntry.Info()
					if infoErr != nil {
						return infoErr
					}
					targetRelative, relErr := filepath.Rel(target, targetPath)
					if relErr != nil {
						return relErr
					}
					records = append(records, strings.Join([]string{root, relative, "target", targetRelative, fmt.Sprint(targetInfo.Mode()), fmt.Sprint(targetInfo.Size()), fmt.Sprint(targetInfo.ModTime().UnixNano())}, "\x00"))
					return nil
				})
				if linkErr != nil {
					return linkErr
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if errors.Is(err, os.ErrNotExist) {
			records = append(records, root+"\x00missing")
		}
	}
	sort.Strings(records)
	digest := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(digest[:]), nil
}
