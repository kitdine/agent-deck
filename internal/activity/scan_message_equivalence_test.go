package activity

import (
	"math/rand"
	"strings"
	"testing"
)

// referenceScanMessage is the original per-vocabulary scan ScanMessage must
// stay equivalent to.
func referenceScanMessage(text string) (string, string) {
	if strings.TrimSpace(text) == "" {
		return MessageClassNone, ""
	}
	lowered := strings.ToLower(text)
	best := -1
	class, sub := MessageClassNone, ""
	for _, candidate := range []struct {
		words []string
		class string
		sub   string
	}{
		{faultWords, MessageClassFault, ""},
		{featureWords, MessageClassBuild, SubFeature},
		{refactoringWords, MessageClassBuild, SubRefactoring},
		{testingWords, MessageClassBuild, SubTesting},
	} {
		at := earliestWord(lowered, candidate.words)
		if at < 0 {
			continue
		}
		if best < 0 || at < best {
			best, class, sub = at, candidate.class, candidate.sub
		}
	}
	return class, sub
}

var scanMessageSeeds = []string{
	"", "   ", "fix the bug", "Add tests", "test fix", "address renew", "notworking not working",
	"clean up later", "cleanup", "split_test", "tests", "testing 404", "a404", "4040 404x 404",
	"rename: refactor", "BUILD it", "new_feature then add", "x-fix", "fix-", "spec.", "the KELVIN Kelvin",
	"İmplement", "Kill error", "implementé add", "migrate\nerror", "what if we add caching",
	"not workingfix", "clean  up", "exceptional exception", "extractor extract", "scaffold;crash",
}

func TestScanMessageMatchesReferenceScan(t *testing.T) {
	for _, text := range scanMessageSeeds {
		if gotClass, gotSub := ScanMessage(text); true {
			wantClass, wantSub := referenceScanMessage(text)
			if gotClass != wantClass || gotSub != wantSub {
				t.Fatalf("ScanMessage(%q)=%q/%q want %q/%q", text, gotClass, gotSub, wantClass, wantSub)
			}
		}
	}
	// Random texts built from vocabulary words, fragments and separators.
	var pieces []string
	for _, vocabulary := range [][]string{faultWords, featureWords, refactoringWords, testingWords, brainstormWords} {
		for _, word := range vocabulary {
			pieces = append(pieces, word, strings.ToUpper(word), word[:len(word)/2+1], "x"+word, word+"s", word+"_")
		}
	}
	pieces = append(pieces, " ", " ", "\n", "-", "_", ".", "0", "é", "K", "İ", "ab", "4", "no", "up")
	random := rand.New(rand.NewSource(20261022))
	for range 200000 {
		var text strings.Builder
		for range random.Intn(12) {
			text.WriteString(pieces[random.Intn(len(pieces))])
		}
		gotClass, gotSub := ScanMessage(text.String())
		wantClass, wantSub := referenceScanMessage(text.String())
		if gotClass != wantClass || gotSub != wantSub {
			t.Fatalf("ScanMessage(%q)=%q/%q want %q/%q", text.String(), gotClass, gotSub, wantClass, wantSub)
		}
	}
}

func FuzzScanMessageMatchesReferenceScan(f *testing.F) {
	for _, text := range scanMessageSeeds {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		gotClass, gotSub := ScanMessage(text)
		wantClass, wantSub := referenceScanMessage(text)
		if gotClass != wantClass || gotSub != wantSub {
			t.Fatalf("ScanMessage(%q)=%q/%q want %q/%q", text, gotClass, gotSub, wantClass, wantSub)
		}
	})
}
