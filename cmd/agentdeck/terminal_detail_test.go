package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestRenderTerminalDetailUsesExplicitRolesAcrossLayouts(t *testing.T) {
	longNote := strings.Repeat("内容🧭e\u0301", 20)
	longToken := strings.Repeat("Z", 95)
	detail := terminalDetailModel{
		title: "模型😀\x1b[31m",
		fields: []terminalDetailField{
			{label: "PRICING STATUS", value: "partial", role: terminalDetailRoleWarning, priority: terminalDetailPrioritySecondary},
			{label: "INPUT TOKENS", value: "1,234", role: terminalDetailRoleToken, priority: terminalDetailPriorityPrimary},
			{label: "PROVIDER COST", value: "$12.340000000", role: terminalDetailRoleCost, priority: terminalDetailPriorityPrimary},
			{label: "EVENT", value: "42", role: terminalDetailRoleSession, priority: terminalDetailPriorityPrimary},
			{label: "COMPLETION", value: "complete", role: terminalDetailRoleSuccess, priority: terminalDetailPriorityPrimary},
			{label: "FAILURE", value: "failed", role: terminalDetailRoleError, priority: terminalDetailPriorityPrimary},
			{label: "MESSAGE", value: "FAILED COST TOKEN", role: terminalDetailRoleNeutral, priority: terminalDetailPriorityPrimary},
			{label: "LONG TOKEN", value: longToken, role: terminalDetailRoleNeutral, priority: terminalDetailPriorityPrimary},
		},
		notes: []terminalDetailNote{{
			text:     longNote,
			status:   "warning",
			role:     terminalDetailRoleWarning,
			priority: terminalDetailPriorityTertiary,
		}},
	}

	colorful := renderTerminalDetailModel(detail, 80, usageTextPrimitives{color: true})
	joined := strings.Join(colorful, "\n")
	for _, want := range []string{
		"DETAIL · 模型😀",
		"\x1b[1;96m1,234\x1b[0m",
		"\x1b[1;93m$12.340000000\x1b[0m",
		"\x1b[1;95m42\x1b[0m",
		"\x1b[1;92mcomplete\x1b[0m",
		"\x1b[1;91mfailed\x1b[0m",
		"\x1b[1;94mFAILED COST TOKEN\x1b[0m",
		"WARNING",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("semantic detail missing %q:\n%s", want, joined)
		}
	}
	if len(colorful) < 2 || !strings.Contains(colorful[1], "INPUT TOKENS") || !strings.Contains(colorful[1], "PROVIDER COST") {
		t.Fatalf("wide detail should use two useful field cells: %#v", colorful)
	}
	if strings.Index(joined, "INPUT TOKENS") > strings.Index(joined, "PRICING STATUS") {
		t.Fatalf("primary fields should precede secondary fields:\n%s", joined)
	}
	for _, line := range colorful {
		if got := statsVisibleWidth(line); got > 80 {
			t.Fatalf("detail line width = %d, want <= 80: %q", got, line)
		}
	}

	plain := strings.Join(renderTerminalDetailModel(detail, 80, usageTextPrimitives{}), "\n")
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("no-color detail contains ANSI: %q", plain)
	}
	for _, want := range []string{"DETAIL · 模型😀", "INPUT TOKENS", "1,234", "$12.340000000", "partial", "FAILED COST TOKEN", "WARNING"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("no-color detail missing %q:\n%s", want, plain)
		}
	}
	if stripped := stripStatsANSI(joined); stripped != plain {
		t.Fatalf("color and no-color semantic frames differ:\ncolor stripped:\n%s\nplain:\n%s", stripped, plain)
	}
	if strings.Count(plain, "Z") != len(longToken) || strings.Count(plain, "内") != 20 || strings.Count(plain, "容") != 20 || strings.Count(plain, "🧭") != 20 || strings.Count(plain, "e\u0301") != 20 || strings.Contains(plain, "…") {
		t.Fatalf("detail wrapping lost visible content:\n%s", plain)
	}

	narrow := renderTerminalDetailModel(detail, 48, usageTextPrimitives{})
	for _, line := range narrow {
		if got := statsVisibleWidth(line); got > 48 {
			t.Fatalf("narrow detail line width = %d, want <= 48: %q", got, line)
		}
	}
}

func TestRenderTerminalDetailOmitsEmptyRegion(t *testing.T) {
	detail := terminalDetailModel{
		title:  "EMPTY",
		fields: []terminalDetailField{{label: "LABEL", value: "   ", role: terminalDetailRoleToken}},
		notes:  []terminalDetailNote{{text: "\n\t"}},
	}
	if lines := renderTerminalDetailModel(detail, 80, usageTextPrimitives{color: true}); lines != nil {
		t.Fatalf("empty detail = %v, want nil", lines)
	}
	lines := renderTerminalDetailModel(terminalDetailModel{fields: []terminalDetailField{{label: "STATUS", value: "available", role: terminalDetailRoleSuccess}}}, 80, usageTextPrimitives{})
	if len(lines) == 0 || lines[0] != "DETAIL" {
		t.Fatalf("untitled non-empty detail title = %#v", lines)
	}
	zeroAndWarning := strings.Join(renderTerminalDetailModel(terminalDetailModel{
		fields: []terminalDetailField{{label: "CACHE TOKENS", value: "0", role: terminalDetailRoleToken}},
		notes:  []terminalDetailNote{{status: "unpriced", role: terminalDetailRoleWarning}},
	}, 80, usageTextPrimitives{}), "\n")
	for _, want := range []string{"CACHE TOKENS", "0", "UNPRICED"} {
		if !strings.Contains(zeroAndWarning, want) {
			t.Fatalf("semantic zero or status-only warning lost %q:\n%s", want, zeroAndWarning)
		}
	}
}

func TestRenderTerminalDetailDoesNotExposeControls(t *testing.T) {
	plain := strings.Join(renderTerminalDetailModel(terminalDetailModel{
		title: "SAFE\x1b[31m",
		fields: []terminalDetailField{{
			label: "LABEL\nSPOOF",
			value: "value\rhidden",
			role:  terminalDetailRoleNeutral,
		}},
		notes: []terminalDetailNote{{
			status: "warn\x1b[32ming",
			text:   "note\nspoof\rhidden",
			role:   terminalDetailRoleWarning,
		}},
	}, 48, usageTextPrimitives{}), "\n")
	if strings.Contains(plain, "\x1b[") || strings.Contains(plain, "\r") || !strings.Contains(plain, "LABEL SPOOF") || !strings.Contains(plain, "value hidden") || !strings.Contains(plain, "WARN ING · note spoof hidden") {
		t.Fatalf("no-color sanitized detail = %q", plain)
	}
}

func TestRenderTerminalDetailGeometryMatrixPreservesSemantics(t *testing.T) {
	detail := terminalDetailModel{
		title: "宽度😀",
		fields: []terminalDetailField{
			{label: "SESSION", value: "session-0123456789abcdef", role: terminalDetailRoleSession, priority: terminalDetailPriorityPrimary},
			{label: "INPUT TOKENS", value: "1234567890123456789012345678901234567890", role: terminalDetailRoleToken, priority: terminalDetailPriorityPrimary},
			{label: "COST", value: "$12.345678", role: terminalDetailRoleCost, priority: terminalDetailPrioritySecondary},
		},
		notes: []terminalDetailNote{{
			status:   "warning",
			text:     strings.Repeat("内容🧭e\u0301", 8),
			role:     terminalDetailRoleWarning,
			priority: terminalDetailPriorityPrimary,
		}},
	}

	for _, width := range []int{48, 60, 80, 120, 180} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			plainLines := renderTerminalDetailModel(detail, width, usageTextPrimitives{})
			colorLines := renderTerminalDetailModel(detail, width, usageTextPrimitives{color: true})
			plain := strings.Join(plainLines, "\n")
			stripped := stripStatsANSI(strings.Join(colorLines, "\n"))
			if stripped != plain {
				t.Fatalf("color and no-color frames differ at width %d:\ncolor stripped:\n%s\nplain:\n%s", width, stripped, plain)
			}
			for _, line := range plainLines {
				if got := statsVisibleWidth(line); got > width {
					t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
				}
			}
			twoColumnRow := false
			for _, line := range plainLines {
				if strings.Contains(line, "SESSION") && strings.Contains(line, "INPUT TOKENS") {
					twoColumnRow = true
					break
				}
			}
			wantTwoColumns := width >= 80
			if twoColumnRow != wantTwoColumns {
				t.Fatalf("width %d two-column row = %t, want %t:\n%s", width, twoColumnRow, wantTwoColumns, plain)
			}
			for _, want := range []string{"DETAIL · 宽度😀", "SESSION", "session-0123456789abcdef", "INPUT TOKENS", "COST", "$12.345678", "WARNING"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("width %d lost %q:\n%s", width, want, plain)
				}
			}
			compact := strings.NewReplacer("\n", "", " ", "").Replace(plain)
			if !strings.Contains(compact, "1234567890123456789012345678901234567890") {
				t.Fatalf("width %d lost the wrapped token value:\n%s", width, plain)
			}
			if strings.Count(plain, "内") != 8 || strings.Count(plain, "容") != 8 || strings.Count(plain, "🧭") != 8 || strings.Count(plain, "e\u0301") != 8 || strings.Contains(plain, "…") {
				t.Fatalf("width %d lost wrapped note content:\n%s", width, plain)
			}
		})
	}
}
