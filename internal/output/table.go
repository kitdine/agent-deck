package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// WriteASCIITable renders a collection as a deterministic, bordered text grid.
// Cells are single-line, left-aligned, and measured by terminal display width.
func WriteASCIITable(w io.Writer, headers []string, rows [][]string) error {
	if len(headers) == 0 {
		return fmt.Errorf("table requires at least one column")
	}

	widths := make([]int, len(headers))
	headers = normalizeTableRow(headers)
	for i, value := range headers {
		widths[i] = runewidth.StringWidth(value)
	}

	normalizedRows := make([][]string, len(rows))
	for rowIndex, row := range rows {
		if len(row) != len(headers) {
			return fmt.Errorf("table row %d has %d columns; want %d", rowIndex, len(row), len(headers))
		}
		normalizedRows[rowIndex] = normalizeTableRow(row)
		for column, value := range normalizedRows[rowIndex] {
			widths[column] = max(widths[column], runewidth.StringWidth(value))
		}
	}

	var table strings.Builder
	writeTableBorder(&table, widths)
	writeTableRow(&table, headers, widths)
	writeTableBorder(&table, widths)
	for _, row := range normalizedRows {
		writeTableRow(&table, row, widths)
		writeTableBorder(&table, widths)
	}
	_, err := io.WriteString(w, table.String())
	return err
}

func normalizeTableRow(values []string) []string {
	normalized := make([]string, len(values))
	for i, value := range values {
		normalized[i] = sanitizeTableCell(value)
	}
	return normalized
}

func sanitizeTableCell(value string) string {
	var sanitized strings.Builder
	sanitized.Grow(len(value))
	for index := 0; index < len(value); {
		if value[index] == 0x1b {
			sanitized.WriteByte(' ')
			index = skipANSIEscape(value, index)
			continue
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		switch {
		case r == 0x9b:
			sanitized.WriteByte(' ')
			index = skipCSI(value, index+size)
		case r == 0x9d:
			sanitized.WriteByte(' ')
			index = skipOSC(value, index+size)
		case r < 0x20 || r >= 0x7f && r <= 0x9f:
			sanitized.WriteByte(' ')
			index += size
		case r == utf8.RuneError && size == 1:
			sanitized.WriteRune(utf8.RuneError)
			index++
		default:
			sanitized.WriteString(value[index : index+size])
			index += size
		}
	}
	return sanitized.String()
}

func skipANSIEscape(value string, index int) int {
	if index+1 >= len(value) {
		return index + 1
	}
	switch value[index+1] {
	case '[':
		return skipCSI(value, index+2)
	case ']':
		return skipOSC(value, index+2)
	default:
		return index + 1
	}
}

func skipCSI(value string, index int) int {
	for index < len(value) {
		if value[index] >= 0x40 && value[index] <= 0x7e {
			return index + 1
		}
		index++
	}
	return len(value)
}

func skipOSC(value string, index int) int {
	for index < len(value) {
		switch value[index] {
		case 0x07:
			return index + 1
		case 0x1b:
			if index+1 < len(value) && value[index+1] == '\\' {
				return index + 2
			}
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		if r == 0x9c {
			return index + size
		}
		index += size
	}
	return len(value)
}

func writeTableBorder(table *strings.Builder, widths []int) {
	table.WriteByte('+')
	for _, width := range widths {
		table.WriteString(strings.Repeat("-", width+2))
		table.WriteByte('+')
	}
	table.WriteByte('\n')
}

func writeTableRow(table *strings.Builder, values []string, widths []int) {
	table.WriteByte('|')
	for i, value := range values {
		table.WriteByte(' ')
		table.WriteString(value)
		table.WriteString(strings.Repeat(" ", widths[i]-runewidth.StringWidth(value)))
		table.WriteString(" |")
	}
	table.WriteByte('\n')
}
