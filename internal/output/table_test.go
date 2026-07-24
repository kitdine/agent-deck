package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestWriteASCIITable(t *testing.T) {
	var output bytes.Buffer
	err := WriteASCIITable(&output, []string{"NAME", "READY"}, [][]string{
		{"official", "true"},
		{"自定义", "false"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "" +
		"+----------+-------+\n" +
		"| NAME     | READY |\n" +
		"+----------+-------+\n" +
		"| official | true  |\n" +
		"+----------+-------+\n" +
		"| 自定义   | false |\n" +
		"+----------+-------+\n"
	if output.String() != want {
		t.Fatalf("table output:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestWriteASCIITableNormalizesMultilineCells(t *testing.T) {
	var output bytes.Buffer
	if err := WriteASCIITable(&output, []string{"TEXT"}, [][]string{{"first\r\nsecond\tvalue"}}); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(strings.TrimSuffix(output.String(), "\n"), "\r\t") || strings.Count(output.String(), "\n") != 5 {
		t.Fatalf("normalized table = %q", output.String())
	}
}

func TestWriteASCIITableSanitizesTerminalControlSequences(t *testing.T) {
	var output bytes.Buffer
	value := "safe\x1b[31mred\x1b[0m\x1b]0;forged-title\x07end\b!\x7f\u009b31mC1"
	if err := WriteASCIITable(&output, []string{"TEXT"}, [][]string{{value}}); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, forbidden := range []string{"\x1b", "[31m", "forged-title", "\a", "\b", "\x7f", "\u009b"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("table retained terminal control content %q: %q", forbidden, rendered)
		}
	}
	for _, value := range []string{"safe", "red", "end", "C1"} {
		if !strings.Contains(rendered, value) {
			t.Fatalf("table removed visible content %q: %q", value, rendered)
		}
	}
	for _, r := range rendered {
		if r != '\n' && (r < 0x20 || r >= 0x7f && r <= 0x9f) {
			t.Fatalf("table retained control rune U+%04X: %q", r, rendered)
		}
	}
}

func TestWriteASCIITableSanitizesSupportedCSIAndOSCForms(t *testing.T) {
	var output bytes.Buffer
	value := "start\x1b[31mred\x1b[0m/\u009b32mgreen\u009b0m/" +
		"\x1b]0;bel-payload\x07after-bel/" +
		"\x1b]0;esc-payload\x1b\\after-esc/" +
		"\u009d0;c1-payload\u009cafter-c1"
	if err := WriteASCIITable(&output, []string{"TEXT"}, [][]string{{value}}); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	if want := "start red / green / after-bel/ after-esc/ after-c1"; !strings.Contains(rendered, want) {
		t.Fatalf("sanitized table missing visible text %q: %q", want, rendered)
	}
	for _, forbidden := range []string{
		"\x1b", "\u009b", "\u009d", "\u009c", "[31m", "[0m", "32m",
		"bel-payload", "esc-payload", "c1-payload",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("table retained terminal control content %q: %q", forbidden, rendered)
		}
	}
	assertTableHasNoControlRunes(t, rendered)
}

func TestWriteASCIITableTruncatedControlsPreserveUnicodeAlignment(t *testing.T) {
	var output bytes.Buffer
	if err := WriteASCIITable(&output, []string{"TEXT"}, [][]string{
		{"界🙂e\u0301\x1b"},
		{"短🙂\x1b[31"},
		{"e\u0301界\x1b]0;truncated-osc-payload"},
	}); err != nil {
		t.Fatal(err)
	}

	rendered := output.String()
	for _, visible := range []string{"界🙂e\u0301", "短🙂", "e\u0301界"} {
		if !strings.Contains(rendered, visible) {
			t.Fatalf("truncated control removed visible text %q: %q", visible, rendered)
		}
	}
	for _, forbidden := range []string{"\x1b", "[31", "]0;", "truncated-osc-payload"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("truncated control content leaked %q: %q", forbidden, rendered)
		}
	}
	assertTableHasNoControlRunes(t, rendered)

	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) != 9 {
		t.Fatalf("truncated controls produced %d table lines, want 9: %q", len(lines), rendered)
	}
	wantWidth := runewidth.StringWidth(lines[0])
	for index, line := range lines {
		if got := runewidth.StringWidth(line); got != wantWidth {
			t.Fatalf("line %d display width = %d, want %d: %q\n%s", index, got, wantWidth, line, rendered)
		}
	}
}

func assertTableHasNoControlRunes(t *testing.T, rendered string) {
	t.Helper()
	for _, r := range rendered {
		if r != '\n' && (r < 0x20 || r >= 0x7f && r <= 0x9f) {
			t.Fatalf("table retained control rune U+%04X: %q", r, rendered)
		}
	}
}

func TestWriteASCIITableRejectsInvalidShape(t *testing.T) {
	for _, test := range []struct {
		name    string
		headers []string
		rows    [][]string
	}{
		{name: "no columns"},
		{name: "row mismatch", headers: []string{"ONE"}, rows: [][]string{{"one", "two"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := WriteASCIITable(&bytes.Buffer{}, test.headers, test.rows); err == nil {
				t.Fatal("WriteASCIITable succeeded")
			}
		})
	}
}
