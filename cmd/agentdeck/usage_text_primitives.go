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

type usageAlignedColumn struct {
	label string
	value string
	width int
}

// usageAlignColumnRows gives every visible row in one section the same value
// width per column. Values stay intact; usageAlignedColumns moves a complete
// field onto the next line when that shared profile no longer fits.
func usageAlignColumnRows(rows [][]usageAlignedColumn) {
	widths := make([]int, 0)
	for _, row := range rows {
		for index, column := range row {
			if index == len(widths) {
				widths = append(widths, column.width)
			}
			widths[index] = max(widths[index], column.width, statsVisibleWidth(column.value))
		}
	}
	for _, row := range rows {
		for index := range row {
			row[index].width = widths[index]
		}
	}
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

// usageAlignedColumns keeps values in fixed-width fields and moves whole
// columns to a continuation line when the terminal cannot fit them together.
func usageAlignedColumns(width int, columns ...usageAlignedColumn) []string {
	lines := make([]string, 0, len(columns))
	line := ""
	for _, column := range columns {
		valueWidth := max(column.width, statsVisibleWidth(column.value))
		field := column.label + " " + statsPadLeft(column.value, valueWidth)
		candidate := field
		if line != "" {
			candidate = line + "  " + field
		}
		if line != "" && statsVisibleWidth(candidate) > width {
			lines = append(lines, line)
			line = field
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
