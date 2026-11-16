package sessionorigin

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

// ReducerVersion names the shared reduction and the canonical encodings below.
// Diagnostic results use this version; normal defaults name their rule.
const ReducerVersion = "origin-reducer-v2"

// Evidence purposes.
const (
	PurposeWork      = "work"
	PurposeAuxiliary = "auxiliary"
)

// Evidence is one rule's validated purpose for a canonical session.
type Evidence struct {
	Adapter          string
	RuleVersion      string
	Purpose          string
	SourceToken      string
	ValidatedVersion string
}

// Assessment is everything the reducer knows about one canonical session.
// Flags record why evidence could not be produced; they never stand in for it.
type Assessment struct {
	Evidence     []Evidence
	FallbackRule string
	// Unavailable names the rule whose source could not be read for this key.
	Unavailable string
	// Unsupported names the rule whose source marks this key outside its
	// supported entries.
	Unsupported string
	// TaskChild marks a task child with its own ID and no verifiable purpose.
	TaskChild bool
	// Conflicting marks mixed purpose or a cyclic relation reported by a source.
	Conflicting bool
	// InputMismatch marks evidence bound to another source generation.
	InputMismatch bool
}

// Result is the automatic classification of one canonical session.
type Result struct {
	Automatic   string
	ReasonCode  string
	RuleVersion string
}

// Reduce prioritizes stale inputs, conflicts, auxiliary evidence and source
// diagnostics before defaults. Default Work is a result, not work evidence.
func Reduce(a Assessment) Result {
	if a.InputMismatch {
		return Result{ClassWork, ReasonOriginInputMismatch, ReducerVersion}
	}
	auxiliary, work := []string{}, false
	for _, item := range a.Evidence {
		switch item.Purpose {
		case PurposeAuxiliary:
			auxiliary = append(auxiliary, item.RuleVersion)
		case PurposeWork:
			work = true
		}
	}
	sort.Slice(auxiliary, func(i, j int) bool {
		priority := func(id string) int {
			for _, r := range builtinRules {
				if r.ID == id {
					return r.Priority
				}
			}
			return 1000
		}
		a, b := priority(auxiliary[i]), priority(auxiliary[j])
		if a == b {
			return auxiliary[i] < auxiliary[j]
		}
		return a < b
	})
	if a.Conflicting || (work && len(auxiliary) > 0) {
		return Result{ClassWork, ReasonConflictingOrigin, ReducerVersion}
	}
	if len(auxiliary) > 0 {
		for _, rule := range builtinRules {
			if rule.Class == ClassAuxiliary && rule.matches(a) {
				return Result{ClassAuxiliary, ReasonMaintenanceOrigin, rule.ID}
			}
		}
		return Result{ClassAuxiliary, ReasonMaintenanceOrigin, auxiliary[0]}
	}
	// Diagnostics remain visible even though the session stays in Work.
	if a.Unavailable != "" {
		return Result{ClassWork, ReasonOriginSourceUnavailable, a.Unavailable}
	}
	if a.Unsupported != "" {
		return Result{ClassWork, ReasonOriginRuleUnsupported, a.Unsupported}
	}
	if a.TaskChild {
		return Result{ClassWork, ReasonTaskChildUnconfirmed, ReducerVersion}
	}
	for _, rule := range builtinRules {
		if rule.Class == ClassWork && rule.matches(a) {
			return Result{ClassWork, ReasonDefaultWork, rule.ID}
		}
	}
	panic("missing default rule")
}

// Effective overlays a user override on the automatic class. Only an active
// override bound to the current source applies; an unmatched one is kept for
// review and revocation but never classifies.
func Effective(automatic string, override *store.OriginOverride) (string, bool) {
	if override == nil || override.Value == nil || override.BindingState != BindingMatched {
		return automatic, false
	}
	return *override.Value, true
}

// Override binding states.
const (
	BindingMatched   = "matched"
	BindingUnmatched = "unmatched"
)

// SourceGeneration is one indexed copy's identity and generation.
type SourceGeneration struct {
	Identity      string
	PrefixHash    string
	ParserVersion int64
}

// ruleVersions are the ordered rules included in every input token.
var ruleVersions = BuiltinRuleIDs()

// WatchFingerprint binds a completed session inventory to its rule policy.
// A rule upgrade schedules classification even when source files are unchanged.
func WatchFingerprint(epoch int64, sourceFingerprint string) string {
	return "v2:" + strconv.FormatInt(epoch, 10) + ":" + sourceFingerprint +
		":" + ReducerVersion + ":" + strings.Join(ruleVersions, ",")
}

// InputToken is the canonical origin_input_token: SHA-256 over the canonical
// key, each current source's identity and generation, parser-supplied purpose
// evidence and the reducer and rule versions, all sorted. The session index
// and the classifier use this one encoding; it never reads a path or content,
// so an append or a move keeps the token and a rewrite changes it.
func InputToken(client, sessionID string, sources []SourceGeneration, parserEvidence []Evidence) string {
	return inputTokenWithRules(client, sessionID, sources, parserEvidence, ReducerVersion, ruleVersions)
}

func inputTokenWithRules(client, sessionID string, sources []SourceGeneration, parserEvidence []Evidence, reducer string, rules []string) string {
	lines := []string{"origin-input-v1", "reducer=" + reducer, "rules=" + strings.Join(rules, ","),
		"client=" + escape(client), "session=" + escape(sessionID)}
	copies := make([]string, 0, len(sources))
	for _, source := range sources {
		copies = append(copies, "source="+escape(source.Identity)+"|"+escape(source.PrefixHash)+"|"+strconv.FormatInt(source.ParserVersion, 10))
	}
	sort.Strings(copies)
	lines = append(lines, dedupe(copies)...)
	purposes := make([]string, 0, len(parserEvidence))
	for _, item := range parserEvidence {
		purposes = append(purposes, "evidence="+escape(item.RuleVersion)+"|"+escape(item.Purpose)+"|"+escape(item.SourceToken))
	}
	sort.Strings(purposes)
	lines = append(lines, dedupe(purposes)...)
	return digest(lines)
}

// Birth is the stable birth identity a source provides for a canonical session.
type Birth struct {
	// StartedAt is the fixed start time: the earliest start over the copies.
	StartedAt string
}

// BirthOf derives the stable birth from a session's indexed copies. Copies,
// moves and rebuilds keep it; appended events never move an earlier start.
func BirthOf(sources []store.SessionOriginSource) Birth {
	earliest := time.Time{}
	for _, source := range sources {
		at, err := time.Parse(time.RFC3339Nano, source.FirstAt)
		if err != nil {
			continue
		}
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	if earliest.IsZero() {
		return Birth{}
	}
	return Birth{StartedAt: earliest.UTC().Format(time.RFC3339Nano)}
}

// BindingFor is the override binding of a canonical session: a digest of the
// key and its stable birth, without path, content or inode. An incomplete
// birth still binds locally but is marked limited.
func BindingFor(client, sessionID string, birth Birth) store.OriginBinding {
	lines := []string{"origin-binding-v1", "client=" + escape(client), "session=" + escape(sessionID), "started_at=" + escape(birth.StartedAt)}
	return store.OriginBinding{Digest: digest(lines), Limited: birth.StartedAt == ""}
}

func escape(value string) string {
	return strconv.Quote(value)
}

func dedupe(sorted []string) []string {
	out := sorted[:0]
	for index, item := range sorted {
		if index == 0 || item != sorted[index-1] {
			out = append(out, item)
		}
	}
	return out
}

func digest(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return hex.EncodeToString(sum[:])
}
