package quota

import (
	"testing"
	"time"
)

func TestApplyObservationFirstEverIsNotAReset(t *testing.T) {
	next := Observation{UsedPercent: 10, ObservedAt: time.Now()}
	updated, observedResetAt, reset := ApplyObservation(nil, next)
	if reset {
		t.Fatal("the first observation for a key must never be a reset")
	}
	if !observedResetAt.IsZero() {
		t.Fatal("observedResetAt must be zero when no reset was observed")
	}
	if updated.Prior != nil {
		t.Fatal("a first observation has no prior")
	}
}

func TestApplyObservationResetOnlyOnDecrease(t *testing.T) {
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	first := Observation{UsedPercent: 40, ObservedAt: t1, ResetsAt: t1.Add(2 * time.Hour)}
	stored := Stored{Latest: first}

	t2 := t1.Add(time.Hour)
	// resets_at has now passed, but used_percent increased: no local reset.
	increased := Observation{UsedPercent: 55, ObservedAt: t2, ResetsAt: first.ResetsAt}
	_, observedResetAt, reset := ApplyObservation(&stored, increased)
	if reset {
		t.Fatal("resets_at passing without a decrease must not be treated as a reset")
	}
	if !observedResetAt.IsZero() {
		t.Fatal("observedResetAt must stay zero when resets_at passes without a decrease")
	}

	t3 := t2.Add(time.Hour)
	decreased := Observation{UsedPercent: 5, ObservedAt: t3, ResetsAt: t3.Add(2 * time.Hour)}
	updated, observedResetAt, reset := ApplyObservation(&stored, decreased)
	if !reset {
		t.Fatal("a decrease in used_percent must be observed as a reset")
	}
	if !observedResetAt.Equal(t3) {
		t.Fatalf("observedResetAt = %v, want %v", observedResetAt, t3)
	}
	if updated.Prior == nil || updated.Prior.UsedPercent != first.UsedPercent {
		t.Fatal("the prior observation must carry forward exactly the previous latest")
	}
}

func TestApplyObservationRetainsOnlyOnePrior(t *testing.T) {
	t1 := time.Now()
	stored := Stored{Latest: Observation{UsedPercent: 10, ObservedAt: t1}}
	t2 := t1.Add(time.Hour)
	updated, _, _ := ApplyObservation(&stored, Observation{UsedPercent: 20, ObservedAt: t2})
	t3 := t2.Add(time.Hour)
	updated, _, _ = ApplyObservation(&updated, Observation{UsedPercent: 30, ObservedAt: t3})

	if updated.Latest.UsedPercent != 30 {
		t.Fatalf("Latest.UsedPercent = %v, want 30", updated.Latest.UsedPercent)
	}
	if updated.Prior == nil || updated.Prior.UsedPercent != 20 {
		t.Fatal("only the immediately preceding observation may be retained as Prior")
	}
}

func TestSelectObservationNewestWins(t *testing.T) {
	older := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(100, 0)}
	newer := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(200, 0)}
	got, ok := SelectObservation([]Observation{older, newer})
	if !ok || got.ObservedAt != newer.ObservedAt {
		t.Fatalf("SelectObservation must return the newest observation")
	}
}

func TestSelectObservationEqualAgePrefersStatusLine(t *testing.T) {
	at := time.Unix(100, 0)
	prose := Observation{Source: SourceClaudeProse, ObservedAt: at}
	statusLine := Observation{Source: SourceClaudeStatusLine, ObservedAt: at}

	got, ok := SelectObservation([]Observation{prose, statusLine})
	if !ok || got.Source != SourceClaudeStatusLine {
		t.Fatalf("equal-age tie must resolve to the status-line route, got %+v", got)
	}

	// Order in the input must not matter.
	got, ok = SelectObservation([]Observation{statusLine, prose})
	if !ok || got.Source != SourceClaudeStatusLine {
		t.Fatalf("equal-age tie must resolve to the status-line route regardless of order, got %+v", got)
	}
}

func TestSelectObservationEmpty(t *testing.T) {
	if _, ok := SelectObservation(nil); ok {
		t.Fatal("selecting from no candidates must report not-found")
	}
}

func TestAcceptNewerAlwaysWins(t *testing.T) {
	existing := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(100, 0)}
	newer := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(200, 0)}
	if !Accept(existing, newer) {
		t.Fatal("a strictly newer observation must be accepted")
	}
}

func TestAcceptRejectsOlder(t *testing.T) {
	existing := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(200, 0)}
	older := Observation{Source: SourceClaudeProse, ObservedAt: time.Unix(100, 0)}
	if Accept(existing, older) {
		t.Fatal("an older observation arriving later must not override the newer stored one (QD-R1-F1)")
	}
}

func TestAcceptEqualAgeTiebreak(t *testing.T) {
	at := time.Unix(100, 0)
	statusLine := Observation{Source: SourceClaudeStatusLine, ObservedAt: at}
	prose := Observation{Source: SourceClaudeProse, ObservedAt: at}

	if !Accept(prose, statusLine) {
		t.Fatal("an equal-age status-line observation must replace an equal-age prose one")
	}
	if Accept(statusLine, prose) {
		t.Fatal("an equal-age prose observation must not replace an equal-age status-line one (QD-R1-F1)")
	}
}

func TestApplyObservationCrossRoutePrecisionRequiresLargerDecrease(t *testing.T) {
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	stored := Stored{Latest: Observation{Source: SourceClaudeStatusLine, UsedPercent: 22.4, ObservedAt: t1}}

	t2 := t1.Add(5 * time.Minute)
	prose := Observation{Source: SourceClaudeProse, UsedPercent: 22, ObservedAt: t2}
	_, _, reset := ApplyObservation(&stored, prose)
	if reset {
		t.Fatal("a sub-threshold cross-route decrease (22.4->22) must not be a reset (QD-R1-F2)")
	}

	bigDrop := Observation{Source: SourceClaudeProse, UsedPercent: 5, ObservedAt: t2}
	statusLine := Stored{Latest: Observation{Source: SourceClaudeStatusLine, UsedPercent: 80, ObservedAt: t1}}
	_, observedResetAt, reset := ApplyObservation(&statusLine, bigDrop)
	if !reset || !observedResetAt.Equal(t2) {
		t.Fatal("a large cross-route decrease (80->5) must still be a reset")
	}
}

func TestApplyObservationSameSourceAnyDecreaseIsAReset(t *testing.T) {
	t1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	stored := Stored{Latest: Observation{Source: SourceClaudeStatusLine, UsedPercent: 22.4, ObservedAt: t1}}
	t2 := t1.Add(5 * time.Minute)
	next := Observation{Source: SourceClaudeStatusLine, UsedPercent: 22.1, ObservedAt: t2}
	_, _, reset := ApplyObservation(&stored, next)
	if !reset {
		t.Fatal("a same-source decrease must be a reset even when smaller than the cross-route threshold")
	}
}
