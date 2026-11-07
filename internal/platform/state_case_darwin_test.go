//go:build darwin

package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemCasePolicyMatchesNativeLookup(t *testing.T) {
	r, root, home := pathFixture(t)
	if err := os.Mkdir(root+"/case-probe", 0700); err != nil {
		t.Fatal(err)
	}
	lower, err := os.Stat(root + "/case-probe")
	if err != nil {
		t.Fatal(err)
	}
	upper, upperErr := os.Stat(root + "/CASE-PROBE")
	if upperErr != nil && !errors.Is(upperErr, fs.ErrNotExist) {
		t.Fatal(upperErr)
	}
	sensitive, err := filesystemCaseSensitive(root)
	if err != nil {
		t.Fatal(err)
	}
	if sensitive != errors.Is(upperErr, fs.ErrNotExist) {
		t.Fatalf("pathconf=%v actual lookup=%v", sensitive, upperErr)
	}
	if !sensitive && !os.SameFile(lower, upper) {
		t.Fatal("native aliases do not share object identity")
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.walk(home+"/.AGENTDECK/new", protected, nil)
	if sensitive {
		if err != nil || got != home+"/.AGENTDECK/new" {
			t.Fatalf("sensitive missing path=%s %v", got, err)
		}
	} else if !errors.Is(err, ErrProtectedStatePath) {
		t.Fatalf("insensitive missing path=%s %v", got, err)
	}
}

func TestMissingProtectedTargetCanonicalAliases(t *testing.T) {
	longA := "a" + strings.Repeat("\u0301", 30) + "\u0323"
	longB := "a\u0323" + strings.Repeat("\u0301", 30)
	for _, names := range [][2]string{
		{"caf\u00e9", "cafe\u0301"}, {"cafe\u0301", "caf\u00e9"}, {"caf\u00e9/na\u00efve", "cafe\u0301/nai\u0308ve"},
		{longA, longB}, {longB, longA}, {"z\u034F" + longA, "z\u034F" + longB},
	} {
		t.Run(names[0], func(t *testing.T) {
			r, root, home := pathFixture(t)
			// Prove the filesystem's equivalence on a separate control, while
			// both spellings of the protected target are still absent.
			control := filepath.Join(root, "control")
			if err := os.MkdirAll(filepath.Join(control, names[0]), 0700); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(filepath.Join(control, names[0]))
			if err != nil {
				t.Fatal(err)
			}
			alias, err := os.Stat(filepath.Join(control, names[1]))
			if errors.Is(err, fs.ErrNotExist) {
				t.Skip("native volume distinguishes canonical spellings; covered by policy model")
			}
			if err != nil || !os.SameFile(original, alias) {
				t.Fatalf("native canonical-equivalence control: %v", err)
			}
			target := filepath.Join(root, names[0])
			aliasTarget := filepath.Join(root, names[1])
			if err := os.Symlink(target, home+"/.agentdeck"); err != nil {
				t.Fatal(err)
			}
			protected, err := r.protectedPaths()
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.walk(aliasTarget+"/sentinel", protected, nil)
			if err == nil {
				// Failure-only creation is confined to this fixture and proves
				// that the accepted path would create the protected target.
				if createErr := os.MkdirAll(got, 0700); createErr != nil {
					t.Fatal(createErr)
				}
				a, aErr := os.Stat(target)
				b, bErr := os.Stat(aliasTarget)
				t.Fatalf("accepted canonical alias %q; created protected target=%v", got, aErr == nil && bErr == nil && os.SameFile(a, b))
			}
			if !errors.Is(err, ErrProtectedStatePath) {
				t.Fatalf("canonical alias: %v", err)
			}
			for _, path := range []string{target, aliasTarget} {
				if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("refusal created %q: %v", path, err)
				}
			}
		})
	}
}

func TestNativeLiteralGraphemeJoinerRemainsDistinct(t *testing.T) {
	r, root, home := pathFixture(t)
	plain := "a" + strings.Repeat("\u0301", 30) + "\u0323"
	literal := "a" + strings.Repeat("\u0301", 30) + "\u034F\u0323"
	control := filepath.Join(root, "control")
	if err := os.MkdirAll(filepath.Join(control, plain), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(control, literal), 0700); err != nil {
		t.Fatalf("native literal CGJ name must remain distinct: %v", err)
	}
	a, err := os.Stat(filepath.Join(control, plain))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(filepath.Join(control, literal))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(a, b) {
		t.Fatal("native literal CGJ control aliases the plain name")
	}
	target := filepath.Join(root, plain)
	if err := os.Symlink(target, home+"/.agentdeck"); err != nil {
		t.Fatal(err)
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(root, literal, "sentinel")
	if got, err := r.walk(candidate, protected, nil); err != nil || got != candidate {
		t.Fatalf("distinct native name=%q err=%v", got, err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("created protected target: %v", err)
	}
}
