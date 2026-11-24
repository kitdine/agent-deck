package session

import (
	"context"
	"os"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// This is the session domain target for verified native plaintext. The replay
// has its own version and other source types keep their existing parser.
const nativeSessionParserVersion = 7

// Independent capture, owned by exactly one provisional session reduction.
// Text never enters the workset sink or the proof.
type sessionMaterial struct {
	prefix  ingest.CodexPrefixProof
	visible sessionMessageCapture
}

type sessionMessageCapture interface {
	lookup(ingest.CodexSpan) (ingest.CodexVisible, bool)
	discard()
}

type capturedMessages map[ingest.CodexSpan]ingest.CodexVisible

func (v capturedMessages) lookup(span ingest.CodexSpan) (ingest.CodexVisible, bool) {
	m, ok := v[span]
	return m, ok
}

func (v capturedMessages) discard() { clear(v) }

func captureSessionMaterial(ctx context.Context, file *os.File, path string, end int64, maxLineBytes int) (sessionMaterial, error) {
	visible := make(capturedMessages)
	prefix, err := ingest.CaptureCodexPrefix(ctx, file, path, end, maxLineBytes, true, func(_ ingest.CodexRecord, v *ingest.CodexVisible) error {
		if v != nil {
			visible[v.Span] = *v
		}
		return nil
	})
	if err != nil {
		return sessionMaterial{}, err
	}
	return sessionMaterial{prefix, visible}, nil
}

type nativeSessionDocument struct {
	Document
	Span    ingest.CodexSpan
	Ordinal int
}

type nativeSessionUpdate struct {
	Source        ingest.CodexSourceProof
	Evidence      ingest.CodexPrefixEvidence
	Checkpoint    ingest.CodexCheckpoint
	ParserVersion int
	Model         string
	LastAt        time.Time
	Documents     []nativeSessionDocument
}

// The reducer emits records followed by their segment. It never receives the
// independent body capture. Keep only the facts needed for the later join.
type sessionMessageRef struct {
	span              ingest.CodexSpan
	kind              ingest.CodexVisibleKind
	turn              string
	independentThread bool
}

type sessionSegmentRefs struct {
	proof ingest.CodexSegmentProof
	refs  []sessionMessageRef
}

type sessionMaterialSink struct {
	thread   string
	model    string
	last     time.Time
	pending  []sessionMessageRef
	segments []sessionSegmentRefs
}

func (s *sessionMaterialSink) StageCodexProof(_ context.Context, r ingest.CodexResolvedRecord) error {
	if !r.Local() || r.ThreadID != s.thread {
		return nil
	}
	if s.model == "" {
		s.model = r.Model
	}
	if t := r.Record.At; t.After(s.last) {
		s.last = t
	}
	independent := r.Execution == ingest.CodexExplicit || r.Execution == ingest.CodexSingleSession
	if (r.TurnID != "" || independent) && r.Record.Kind == ingest.CodexMessage && r.Record.VisibleKind != ingest.CodexNoDocument {
		s.pending = append(s.pending, sessionMessageRef{r.Record.Span, r.Record.VisibleKind, r.TurnID, independent})
	}
	return nil
}

func (s *sessionMaterialSink) StageCodexSegment(_ context.Context, seg ingest.CodexSegmentProof) error {
	if len(s.pending) > 0 {
		s.segments = append(s.segments, sessionSegmentRefs{seg, s.pending})
		s.pending = nil
	}
	return nil
}

func (s *sessionMaterialSink) Abort() {
	s.pending, s.segments = nil, nil
}

func (s *sessionMaterialSink) documents(ctx context.Context, capture sessionMessageCapture) ([]nativeSessionDocument, error) {
	var docs []nativeSessionDocument
	for _, segment := range s.segments {
		seg := segment.proof
		if seg.ThreadID != s.thread {
			continue
		}
		for _, ref := range segment.refs {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if (!ref.independentThread && ref.turn != seg.TurnID) || seg.Span.Start > ref.span.Start || seg.Span.End < ref.span.End {
				continue
			}
			m, ok := capture.lookup(ref.span)
			if !ok || m.Span != ref.span || m.Kind != ref.kind {
				return nil, ingest.ErrSourceChanged
			}
			var kind string
			switch m.Kind {
			case ingest.CodexUserPrompt:
				kind = "user_prompt"
			case ingest.CodexAssistantFinal:
				kind = "assistant_final"
			default:
				continue
			}
			doc := Document{Client: "codex", SessionID: s.thread, Kind: kind, Text: m.Text, EventAt: m.EventAt}
			docs = append(docs, nativeSessionDocument{Document: doc, Span: m.Span, Ordinal: len(docs) + 1})
		}
	}
	return docs, nil
}

// acceptSessionMaterial consumes the capture, even on failure. Retiring the
// sealed attempt and publishing any accepted update belong to the caller.
func acceptSessionMaterial(ctx context.Context, replay *ingest.CodexCheckpointReplay, limits ingest.CodexWorksetLimits,
	material sessionMaterial, selected ingest.CodexSelectedSource, current ingest.CodexCurrentSelection) (nativeSessionUpdate, error) {
	if material.visible == nil {
		return nativeSessionUpdate{}, ingest.ErrSourceChanged
	}
	defer material.visible.discard()
	if current == nil || selected.Epoch <= 0 || replay == nil {
		return nativeSessionUpdate{}, ingest.ErrSourceChanged
	}
	pending := sessionMaterialSink{thread: selected.ThreadID}
	reduced, err := replay.ReduceWorkset(ctx, limits, &pending)
	if err != nil {
		return nativeSessionUpdate{}, err
	}
	evidence := reduced.Capture().Evidence()
	bytes := evidence
	bytes.Version = ingest.CodexCheckpointVersion{}
	if material.prefix.Evidence() != bytes {
		return nativeSessionUpdate{}, ingest.ErrSourceChanged
	}
	if err := reduced.VerifySelectedSource(ctx, selected, current); err != nil {
		return nativeSessionUpdate{}, err
	}
	docs, err := pending.documents(ctx, material.visible)
	if err != nil {
		return nativeSessionUpdate{}, err
	}
	return nativeSessionUpdate{Source: reduced.Source(), Evidence: evidence, Checkpoint: reduced.Checkpoint(), ParserVersion: nativeSessionParserVersion, Model: pending.model, LastAt: pending.last, Documents: docs}, nil
}
