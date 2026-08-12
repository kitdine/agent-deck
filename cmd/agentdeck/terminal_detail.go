package main

import (
	"sort"
	"strings"

	"github.com/clipperhouse/uax29/v2/graphemes"
	terminaloutput "github.com/kitdine/agent-deck/internal/output"
	"github.com/mattn/go-runewidth"
)

type terminalDetailRole uint8

const (
	terminalDetailRoleNeutral terminalDetailRole = iota
	terminalDetailRoleToken
	terminalDetailRoleCost
	terminalDetailRoleSession
	terminalDetailRoleSuccess
	terminalDetailRoleWarning
	terminalDetailRoleError
)

const (
	terminalDetailPriorityPrimary = iota
	terminalDetailPrioritySecondary
	terminalDetailPriorityTertiary
)

type terminalDetailField struct {
	label    string
	value    string
	role     terminalDetailRole
	priority int
}

type terminalDetailNote struct {
	text     string
	status   string
	role     terminalDetailRole
	priority int
}

type terminalDetailModel struct {
	title  string
	fields []terminalDetailField
	notes  []terminalDetailNote
}

func renderTerminalDetailModel(detail terminalDetailModel, width int, p usageTextPrimitives) []string {
	width = max(1, width)
	detail = normalizeTerminalDetail(detail)
	if len(detail.fields) == 0 && len(detail.notes) == 0 {
		return nil
	}
	lines := renderTerminalDetailTitle(detail.title, width, p)
	if terminalDetailUsesTwoColumns(detail.fields, width) {
		lines = append(lines, renderTerminalDetailTwoColumns(detail.fields, width, p)...)
	} else {
		lines = append(lines, renderTerminalDetailOneColumn(detail.fields, width, p)...)
	}
	for _, note := range detail.notes {
		text := note.text
		if note.status != "" {
			if text == "" {
				text = note.status
			} else {
				text = note.status + " · " + text
			}
		}
		for _, line := range terminalDetailWrap(text, max(1, width-2)) {
			lines = append(lines, statsFit("  "+p.style(line, terminalDetailRoleColor(note.role)), width))
		}
	}
	return lines
}

func renderTerminalDetailTitle(title string, width int, p usageTextPrimitives) []string {
	const prefix = "DETAIL · "
	if title == "" {
		return []string{p.style(statsFit("DETAIL", width), usageColorBrand)}
	}

	prefixWidth := statsVisibleWidth(prefix)
	if width <= prefixWidth {
		wrapped := terminalDetailWrap(prefix+title, width)
		lines := make([]string, 0, len(wrapped))
		for _, line := range wrapped {
			lines = append(lines, p.style(line, usageColorBrand))
		}
		return lines
	}

	wrapped := terminalDetailWrap(title, width-prefixWidth)
	lines := make([]string, 0, len(wrapped))
	for index, line := range wrapped {
		if index == 0 {
			lines = append(lines, p.style(prefix+line, usageColorBrand))
			continue
		}
		lines = append(lines, p.style(strings.Repeat(" ", prefixWidth)+line, usageColorBrand))
	}
	return lines
}

func normalizeTerminalDetail(detail terminalDetailModel) terminalDetailModel {
	detail.title = strings.TrimSpace(terminaloutput.SanitizeTerminalCell(detail.title))
	fields := make([]terminalDetailField, 0, len(detail.fields))
	for _, field := range detail.fields {
		field.label = strings.ToUpper(strings.TrimSpace(terminaloutput.SanitizeTerminalCell(field.label)))
		field.value = strings.TrimSpace(terminaloutput.SanitizeTerminalCell(field.value))
		if field.label == "" || field.value == "" {
			continue
		}
		fields = append(fields, field)
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].priority < fields[j].priority })
	detail.fields = fields

	notes := make([]terminalDetailNote, 0, len(detail.notes))
	for _, note := range detail.notes {
		note.text = strings.TrimSpace(terminaloutput.SanitizeTerminalCell(note.text))
		note.status = strings.ToUpper(strings.TrimSpace(terminaloutput.SanitizeTerminalCell(note.status)))
		if note.text == "" && note.status == "" {
			continue
		}
		notes = append(notes, note)
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].priority < notes[j].priority })
	detail.notes = notes
	return detail
}

func terminalDetailUsesTwoColumns(fields []terminalDetailField, width int) bool {
	if len(fields) < 2 || width < 65 {
		return false
	}
	cellWidth := (width - 1) / 2
	if cellWidth < 28 {
		return false
	}
	for _, field := range fields {
		if statsVisibleWidth(field.label) > cellWidth-12 {
			return false
		}
	}
	return true
}

func renderTerminalDetailOneColumn(fields []terminalDetailField, width int, p usageTextPrimitives) []string {
	labelWidth := terminalDetailLabelWidth(fields, width)
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, renderTerminalDetailField(field, width, labelWidth, p)...)
	}
	return lines
}

func renderTerminalDetailTwoColumns(fields []terminalDetailField, width int, p usageTextPrimitives) []string {
	leftWidth := (width - 1) / 2
	rightWidth := width - leftWidth - 1
	lines := make([]string, 0, len(fields))
	for index := 0; index < len(fields); index += 2 {
		if index+1 == len(fields) {
			lines = append(lines, renderTerminalDetailOneColumn(fields[index:index+1], width, p)...)
			break
		}
		leftLabelWidth := terminalDetailLabelWidth(fields[index:index+1], leftWidth)
		rightLabelWidth := terminalDetailLabelWidth(fields[index+1:index+2], rightWidth)
		left := renderTerminalDetailField(fields[index], leftWidth, leftLabelWidth, p)
		right := renderTerminalDetailField(fields[index+1], rightWidth, rightLabelWidth, p)
		rowHeight := max(len(left), len(right))
		for row := 0; row < rowHeight; row++ {
			leftLine, rightLine := "", ""
			if row < len(left) {
				leftLine = left[row]
			}
			if row < len(right) {
				rightLine = right[row]
			}
			lines = append(lines, statsFit(statsPad(leftLine, leftWidth)+" "+rightLine, width))
		}
	}
	return lines
}

func terminalDetailLabelWidth(fields []terminalDetailField, width int) int {
	labelWidth := 0
	for _, field := range fields {
		labelWidth = max(labelWidth, statsVisibleWidth(field.label))
	}
	return min(labelWidth, max(1, min(24, width/3)))
}

func renderTerminalDetailField(field terminalDetailField, width, labelWidth int, p usageTextPrimitives) []string {
	if statsVisibleWidth(field.label) > labelWidth {
		lines := make([]string, 0, 3)
		for _, labelLine := range terminalDetailWrap(field.label, max(1, width-2)) {
			lines = append(lines, statsFit("  "+p.style(labelLine, usageColorInfo), width))
		}
		for _, valueLine := range terminalDetailWrap(field.value, max(1, width-4)) {
			lines = append(lines, statsFit("    "+p.style(valueLine, terminalDetailRoleColor(field.role)), width))
		}
		return lines
	}
	valueWidth := max(1, width-labelWidth-4)
	wrapped := terminalDetailWrap(field.value, valueWidth)
	if len(wrapped) == 0 {
		return nil
	}
	prefix := "  " + p.style(statsPad(field.label, labelWidth), usageColorInfo) + "  "
	lines := []string{statsFit(prefix+p.style(wrapped[0], terminalDetailRoleColor(field.role)), width)}
	continuation := strings.Repeat(" ", labelWidth+4)
	for _, line := range wrapped[1:] {
		lines = append(lines, statsFit(continuation+p.style(line, terminalDetailRoleColor(field.role)), width))
	}
	return lines
}

// terminalDetailWrap keeps sanitized user-visible text lossless while fitting
// terminal cells. Whitespace is normalized, but no visible rune is elided.
func terminalDetailWrap(value string, width int) []string {
	width = max(1, width)
	words := strings.Fields(value)
	lines := make([]string, 0, 2)
	current := ""
	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}
	for _, word := range words {
		wordWidth := runewidth.StringWidth(word)
		if wordWidth <= width {
			if current == "" {
				current = word
				continue
			}
			if runewidth.StringWidth(current)+1+wordWidth <= width {
				current += " " + word
				continue
			}
			flush()
			current = word
			continue
		}
		flush()
		parts := terminalDetailHardWrapWord(word, width)
		if len(parts) == 0 {
			continue
		}
		lines = append(lines, parts[:len(parts)-1]...)
		current = parts[len(parts)-1]
	}
	flush()
	return lines
}

func terminalDetailHardWrapWord(word string, width int) []string {
	parts := make([]string, 0, 2)
	var part strings.Builder
	used := 0
	flush := func() {
		if part.Len() == 0 {
			return
		}
		parts = append(parts, part.String())
		part.Reset()
		used = 0
	}
	iterator := graphemes.FromString(word)
	for iterator.Next() {
		value := iterator.Value()
		cellWidth := max(0, runewidth.StringWidth(value))
		if used > 0 && used+cellWidth > width {
			flush()
		}
		part.WriteString(value)
		used += cellWidth
	}
	flush()
	return parts
}

func terminalDetailRoleColor(role terminalDetailRole) string {
	switch role {
	case terminalDetailRoleToken:
		return usageColorToken
	case terminalDetailRoleCost:
		return usageColorWarning
	case terminalDetailRoleSession:
		return usageColorSession
	case terminalDetailRoleSuccess:
		return usageColorSuccess
	case terminalDetailRoleWarning:
		return usageColorWarning
	case terminalDetailRoleError:
		return usageColorError
	default:
		return usageColorInfo
	}
}
