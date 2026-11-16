package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/store"
)

// Bounds of the historical source identity preparation: sources per
// transaction and bytes read from the head of one source.
const (
	sourceIdentityBatch     = 64
	sourceIdentityHeadBytes = 1 << 20
)

// PrepareSourceIdentities assesses the thread identity of every source stored
// before identities were recorded, so that settlement can tell a child
// thread's JSONL from an OTel-only conversation. It captures the unassessed
// sources once and walks that finite set in path order, one transaction per
// batch; each batch stores its identities and retracts the OTel ownership they
// contradict. A source that cannot be read stays unassessed and the walk moves
// past it, so it never holds back the sources after it and is tried again on
// the next call. Committed batches survive cancellation and restart.
//
// Whether settlement may promote afterwards is the store's decision
// (TelemetrySourceIdentityReady), which the settle transaction applies itself.
func (s *Service) PrepareSourceIdentities(ctx context.Context) (changed bool, err error) {
	through, err := s.Store.LastUnassessedSourcePath(ctx)
	if err != nil {
		return changed, err
	}
	after := ""
	for through != "" {
		if err := ctx.Err(); err != nil {
			return changed, err
		}
		batch, err := s.Store.UnassessedSources(ctx, after, through, sourceIdentityBatch)
		if err != nil {
			return changed, err
		}
		if len(batch) == 0 {
			break
		}
		assessed := make([]store.SourceThreadIdentity, 0, len(batch))
		for _, source := range batch {
			after = source.Path
			if thread, ok := s.assessSourceIdentity(source); ok {
				assessed = append(assessed, store.SourceThreadIdentity{Path: source.Path, Identity: source.Identity, ThreadID: thread})
			}
		}
		if len(assessed) == 0 {
			continue
		}
		n, err := s.Store.SaveSourceThreadIdentities(ctx, assessed, s.now())
		changed = changed || n > 0
		if err != nil {
			return changed, err
		}
	}
	ready, err := s.Store.TelemetrySourceIdentityReady(ctx)
	if err == nil && !ready {
		err = store.ErrTelemetrySourceIdentityPending
	}
	return changed, err
}

// sourceClient names the client of a stored source from what is known about
// it: Codex facts already imported from it, or the discovery root it lies
// under. Any other source is unknown and is assessed by its content.
func (s *Service) sourceClient(source store.UnassessedSource) string {
	if source.CodexFacts {
		return "codex"
	}
	for _, root := range []struct{ client, path string }{
		{"codex", filepath.Join(s.Home, ".codex", "sessions")},
		{"codex", filepath.Join(s.Home, ".codex", "archived_sessions")},
		{"claude", filepath.Join(s.Home, ".claude", "projects")},
	} {
		if strings.HasPrefix(filepath.Clean(source.Path), root.path+string(filepath.Separator)) {
			return root.client
		}
	}
	return ""
}

// assessSourceIdentity returns the thread identity to store for a source, or
// false to leave it unassessed. A Claude source carries none. A Codex source's
// identity is the payload.id of its first session_meta, taken only from
// complete records of the still-unchanged file. A read failure, a changed
// source or a head without a decisive record leaves the source unassessed; it
// is never recorded as carrying no identity. A missing source keeps the
// identity its canonical rollout name proves.
func (s *Service) assessSourceIdentity(source store.UnassessedSource) (string, bool) {
	if s.sourceClient(source) == "claude" {
		return "", true
	}
	info, err := s.stat(source.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return store.CodexRolloutThreadID(source.Path)
	}
	if err != nil || info.IsDir() {
		return "", false
	}
	if identity, _, _ := ingest.FileGeneration(info); identity != source.Identity {
		return "", false
	}
	file, err := s.open(source.Path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	head, err := io.ReadAll(io.NewSectionReader(file, 0, min(info.Size(), int64(sourceIdentityHeadBytes))))
	if err != nil {
		return "", false
	}
	thread, decided := firstSessionThread(head, info.Size() <= sourceIdentityHeadBytes)
	if !decided {
		return "", false
	}
	identity, changedAt, stable := ingest.FileGeneration(info)
	entry := InventoryEntry{Path: source.Path, Identity: identity, Size: info.Size(), ModifiedAt: info.ModTime().UnixNano(), ChangedAt: changedAt, Stable: stable}
	if s.validateSnapshot(source.Path, file, entry, 0, head, 0, nil) != nil {
		return "", false
	}
	return thread, true
}

// firstSessionThread reads complete JSONL records of a source head. It
// returns the first session_meta's payload.id, empty when that ID is not a
// thread ID. Without a session_meta it decides "no identity" only when whole
// is set and the head held at least one complete record.
func firstSessionThread(head []byte, whole bool) (thread string, decided bool) {
	records := 0
	for len(head) > 0 {
		end := bytes.IndexByte(head, '\n')
		if end < 0 {
			break
		}
		line := head[:end]
		head = head[end+1:]
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				ID string `json:"id"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &record) != nil {
			continue
		}
		records++
		if record.Type == "session_meta" {
			id, _ := store.NormalizeThreadID(record.Payload.ID)
			return id, true
		}
	}
	return "", whole && records > 0 && len(bytes.TrimSpace(head)) == 0
}
