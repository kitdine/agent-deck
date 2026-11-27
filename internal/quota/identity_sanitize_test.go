package quota

import (
	"strings"
	"testing"
)

func TestIdentityEmailMaskingUsesGraphemeBoundaries(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"a@EXAMPLE.TEST", "***@example.test"},
		{"ab@example.test", "a***@example.test"},
		{"abc@example.test", "a***@example.test"},
		{"a\u0301bc👩‍👧@example.test", ""}, // ZWJ is a format character.
		{"a\u0301bc👍🏽@example.test", "á***👍🏽@example.test"},
		{"@example.test", ""},
		{"a@.example", ""},
		{"a@exam ple.test", ""},
		{"a@one@two.test", ""},
		{strings.Repeat("a", 65) + "@example.test", ""},
	} {
		_, _, masked, ok := maskedIdentityEmail(tc.raw)
		if masked != tc.want || ok != (tc.want != "") {
			t.Errorf("fixture %q: got %q, %t; want %q", tc.raw, masked, ok, tc.want)
		}
	}
}

func TestIdentityForbiddenValuesAndNameSanitization(t *testing.T) {
	for _, tc := range []struct {
		id, local, candidate, want string
	}{
		{"ABCDE", "", "abcde", ""},
		{"abc", "", "ABC", ""},
		{"abc", "", "X-ABC", "X-ABC"},
		{"abcd", "", "X-ABCD", ""},
		{"", "abc", "X-ABC", ""},
		{"", "", "prefix person@example.test suffix", ""},
		{"straße", "", "X-STRASSE", ""},
		{"σσσσ", "", "X-ΣΣΣς", ""},
		{"ᎠᎠᎠᎠ", "", "X-ꭰꭰꭰꭰ", ""},
		{"ÁBCD", "", "X-a\u0301bcd", ""},
		{"", "", "\u202eA\u00a0\u2003B\uE000\uFDD0", "A B"},
		{"", "", strings.Repeat("a", 63) + "👍🏽suffix", strings.Repeat("a", 63) + "👍🏽"},
	} {
		if got := sanitizeIdentityName(tc.candidate, newIdentityForbiddenSet(tc.local, tc.id)); got != tc.want {
			t.Errorf("sanitized name %q = %q, want %q", tc.candidate, got, tc.want)
		}
	}
}

func TestClaudePlanProjectionSeparatesConflictAndRejectedValues(t *testing.T) {
	for _, tc := range []struct {
		tier, subscription planField
		state              PlanState
		reason             PlanMissingReason
		name, basis        string
		count              int
	}{
		{state: PlanMissing, reason: PlanNotReported},
		{tier: planField{value: "pro"}, state: PlanKnown, name: "Claude Pro", basis: "mapping", count: 1},
		{tier: planField{value: "pro"}, subscription: planField{value: "pro"}, state: PlanKnown, name: "Claude Pro", basis: "mapping", count: 2},
		{subscription: planField{value: "Brand_X.2"}, state: PlanRawOnly, name: "Brand_X.2", basis: "raw", count: 1},
		{tier: planField{value: "Pro"}, subscription: planField{value: "pro"}, state: PlanConflict, count: 2},
		{tier: planField{rejected: true, value: "must-not-escape"}, subscription: planField{value: "pro"}, state: PlanMissing, reason: PlanUnsafeValue},
	} {
		got := ProjectPlan(PlanSourceClaudeAuthStatus, PlanSources{planTier: tc.tier, subscriptionType: tc.subscription})
		if got.State != tc.state || got.Reason != tc.reason || got.Name != tc.name || string(got.Basis) != tc.basis || got.Raw == nil || len(got.Raw) != tc.count {
			t.Errorf("projection=%+v, expected %q / %q / %q / %q / %d", got, tc.state, tc.reason, tc.name, tc.basis, tc.count)
		}
	}
}
