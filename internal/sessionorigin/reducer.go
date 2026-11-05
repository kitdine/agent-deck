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
// It is the rule_version of a result no maintenance rule produced.
const ReducerVersion = "origin-reducer-v1"

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
	Evidence []Evidence
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

// Reduce applies the classification rules of architecture §3.3 in one fixed
// order. A missing mapping never proves work, and nothing but validated
// maintenance evidence yields auxiliary.
func Reduce(a Assessment) Result {
	if a.InputMismatch {
		return Result{ClassUnknown, ReasonOriginInputMismatch, ReducerVersion}
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
	sort.Strings(auxiliary)
	if a.Conflicting || (work && len(auxiliary) > 0) {
		return Result{ClassUnknown, ReasonConflictingOrigin, ReducerVersion}
	}
	if len(auxiliary) > 0 {
		return Result{ClassAuxiliary, ReasonMaintenanceOrigin, auxiliary[0]}
	}
	// Explicit work evidence alone has no reason code yet (adding one is a
	// contract change for all four surfaces), so it stays conservatively unknown.
	if a.Unavailable != "" {
		return Result{ClassUnknown, ReasonOriginSourceUnavailable, a.Unavailable}
	}
	if a.Unsupported != "" {
		return Result{ClassUnknown, ReasonOriginRuleUnsupported, a.Unsupported}
	}
	if a.TaskChild {
		return Result{ClassUnknown, ReasonTaskChildUnconfirmed, ReducerVersion}
	}
	return Result{ClassUnknown, ReasonOriginMetadataMissing, ReducerVersion}
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

// ruleVersions are the maintenance rules this reducer version evaluates.
var ruleVersions = []string{RuleClaudeMemSDKMapV1, RuleCodexOTelBackgroundV1}

// InputToken is the canonical origin_input_token: SHA-256 over the canonical
// key, each current source's identity and generation, parser-supplied purpose
// evidence and the reducer and rule versions, all sorted. The session index
// and the classifier use this one encoding; it never reads a path or content,
// so an append or a move keeps the token and a rewrite changes it.
func InputToken(client, sessionID string, sources []SourceGeneration, parserEvidence []Evidence) string {
	lines := []string{"origin-input-v1", "reducer=" + ReducerVersion, "rules=" + strings.Join(ruleVersions, ","),
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
