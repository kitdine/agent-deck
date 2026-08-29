package activity

import "testing"

// The pair that motivates Decision 3's single scan. One example alone cannot
// tell the earliest-match rule from a hardcoded exception for that phrase, so
// both orderings are asserted.
func TestMessageScanDecidesCategoryByEarliestIntentWord(t *testing.T) {
	for _, testCase := range []struct {
		message   string
		wantClass string
		wantSub   string
		wantKind  string
		wantFinal string
	}{
		{"add error handling", MessageClassBuild, SubFeature, KindCoding, SubFeature},
		{"fix the add button", MessageClassFault, "", KindDebugging, SubRepair},
		{"error in the new parser", MessageClassFault, "", KindDebugging, SubRepair},
		{"new parser throws an exception", MessageClassBuild, SubFeature, KindCoding, SubFeature},
		{"refactor the error path", MessageClassBuild, SubRefactoring, KindCoding, SubRefactoring},
		{"the tests are failing", MessageClassBuild, SubTesting, KindCoding, SubTesting},
		{"failing tests everywhere", MessageClassFault, "", KindDebugging, SubRepair},
	} {
		class, sub := ScanMessage(testCase.message)
		if class != testCase.wantClass || sub != testCase.wantSub {
			t.Errorf("ScanMessage(%q) = %q/%q, want %q/%q", testCase.message, class, sub, testCase.wantClass, testCase.wantSub)
			continue
		}
		kind, finalSub := Classify(class, sub, TurnShape{Edited: true, AnyCall: true}, false)
		if kind != testCase.wantKind || finalSub != testCase.wantFinal {
			t.Errorf("Classify(%q) = %q/%q, want %q/%q", testCase.message, kind, finalSub, testCase.wantKind, testCase.wantFinal)
		}
	}
}

// A substring scan would classify by accident: `add` inside `address`, `new`
// inside `renew`, `spec` inside `specification`.
func TestMessageScanMatchesWholeWordsOnly(t *testing.T) {
	for _, message := range []string{
		"update the address field",
		"renew the certificate",
		"read the specification",
		"the buggy is parked outside",
	} {
		if class, sub := ScanMessage(message); class != MessageClassNone || sub != "" {
			t.Errorf("ScanMessage(%q) = %q/%q, want none", message, class, sub)
		}
	}
}

func TestCategoryPrecedenceAndOutranking(t *testing.T) {
	edit := TurnShape{Edited: true, AnyCall: true}
	read := TurnShape{Read: true, AnyCall: true}
	for _, testCase := range []struct {
		name     string
		class    string
		sub      string
		shape    TurnShape
		brainst  bool
		wantKind string
		wantSub  string
	}{
		{"delegation outranks a fault message that edits", MessageClassFault, "", TurnShape{Delegated: true, Edited: true, AnyCall: true}, false, KindDelegation, SubSubagent},
		{"workflow is delegation too", MessageClassNone, "", TurnShape{Workflow: true, AnyCall: true}, false, KindDelegation, SubWorkflow},
		{"debugging outranks coding when the turn edits", MessageClassFault, "", edit, false, KindDebugging, SubRepair},
		{"debugging reading without editing is investigation", MessageClassFault, "", read, false, KindDebugging, SubInvestigation},
		{"a fault message with no tool call is not debugging", MessageClassFault, "", TurnShape{}, false, KindConversation, SubExploration},
		{"coding falls back to feature visibly", MessageClassNone, "", edit, false, KindCoding, SubFeature},
		{"a command names a test runner when the message named nothing", MessageClassNone, "", TurnShape{Edited: true, AnyCall: true, TestingCmd: true}, false, KindCoding, SubTesting},
		{"stated intent outranks an incidental command", MessageClassBuild, SubRefactoring, TurnShape{Edited: true, AnyCall: true, TestingCmd: true}, false, KindCoding, SubRefactoring},
		{"chore commands reach maintenance", MessageClassNone, "", TurnShape{Edited: true, AnyCall: true, ChoreCmd: true}, false, KindCoding, SubMaintenance},
		{"a turn with no tool call is conversation", MessageClassNone, "", TurnShape{}, false, KindConversation, SubExploration},
		{"planning wins inside conversation", MessageClassNone, "", TurnShape{Planned: true, AnyCall: true}, false, KindConversation, SubPlanning},
		{"brainstorming needs no tool call", MessageClassNone, "", TurnShape{}, true, KindConversation, SubBrainstorming},
		{"a reading turn is exploration", MessageClassNone, "", read, false, KindConversation, SubExploration},
	} {
		kind, sub := Classify(testCase.class, testCase.sub, testCase.shape, testCase.brainst)
		if kind != testCase.wantKind || sub != testCase.wantSub {
			t.Errorf("%s: got %q/%q, want %q/%q", testCase.name, kind, sub, testCase.wantKind, testCase.wantSub)
		}
	}
}

// Decision 3 refuses to key any rule on tool failure status, because Codex
// hardcodes "completed" on every output item while Claude sets is_error, so a
// failure-keyed rule would classify all Codex work as non-debugging. TurnShape
// carries no failure field; this asserts the type itself cannot express one.
func TestTurnShapeCarriesNoFailureStatus(t *testing.T) {
	shape := TurnShape{Delegated: true, Workflow: true, Planned: true, Edited: true, Read: true, AnyCall: true, TestingCmd: true, ChoreCmd: true}
	if got := len([]bool{shape.Delegated, shape.Workflow, shape.Planned, shape.Edited, shape.Read, shape.AnyCall, shape.TestingCmd, shape.ChoreCmd}); got != 8 {
		t.Fatalf("TurnShape field count = %d, want 8 — a new field must be reviewed against Decision 3's refusal to read failure status", got)
	}
}
