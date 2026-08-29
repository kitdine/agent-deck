package activity

import "strings"

// Message classes and the coding subcategories a message can name. These are the
// only forms in which a user message leaves this package: Decision 2 permits
// reading the text and forbids keeping it, and Decision 11 needs its verdict to
// survive a scan boundary the text itself does not.
const (
	MessageClassNone  = "none"
	MessageClassBuild = "build"
	MessageClassFault = "fault"
)

// Command-shaped subcategory hints. A parsed command reduces to one of these
// before the command text is dropped; nothing else about it survives.
const (
	HintTesting = "testing"
	HintChore   = "chore"
)

// Activity kinds and subcategories, Decision 3's fixed vocabulary.
const (
	KindDelegation   = "delegation"
	KindDebugging    = "debugging"
	KindCoding       = "coding"
	KindConversation = "conversation"

	SubFeature       = "feature"
	SubRefactoring   = "refactoring"
	SubTesting       = "testing"
	SubMaintenance   = "maintenance"
	SubInvestigation = "investigation"
	SubRepair        = "repair"
	SubExploration   = "exploration"
	SubBrainstorming = "brainstorming"
	SubPlanning      = "planning"
	SubSubagent      = "subagent"
	SubWorkflow      = "workflow"
)

// The five vocabularies that take part in the single message scan. `maintenance`
// is absent on purpose: it has no message rule and is command-shaped only.
var (
	faultWords = []string{
		"fix", "bug", "error", "broken", "failing", "crash", "traceback",
		"exception", "not working",
		"400", "401", "403", "404", "409", "422", "500", "502", "503", "504",
	}
	featureWords     = []string{"add", "create", "implement", "new", "build", "scaffold", "generate"}
	refactoringWords = []string{"refactor", "rename", "clean up", "simplify", "extract", "restructure", "split", "migrate"}
	testingWords     = []string{"test", "tests", "testing", "spec", "coverage", "assertion", "regression"}

	brainstormWords = []string{"brainstorm", "idea", "what if", "approach", "should we", "opinion", "suggest"}
)

// TurnShape is what the turn did, read back from persisted tool calls rather
// than carried across scans. Decision 11 keeps message and tool shape apart for
// exactly this reason: only the message has no other home.
type TurnShape struct {
	Delegated  bool // a subagent spawn
	Workflow   bool // a skill or workflow invocation
	Planned    bool // plan mode or a to-do tool
	Edited     bool // at least one edit-shaped call
	Read       bool // at least one read-shaped call
	AnyCall    bool // any tool call at all
	TestingCmd bool // a command named a test runner
	ChoreCmd   bool // a command named git, a build, or a dependency install
}

// ScanMessage performs Decision 3's single pass. The earliest match across the
// fault vocabulary and the three build vocabularies decides both the message
// class and, when it is a build word, the coding subcategory it names. Ties
// break toward fault, then in the order the Subcategories table lists them.
func ScanMessage(text string) (messageClass, intentSub string) {
	if strings.TrimSpace(text) == "" {
		return MessageClassNone, ""
	}
	lowered := strings.ToLower(text)
	best := -1
	class, sub := MessageClassNone, ""
	// Ordered so that an equal position resolves to fault first, then feature,
	// refactoring, testing — the order the Subcategories table lists.
	for _, candidate := range []struct {
		words []string
		class string
		sub   string
	}{
		{faultWords, MessageClassFault, ""},
		{featureWords, MessageClassBuild, SubFeature},
		{refactoringWords, MessageClassBuild, SubRefactoring},
		{testingWords, MessageClassBuild, SubTesting},
	} {
		at := earliestWord(lowered, candidate.words)
		if at < 0 {
			continue
		}
		if best < 0 || at < best {
			best, class, sub = at, candidate.class, candidate.sub
		}
	}
	return class, sub
}

// Classify applies Decision 3's category precedence to a message reduction and a
// turn shape. It consults no tool failure status: Codex records every output
// item as completed while Claude sets is_error, so any failure-keyed rule would
// classify all Codex work as non-debugging.
func Classify(messageClass, intentSub string, shape TurnShape, brainstorming bool) (kind, sub string) {
	switch {
	case shape.Delegated:
		return KindDelegation, SubSubagent
	case shape.Workflow:
		return KindDelegation, SubWorkflow
	}
	if messageClass == MessageClassFault && (shape.Edited || shape.Read) {
		if shape.Edited {
			return KindDebugging, SubRepair
		}
		return KindDebugging, SubInvestigation
	}
	if shape.Edited {
		return KindCoding, codingSub(intentSub, shape)
	}
	return KindConversation, ConversationSub(shape, brainstorming)
}

// codingSub prefers the message-derived intent: a user's stated intent outranks
// an incidental command in the same turn. The command-shaped rules apply only
// when the message named nothing, and `feature` is the visible fallback — a
// definition rather than a silent default.
func codingSub(intentSub string, shape TurnShape) string {
	if intentSub != "" {
		return intentSub
	}
	switch {
	case shape.TestingCmd:
		return SubTesting
	case shape.ChoreCmd:
		return SubMaintenance
	}
	return SubFeature
}

// BrainstormingApplies reports whether a tool-less turn's message matches the
// brainstorming vocabulary. It is separate from ScanMessage because that scan
// decides the category, and this vocabulary decides only a conversation
// subcategory — mixing them would let "what if we add caching" open a coding
// turn on a message that called no tool.
//
// It is answered from the message in hand, so it is available only while the
// message is being read. Decision 2's persisted set carries message_class and
// intent_sub and nothing else, so a turn whose message and assistant reply fall
// in different scans reaches ConversationSub with brainstorming false and takes
// the visible `exploration` fallback. That is the contract as written; widening
// the persisted set to carry a third bit is a design change, not an
// implementation detail.
func BrainstormingApplies(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	return earliestWord(strings.ToLower(text), brainstormWords) >= 0
}

// ConversationSub resolves Decision 3's three conversation subcategories. The
// first subcategory, `exploration`, is the visible fallback.
func ConversationSub(shape TurnShape, brainstorming bool) string {
	switch {
	case shape.Planned:
		return SubPlanning
	case shape.Read:
		return SubExploration
	case !shape.AnyCall && brainstorming:
		return SubBrainstorming
	}
	return SubExploration
}

// earliestWord returns the byte offset of the earliest whole-word match among
// words, or -1. Whole-word matching keeps `add` out of `address` and `new` out
// of `renew`; a substring scan would classify by accident.
func earliestWord(lowered string, words []string) int {
	best := -1
	for _, word := range words {
		for from := 0; from+len(word) <= len(lowered); {
			at := strings.Index(lowered[from:], word)
			if at < 0 {
				break
			}
			at += from
			if wordBoundary(lowered, at, len(word)) {
				if best < 0 || at < best {
					best = at
				}
				break
			}
			from = at + 1
		}
	}
	return best
}

func wordBoundary(text string, at, length int) bool {
	if at > 0 && isWordByte(text[at-1]) {
		return false
	}
	end := at + length
	return end >= len(text) || !isWordByte(text[end])
}

func isWordByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
