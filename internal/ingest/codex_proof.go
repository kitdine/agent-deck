package ingest

import (
	"iter"
	"strings"
	"time"
)

type CodexSourceState uint8

const (
	CodexSourceUnknown CodexSourceState = iota
	CodexSourceVerified
	CodexSourceConflict
)

type CodexSourceProof struct {
	State                                        CodexSourceState
	ThreadID, FilenameID, HeadID                 string
	ParentCandidate, ForkedFromID, RootSessionID string
	RootTurnID                                   string
	Head                                         CodexSpan
	HeadTime, Birth                              time.Time
	Mixed                                        bool
}

type CodexExecutionKind uint8

const (
	CodexUnresolved CodexExecutionKind = iota
	CodexForeign
	CodexExplicit
	CodexSingleSession
	CodexBoundTurn
	CodexRightAnchored
	CodexPairedTool
)

type CodexResolvedRecord struct {
	Record                  CodexRecord
	Execution               CodexExecutionKind
	ThreadID, TurnID, Model string
	TurnIndex               int
	Context, RightAnchor    *CodexSpan
	ToolStart               *CodexSpan
}

// Local reports proved source/thread ownership. It does not certify a usage
// vector, the uniqueness of a request, or a match for a tool completion.
func (r CodexResolvedRecord) Local() bool {
	return r.Execution >= CodexExplicit
}

// CodexSegmentProof describes a candidate range. Unresolved records within
// the range remain unresolved even when some records establish a local turn.
type CodexSegmentProof struct {
	Span                 CodexSpan
	Context, RightAnchor *CodexSpan
	ThreadID, TurnID     string
}

type CodexCheckpoint struct {
	ReadEnd, CompleteEnd, ReplayFrom int64
	Partial                          bool
}

type CodexReduction struct {
	Source     CodexSourceProof
	Records    []CodexResolvedRecord
	Segments   []CodexSegmentProof
	Checkpoint CodexCheckpoint
}

// CodexCapturedReduction holds the read-only result of a verified capture.
// An ordinary reduction has no capture binding.
type CodexCapturedReduction struct {
	capture   CodexPrefixProof
	reduction CodexReduction
}

func (r CodexCapturedReduction) Capture() CodexPrefixProof {
	return r.capture
}

func (r CodexCapturedReduction) MatchesCapture(proof CodexPrefixProof) bool {
	return r.capture.attempt != nil && r.capture == proof
}

func (r CodexCapturedReduction) Source() CodexSourceProof {
	return r.reduction.Source
}

func (r CodexCapturedReduction) Checkpoint() CodexCheckpoint {
	return r.reduction.Checkpoint
}

// ReplayPrefix is the verified prefix from this same checkpoint replay.
// One-shot captures do not carry a checkpoint version.
func (r CodexCapturedReduction) ReplayPrefix() (CodexCheckpointPrefix, bool) {
	p := r.capture
	if p.attempt == nil || p.checkpointVersion == (CodexCheckpointVersion{}) {
		return CodexCheckpointPrefix{}, false
	}
	return CodexCheckpointPrefix{Version: p.checkpointVersion, Identity: p.identity, CompleteEnd: p.progress.CompleteEnd}, true
}

func (r CodexCapturedReduction) Records() iter.Seq[CodexResolvedRecord] {
	return func(yield func(CodexResolvedRecord) bool) {
		for _, record := range r.reduction.Records {
			if !yield(copyCodexResolvedRecord(record)) {
				return
			}
		}
	}
}

func (r CodexCapturedReduction) Segment(index int) (CodexSegmentProof, bool) {
	if index < 0 || index >= len(r.reduction.Segments) {
		return CodexSegmentProof{}, false
	}
	return copyCodexSegmentProof(r.reduction.Segments[index]), true
}

func copyCodexResolvedRecord(record CodexResolvedRecord) CodexResolvedRecord {
	record.Context = copyCodexSpan(record.Context)
	record.RightAnchor = copyCodexSpan(record.RightAnchor)
	record.ToolStart = copyCodexSpan(record.ToolStart)
	return record
}

func copyCodexSegmentProof(segment CodexSegmentProof) CodexSegmentProof {
	segment.Context = copyCodexSpan(segment.Context)
	segment.RightAnchor = copyCodexSpan(segment.RightAnchor)
	return segment
}

func copyCodexSpan(span *CodexSpan) *CodexSpan {
	if span == nil {
		return nil
	}
	value := *span
	return &value
}

// ReduceCodexProof evaluates a complete ordered observation prefix, from byte
// zero to CompleteEnd, of one caller-verified generation. Re-evaluate it with
// added observations when resuming: later evidence can invalidate prior
// inferences. For verified sources, ReplayFrom marks the open range awaiting
// append evidence. A verified complete prefix is still required for reduction.
// The caller owns capture limits and generation validation.
func ReduceCodexProof(path string, records []CodexRecord, progress CodexReadProgress) (CodexReduction, error) {
	var result CodexReduction
	if progress.CompleteEnd < 0 || progress.ReadEnd < progress.CompleteEnd ||
		progress.Partial != (progress.ReadEnd > progress.CompleteEnd) {
		return result, errCodexRange
	}
	var end int64
	for _, record := range records {
		if record.Span.Start != end || record.Span.End <= end {
			return result, errCodexRange
		}
		end = record.Span.End
	}
	if end != progress.CompleteEnd {
		return result, errCodexRange
	}

	result.Source = identifyCodexSource(path, records)
	result.Checkpoint = CodexCheckpoint{progress.ReadEnd, progress.CompleteEnd, progress.CompleteEnd, progress.Partial}
	result.Records = make([]CodexResolvedRecord, len(records))
	for i, record := range records {
		result.Records[i].Record = record
	}
	if result.Source.State != CodexSourceVerified {
		if result.Source.State == CodexSourceUnknown {
			result.Checkpoint.ReplayFrom = 0
		}
		return result, nil
	}

	segments := splitCodexSegments(records, result.Source.ThreadID)
	uses := collectCodexTurns(records, segments, result.Source.ThreadID)
	for _, segment := range segments {
		result.resolveSegment(segment, uses)
	}
	result.numberTurns(make(map[string]int))
	return result, nil
}

func codexFilenameThread(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		path = path[i+1:]
	}
	stem, ok := strings.CutPrefix(path, "rollout-")
	if !ok {
		return ""
	}
	stem, ok = strings.CutSuffix(stem, ".jsonl")
	const stamp = "0000-00-00T00-00-00"
	if !ok || len(stem) <= len(stamp) || stem[len(stamp)] != '-' {
		return ""
	}
	for i, c := range stamp {
		if c == '0' {
			if stem[i] < '0' || stem[i] > '9' {
				return ""
			}
		} else if stem[i] != byte(c) {
			return ""
		}
	}
	thread, rollout, reverted := strings.Cut(stem[len(stamp)+1:], "_")
	if reverted && codexUUID(rollout) == "" {
		return ""
	}
	return codexUUID(thread)
}

func identifyCodexSource(path string, records []CodexRecord) CodexSourceProof {
	scan := newCodexSourceScan(path)
	for _, record := range records {
		scan.observe(record)
	}
	return scan.proof()
}

// Keep only the opening record and the finite set of possible source IDs.
// The mixed and legacy flags account for all later records before resolution.
type codexSourceScan struct {
	source                       CodexSourceProof
	head                         CodexRecord
	seen, stopped                bool
	mixed, legacy, corroborating bool
	corroborated                 bool
}

func newCodexSourceScan(path string) codexSourceScan {
	return codexSourceScan{
		source:  CodexSourceProof{FilenameID: codexFilenameThread(path)},
		stopped: !strings.HasSuffix(path, ".jsonl"),
	}
}

func (s *codexSourceScan) observe(record CodexRecord) {
	if s.stopped {
		return
	}
	if !s.seen {
		if record.Kind == CodexIgnored && record.ThreadID == "" && !record.BadThread {
			return
		}
		s.seen, s.head = true, record
		if record.Kind.isGap() || record.BadThread {
			s.stopped = true
			return
		}
		if record.Kind == CodexSessionMeta {
			s.source.Head, s.source.HeadID = record.Span, record.HeadID
			if record.HasHeadID && record.HeadID == "" {
				s.stopped = true
				return
			}
			if record.HeadID != "" && (s.source.FilenameID != "" && s.source.FilenameID != record.HeadID ||
				record.ThreadID != "" && record.ThreadID != record.HeadID) {
				s.source.State, s.stopped = CodexSourceConflict, true
				return
			}
		}
		clean := !record.BadIdentity && !record.BadLegacyID
		s.legacy = clean && record.LegacyID != "" &&
			(s.source.FilenameID == "" || s.source.FilenameID == record.LegacyID)
		s.corroborating = clean && s.source.FilenameID != "" && record.Kind == CodexSessionMeta
	} else if s.corroborating {
		if record.Kind == CodexSessionMeta || record.Kind.isGap() {
			s.corroborating = false
		} else if record.Kind == CodexRequestUsage && !record.BadThread && record.ThreadID == s.source.FilenameID {
			s.corroborated, s.corroborating = true, false
		}
	}
	s.mixed = s.mixed || codexMixedIdentity(record, s.head.HeadID)
	s.legacy = s.legacy && !codexMixedIdentity(record, s.head.LegacyID) &&
		(!record.HasHeadID || record.HeadID != "")
}

func codexMixedIdentity(record CodexRecord, thread string) bool {
	return record.MixedSource || record.BadIdentity || record.BadThread || record.BadLegacyID ||
		record.HeadID != "" && record.HeadID != thread ||
		record.LegacyID != "" && record.LegacyID != thread ||
		record.ThreadID != "" && record.ThreadID != thread
}

func (s codexSourceScan) proof() CodexSourceProof {
	source, head := s.source, s.head
	if s.stopped {
		return source
	}
	switch {
	case head.HeadID != "":
		source.ThreadID, source.Mixed = head.HeadID, s.mixed
	case s.legacy:
		source.ThreadID = head.LegacyID
	case s.corroborated:
		// The corroborating request says nothing about earlier bare rows.
		source.ThreadID, source.Mixed = source.FilenameID, true
	default:
		return source
	}
	source.State = CodexSourceVerified
	if head.Kind == CodexSessionMeta {
		source.HeadTime = head.HeadAt
		if source.HeadTime.IsZero() {
			source.HeadTime = head.At
		}
		source.Birth = source.HeadTime
		source.RootSessionID, source.RootTurnID = head.LegacyID, head.RootTurnID
		source.ForkedFromID = head.ForkedFromID
		if head.ParentID != source.ThreadID {
			source.ParentCandidate = head.ParentID
		}
	}
	return source
}

// ResolveCodexParents drops conflicting or cyclic direct edges. A missing
// parent source does not change locally established execution or flatten edges.
func ResolveCodexParents(sources []CodexSourceProof) map[string]string {
	edges := make(map[string]string)
	conflicts := make(map[string]bool)
	for _, source := range sources {
		if source.State != CodexSourceVerified || source.ParentCandidate == "" {
			continue
		}
		if parent := edges[source.ThreadID]; parent != "" && parent != source.ParentCandidate {
			conflicts[source.ThreadID] = true
		}
		edges[source.ThreadID] = source.ParentCandidate
	}
	for child := range conflicts {
		delete(edges, child)
	}
	done := make(map[string]bool)
	for child := range edges {
		path := make([]string, 0)
		positions := make(map[string]int)
		for node := child; edges[node] != "" && !done[node]; node = edges[node] {
			if i, seen := positions[node]; seen {
				for _, member := range path[i:] {
					delete(edges, member)
				}
				break
			}
			positions[node] = len(path)
			path = append(path, node)
		}
		for _, member := range path {
			done[member] = true
		}
	}
	return edges
}

type codexTurnUse struct {
	contexts int
	foreign  bool
}

func collectCodexTurns(records []CodexRecord, segments []codexSegment, thread string) map[string]codexTurnUse {
	uses := make(map[string]codexTurnUse)
	for _, record := range records {
		noteCodexTurn(uses, record, thread)
	}
	for _, segment := range segments {
		noteCodexBareStart(uses, records[segment.first], segment)
	}
	return uses
}

func noteCodexTurn(uses map[string]codexTurnUse, record CodexRecord, thread string) {
	if record.TurnID == "" || record.Kind == CodexSessionMeta {
		return
	}
	use := uses[record.TurnID]
	if record.Kind == CodexTurnContext {
		use.contexts++
	}
	use.foreign = use.foreign || record.ThreadID != "" && record.ThreadID != thread
	uses[record.TurnID] = use
}

func noteCodexBareStart(uses map[string]codexTurnUse, first CodexRecord, segment codexSegment) {
	// A separate bare opening with no context is a competing turn claim.
	if segment.context < 0 && first.Kind == CodexTurnStart && first.ThreadID == "" && first.TurnID != "" {
		use := uses[first.TurnID]
		use.contexts++
		uses[first.TurnID] = use
	}
}

type codexSegment struct {
	first, end, context int
	turn                string
	open                bool
	afterIdentityGap    bool
}

func splitCodexSegments(records []CodexRecord, thread string) []codexSegment {
	var segments []codexSegment
	afterIdentityGap := false
	segment := codexSegment{context: -1}
	flush := func(end int) {
		if segment.first < end {
			segment.end = end
			segments = append(segments, segment)
		}
		segment = codexSegment{first: end, context: -1, afterIdentityGap: afterIdentityGap}
	}
	for i, record := range records {
		if codexSegmentFence(record, thread) {
			flush(i)
			afterIdentityGap = afterIdentityGap || record.losesCodexIdentity()
			flush(i + 1)
			continue
		}
		if segment.startsBefore(record) {
			flush(i)
		}
		if record.Kind == CodexTurnContext {
			segment.context = i
		}
		if record.TurnID != "" {
			segment.turn = record.TurnID
		}
		if record.Kind == CodexTurnEnd {
			flush(i + 1)
		}
	}
	segment.open = true
	flush(len(records))
	return segments
}

// An unreadable record can hide an identity switch. A new header without
// an identifiable thread also ends single-session attribution. Explicit
// execution and newly right-anchored evidence can still establish local facts.
func (r CodexRecord) losesCodexIdentity() bool {
	return r.Kind == CodexMalformed || r.Kind == CodexOversize ||
		r.Kind == CodexSessionMeta && (r.BadThread || r.BadLegacyID ||
			r.HeadID == "" && (r.HasHeadID || r.LegacyID == ""))
}

func codexSegmentFence(record CodexRecord, thread string) bool {
	return record.Kind == CodexSessionMeta || record.Kind.isGap() || record.BadThread || record.BadIdentity ||
		record.ThreadID != "" && record.ThreadID != thread
}

func (s codexSegment) startsBefore(record CodexRecord) bool {
	return record.Kind == CodexTurnStart || record.Kind == CodexTurnContext && (s.context >= 0 || s.turn == "") ||
		record.TurnID != "" && s.turn != "" && record.TurnID != s.turn
}

func (r *CodexReduction) resolveSegment(segment codexSegment, uses map[string]codexTurnUse) {
	records := r.Records[segment.first:segment.end]
	span := CodexSpan{records[0].Record.Span.Start, records[len(records)-1].Record.Span.End}
	proof := CodexSegmentProof{Span: span}
	context, anchor, right := r.segmentAnchors(segment, uses)
	turnUse := uses[segment.turn]
	uncontestedTurn := segment.turn != "" && turnUse.contexts <= 1 && !turnUse.foreign
	uniqueTurn := context >= 0 && uncontestedTurn
	if context >= 0 {
		proof.Context = &r.Records[context].Record.Span
	}
	if right >= 0 {
		proof.RightAnchor = &r.Records[right].Record.Span
	}
	starts := make(map[string]int)
	for i := range records {
		record := &records[i]
		r.resolveRecord(record, segment, segment.first+i, context, anchor, right, uniqueTurn)
		if record.Record.Kind == CodexToolStart && record.Record.CallID != "" {
			starts[record.Record.CallID]++
		}
	}
	localStarts := make(map[string]*CodexResolvedRecord)
	for i := range records {
		record := &records[i]
		switch record.Record.Kind {
		case CodexToolStart:
			if record.Local() && record.Record.CallID != "" && starts[record.Record.CallID] == 1 {
				localStarts[record.Record.CallID] = record
			}
		case CodexToolEnd:
			start := localStarts[record.Record.CallID]
			if start == nil || record.Record.BadThread || record.Record.BadIdentity || record.Execution == CodexForeign ||
				record.Record.TurnID != "" && record.Record.TurnID != start.TurnID {
				break
			}
			record.ToolStart = &start.Record.Span
			if !record.Local() {
				record.Execution, record.ThreadID, record.TurnID = CodexPairedTool, start.ThreadID, start.TurnID
				record.Context, record.RightAnchor, record.Model = start.Context, start.RightAnchor, start.Model
			} else if record.TurnID == "" {
				record.TurnID = start.TurnID
				record.Context, record.RightAnchor, record.Model = start.Context, start.RightAnchor, start.Model
			}
			delete(localStarts, record.Record.CallID)
		}
		if segment.open && record.Execution == CodexUnresolved && record.Record.Kind != CodexIgnored {
			followsContext := segment.context >= 0 && segment.first+i >= segment.context && (context >= 0 || uncontestedTurn)
			referencesTurn := uncontestedTurn && record.Record.TurnID == segment.turn
			if followsContext || referencesTurn {
				r.Checkpoint.ReplayFrom = span.Start
			}
		}
		if record.Local() {
			proof.ThreadID = r.Source.ThreadID
			if record.TurnID != "" {
				proof.TurnID = record.TurnID
			}
		}
	}
	if segment.open && len(localStarts) != 0 {
		r.Checkpoint.ReplayFrom = span.Start
	}
	r.Segments = append(r.Segments, proof)
}

func (r *CodexReduction) segmentAnchors(segment codexSegment, uses map[string]codexTurnUse) (context, anchor, right int) {
	context, anchor, right = -1, -1, -1
	if segment.context < 0 {
		return
	}
	c := r.Records[segment.context].Record
	if c.TurnID == "" || c.TurnID != segment.turn || c.BadIdentity || c.BadThread ||
		c.ThreadID != "" && c.ThreadID != r.Source.ThreadID {
		return
	}
	use := uses[c.TurnID]
	for i := segment.context + 1; i < segment.end; i++ {
		record := r.Records[i].Record
		if record.TurnID != c.TurnID || record.BadIdentity || record.BadThread {
			continue
		}
		switch {
		case record.Kind == CodexRequestUsage && record.ThreadID == r.Source.ThreadID:
			anchor = i
		case record.Kind == CodexTurnEnd && (record.ThreadID == r.Source.ThreadID ||
			c.ThreadID == r.Source.ThreadID || anchor >= 0) && (record.ThreadID != "" || use.contexts == 1 && !use.foreign):
			right = i
		}
	}
	if !r.Source.Mixed && !segment.afterIdentityGap || c.ThreadID == r.Source.ThreadID || use.contexts == 1 && !use.foreign && (anchor >= 0 || right >= 0) {
		context = segment.context
		if right < anchor {
			right = anchor
		}
	} else {
		anchor, right = -1, -1
	}
	return
}

func (r *CodexReduction) resolveRecord(resolved *CodexResolvedRecord, segment codexSegment, index, context, anchor, right int, uniqueTurn bool) {
	record := resolved.Record
	if record.BadIdentity || record.BadThread {
		return
	}
	switch record.Kind {
	case CodexIgnored, CodexMalformed, CodexSessionMeta, CodexBoundary, CodexOversize:
		return
	}
	if record.ThreadID != "" && record.ThreadID != r.Source.ThreadID {
		resolved.Execution, resolved.ThreadID, resolved.TurnID = CodexForeign, record.ThreadID, record.TurnID
		return
	}
	needsRightAnchor := record.Kind == CodexMessage && anchor >= 0 && index > anchor
	singleSession := !r.Source.Mixed && !segment.afterIdentityGap
	switch {
	case record.ThreadID == r.Source.ThreadID:
		resolved.Execution = CodexExplicit
	case singleSession:
		resolved.Execution = CodexSingleSession
	case uniqueTurn && !needsRightAnchor && record.TurnID == segment.turn:
		resolved.Execution = CodexBoundTurn
	case context >= 0 && index >= context && index <= right:
		resolved.Execution = CodexRightAnchored
	default:
		return
	}
	resolved.ThreadID, resolved.TurnID = r.Source.ThreadID, record.TurnID
	if resolved.TurnID == "" && (context >= 0 && index >= context && (index == context || index <= right) ||
		singleSession && segment.context >= 0 && index >= segment.context) {
		resolved.TurnID = segment.turn
	}
	if context >= 0 && resolved.TurnID == segment.turn {
		resolved.Context = &r.Records[context].Record.Span
		resolved.Model = r.Records[context].Record.Model
		if right >= 0 {
			// A later turn end is only necessary for records beyond the last
			// request anchor.
			if anchor >= 0 && index <= anchor {
				right = anchor
			}
			resolved.RightAnchor = &r.Records[right].Record.Span
		}
	}
}

func (r *CodexReduction) numberTurns(turns map[string]int) {
	for i := range r.Records {
		record := &r.Records[i]
		if !record.Local() {
			continue
		}
		if record.TurnID != "" {
			if turns[record.TurnID] == 0 {
				turns[record.TurnID] = len(turns) + 1
			}
			record.TurnIndex = turns[record.TurnID]
		}
		if r.Source.HeadTime.IsZero() && !record.Record.At.IsZero() &&
			(record.Record.Kind == CodexTurnContext || record.Record.Kind == CodexTurnStart ||
				record.Record.Kind == CodexRequestUsage && record.Execution == CodexExplicit) &&
			(r.Source.Birth.IsZero() || record.Record.At.Before(r.Source.Birth)) {
			r.Source.Birth = record.Record.At
		}
	}
}
