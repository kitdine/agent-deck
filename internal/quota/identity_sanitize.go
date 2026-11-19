package quota

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	"golang.org/x/text/unicode/norm"
)

func maskedIdentityEmail(raw string) (normalized, local, masked string, ok bool) {
	if !utf8.ValidString(raw) {
		return "", "", "", false
	}
	email := norm.NFC.String(raw)
	for _, r := range email {
		if unicode.IsSpace(r) || unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return "", "", "", false
		}
	}
	local, domain, found := strings.Cut(email, "@")
	if !found || strings.Contains(domain, "@") || !strings.Contains(domain, ".") ||
		strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || utf8.RuneCountInString(domain) > 253 {
		return "", "", "", false
	}
	var first, last string
	count := 0
	g := graphemes.FromString(local)
	for g.Next() {
		last = g.Value()
		if count == 0 {
			first = last
		}
		count++
	}
	if count == 0 || count > 64 {
		return "", "", "", false
	}
	masked = "***"
	if count >= 2 {
		masked = first + masked
	}
	if count >= 4 {
		masked += last
	}
	return email, local, masked + "@" + strings.ToLower(domain), true
}

type identityForbiddenValue struct {
	folded    string
	substring bool
}

type identityForbiddenSet []identityForbiddenValue

var identityEmailShape = regexp.MustCompile(`[^\s@]+@[^\s@]+\.[^\s@]+`)

func foldIdentityString(s string) string {
	var folded strings.Builder
	for _, r := range norm.NFC.String(s) {
		switch {
		case r >= 0x13a0 && r <= 0x13fd, r >= 0xab70 && r <= 0xabbf:
			// Cherokee folds to uppercase in Unicode default folding.
			folded.WriteRune(unicode.ToUpper(r))
		default:
			if replacement, ok := identityFoldOverrides[r]; ok {
				folded.WriteString(replacement)
			} else {
				folded.WriteRune(unicode.ToLower(r))
			}
		}
	}
	return norm.NFC.String(folded.String())
}

func newIdentityForbiddenSet(local string, rawValues ...string) identityForbiddenSet {
	var set identityForbiddenSet
	for _, value := range rawValues {
		if value != "" {
			set = append(set, identityForbiddenValue{foldIdentityString(value), hasIdentityGraphemes(value, 4)})
		}
	}
	if local != "" {
		set = append(set, identityForbiddenValue{foldIdentityString(local), hasIdentityGraphemes(local, 3)})
	}
	return set
}

func hasIdentityGraphemes(value string, count int) bool {
	g := graphemes.FromString(norm.NFC.String(value))
	for i := 0; i < count; i++ {
		if !g.Next() {
			return false
		}
	}
	return true
}

func (set identityForbiddenSet) rejects(value string) bool {
	folded := foldIdentityString(value)
	if identityEmailShape.MatchString(folded) {
		return true
	}
	for _, forbidden := range set {
		if folded == forbidden.folded || (forbidden.substring && strings.Contains(folded, forbidden.folded)) {
			return true
		}
	}
	return false
}

func sanitizeIdentityName(raw string, forbidden identityForbiddenSet) string {
	filtered := strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Co, unicode.Cs) ||
			(r >= 0xFDD0 && r <= 0xFDEF) || r&0xFFFE == 0xFFFE {
			return -1
		}
		return r
	}, norm.NFC.String(raw))
	compact := strings.Join(strings.Fields(filtered), " ")
	g := graphemes.FromString(compact)
	end := 0
	for count := 0; count < 64 && g.Next(); count++ {
		end += len(g.Value())
	}
	value := strings.TrimRightFunc(compact[:end], unicode.IsSpace)
	if forbidden.rejects(value) {
		return ""
	}
	return value
}

func sanitizeIdentityPlan(raw string, forbidden identityForbiddenSet) planField {
	if raw == "" {
		return planField{}
	}
	if !safePlanToken(raw) || forbidden.rejects(raw) {
		return planField{rejected: true}
	}
	return planField{value: raw}
}
