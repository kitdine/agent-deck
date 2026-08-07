package main

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// usageTextPrimitives is the shared terminal presentation base for usage
// report commands. It keeps terminal detection and ANSI decisions separate
// from report-specific data layout.
type usageTextPrimitives struct {
	width int
	color bool
}

func newUsageTextPrimitives(w io.Writer, noColor bool) usageTextPrimitives {
	width := statsDefaultWidth
	terminal := false
	if file, ok := w.(*os.File); ok {
		terminal = term.IsTerminal(int(file.Fd()))
		if terminal {
			if columns, _, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
				width = columns
			}
		}
	}
	if raw := os.Getenv("COLUMNS"); raw != "" {
		if columns, err := strconv.Atoi(raw); err == nil && columns > 0 {
			width = columns
		}
	}
	width = min(max(width, statsMinWidth), statsMaxWidth)
	color := terminal && !noColor && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	return usageTextPrimitives{width: width, color: color}
}

func (p usageTextPrimitives) sectionTitle(label string, width int, color string) string {
	plain := label + " "
	return p.style(label, color) + " " + strings.Repeat("─", max(0, width-runewidth.StringWidth(plain)))
}

func (p usageTextPrimitives) style(value, code string) string {
	if !p.color || value == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func (p usageTextPrimitives) barTrack(filled, width int, color string) string {
	return p.style(strings.Repeat("█", filled), color) + strings.Repeat("░", width-filled)
}

func usageJoinColumns(left []string, leftWidth int, right []string, rightWidth, gap int) []string {
	count := max(len(left), len(right))
	lines := make([]string, 0, count)
	for index := 0; index < count; index++ {
		leftLine, rightLine := "", ""
		if index < len(left) {
			leftLine = left[index]
		}
		if index < len(right) {
			rightLine = right[index]
		}
		lines = append(lines, strings.TrimRight(statsPad(leftLine, leftWidth)+strings.Repeat(" ", gap)+statsFit(rightLine, rightWidth), " "))
	}
	return lines
}

func usageResponsiveTableFits(width, gap, leftMin, rightMin int) bool {
	return width-gap >= leftMin+rightMin
}

func usageResponsiveTableWidths(width, gap, rightMin int) (leftWidth, rightWidth int) {
	inner := width - gap
	rightWidth = max(rightMin, inner*2/5)
	return inner - rightWidth, rightWidth
}
