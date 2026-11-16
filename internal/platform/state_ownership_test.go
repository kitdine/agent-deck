package platform

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pathFixture(t *testing.T) (*pathResolver, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "account")
	if err := os.MkdirAll(filepath.Join(home, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(root, "environment-home"))
	return &pathResolver{accountHome: func() (string, error) { return home, nil }, lstat: os.Lstat, readlink: os.Readlink}, root, home
}

func TestProtectedPathFilesystemAliases(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "link-chain"}[linked], func(t *testing.T) {
			testProtectedPathFilesystemAliases(t, linked)
		})
	}
}

func testProtectedPathFilesystemAliases(t *testing.T, linked bool) {
	r, root, home := pathFixture(t)
	entrance := filepath.Join(home, ".agentdeck")
	if linked {
		if err := os.Mkdir(root+"/moved", 0700); err != nil {
			t.Fatal(err)
		}
		for link, target := range map[string]string{entrance: root + "/middle", root + "/middle": root + "/moved"} {
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
		}
	} else if err := os.Mkdir(entrance, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entrance, "sentinel"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.Stat(entrance)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := os.Stat(filepath.Join(home, ".AGENTDECK"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("fixture volume is case-sensitive; covered separately")
	}
	if err != nil || !os.SameFile(canonical, alias) {
		t.Fatalf("fixture aliases are not the same directory: %v", err)
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	childLookups := 0
	r.lstat = func(path string) (fs.FileInfo, error) {
		// Independent oracle: compare the directory's OS identity, never the
		// implementation's path predicate. Refuse even the attempted child stat.
		parent, err := os.Stat(filepath.Dir(path))
		if err == nil && os.SameFile(parent, canonical) {
			childLookups++
			return nil, errors.New("protected child metadata lookup")
		}
		return os.Lstat(path)
	}
	candidates := []string{
		entrance, entrance + "/sentinel",
		home + "/.AGENTDECK", home + "/.AGENTDECK/sentinel",
		root + "/ACCOUNT/.agentdeck/sentinel", root + "/ACCOUNT/.AGENTDECK/sentinel",
	}
	if linked {
		candidates = append(candidates, root+"/MIDDLE/sentinel", root+"/MOVED/sentinel")
	}
	for _, candidate := range candidates {
		t.Run(strings.TrimPrefix(candidate, root), func(t *testing.T) {
			childLookups = 0
			_, err := r.walk(candidate, protected, nil)
			if !errors.Is(err, ErrProtectedStatePath) {
				t.Errorf("%s: %v", candidate, err)
			}
			if childLookups != 0 {
				t.Errorf("attempted protected child metadata reads: %d", childLookups)
			}
		})
	}
}

func TestMissingProtectedPathAliases(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-root", true: "missing-link-target"}[nested], func(t *testing.T) {
			r, root, home := pathFixture(t)
			r.caseSensitive = func(directory string) (bool, error) {
				// The capability seam represents a case-insensitive volume; the
				// native Darwin query is independently checked in its own test.
				if directory != root && directory != home {
					t.Fatalf("unexpected policy directory %s", directory)
				}
				return false, nil
			}
			candidate := home + "/.AGENTDECK/sentinel"
			if nested {
				if err := os.Symlink(root+"/missing/target", home+"/.agentdeck"); err != nil {
					t.Fatal(err)
				}
				candidate = root + "/MISSING/TARGET/sentinel"
			}
			protected, err := r.protectedPaths()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.walk(candidate, protected, nil); !errors.Is(err, ErrProtectedStatePath) {
				t.Fatalf("%s: %v", candidate, err)
			}
			r.caseSensitive = func(string) (bool, error) { return false, fs.ErrPermission }
			if _, err := r.walk(candidate, protected, nil); !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("policy failure must refuse: %v", err)
			}
		})
	}
}

func TestMissingTargetNamePoliciesAreIndependent(t *testing.T) {
	for _, policy := range []struct {
		name                                  string
		caseSensitive, normalizationSensitive bool
	}{
		{"insensitive-both", false, false}, {"case-sensitive", true, false},
		{"normalization-sensitive", false, true}, {"sensitive-both", true, true},
	} {
		for _, names := range []struct {
			name, target, candidate         string
			caseChange, normalizationChange bool
		}{
			{"canonical", "caf\u00e9", "cafe\u0301", false, true},
			{"canonical-reverse", "cafe\u0301", "caf\u00e9", false, true},
			{"case-and-canonical", "CAF\u00c9", "cafe\u0301", true, true},
			{"case-only", "caf\u00e9", "CAF\u00c9", true, false},
			{"canonical-casefold-overlap", "\u212b", "\u00c5", false, true},
		} {
			t.Run(policy.name+"/"+names.name, func(t *testing.T) {
				r, root, home := pathFixture(t)
				r.caseSensitive = func(string) (bool, error) { return policy.caseSensitive, nil }
				r.normalizationSensitive = func(string) (bool, error) { return policy.normalizationSensitive, nil }
				target := filepath.Join(root, names.target)
				if err := os.Symlink(target, home+"/.agentdeck"); err != nil {
					t.Fatal(err)
				}
				protected, err := r.protectedPaths()
				if err != nil {
					t.Fatal(err)
				}
				candidate := filepath.Join(root, names.candidate, "sentinel")
				got, err := r.walk(candidate, protected, nil)
				matches := (!names.caseChange || !policy.caseSensitive) && (!names.normalizationChange || !policy.normalizationSensitive)
				// Unicode simple case folding also equates Angstrom and A-ring:
				// an insensitive case policy alone can establish that match.
				if names.name == "canonical-casefold-overlap" && !policy.caseSensitive {
					matches = true
				}
				if matches {
					if !errors.Is(err, ErrProtectedStatePath) {
						t.Fatalf("got=%q err=%v", got, err)
					}
				} else if err != nil || got != candidate {
					t.Fatalf("distinct name got=%q err=%v want=%q", got, err, candidate)
				}
				if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("walk created protected target: %v", err)
				}
			})
		}
	}
}

func TestUnknownNormalizationPolicyRejectsOnlyAmbiguousNames(t *testing.T) {
	r, root, home := pathFixture(t)
	target := root + "/caf\u00e9/protected"
	if err := os.Symlink(target, home+"/.agentdeck"); err != nil {
		t.Fatal(err)
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	queries := 0
	r.normalizationSensitive = func(directory string) (bool, error) {
		queries++
		if directory != root {
			t.Fatalf("policy queried on %s", directory)
		}
		return false, fs.ErrPermission
	}
	r.caseSensitive = func(string) (bool, error) {
		t.Fatal("canonical-only equality does not need case policy")
		return false, nil
	}
	if _, err := r.walk(root+"/cafe\u0301/protected/sentinel", protected, nil); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("unknown rule accepted ambiguity: %v", err)
	}
	if queries != 1 {
		t.Fatalf("normalization queries=%d", queries)
	}
	queries = 0
	for _, candidate := range []string{root + "/unrelated/sentinel", root + "/cafe\u0301/different/sentinel"} {
		if got, err := r.walk(candidate, protected, nil); err != nil || got != candidate {
			t.Fatalf("unrelated path=%q err=%v", got, err)
		}
	}
	if queries != 0 {
		t.Fatalf("unrelated paths queried normalization %d times", queries)
	}
}

func TestCanonicalNameKeyPreservesInputBoundaries(t *testing.T) {
	acuteRun := strings.Repeat("\u0301", 30)
	for _, tc := range []struct{ name, input, want string }{
		{"long-run", "a" + acuteRun + "\u0323", "a\u0323" + acuteRun},
		{"literal-cgj-boundary", "a" + acuteRun + "\u034F\u0323", "a" + acuteRun + "\u034F\u0323"},
		{"literal-cgj-and-long-run", "z\u034Fa" + acuteRun + "\u0323", "z\u034Fa\u0323" + acuteRun},
		{"leading-marks", "\u0301\u0323a", "\u0323\u0301a"},
		{"equal-class-stability", "a\u0301\u0300", "a\u0301\u0300"},
		{"equal-class-reverse", "a\u0300\u0301", "a\u0300\u0301"},
		{"hangul", "\uAC01", "\u1100\u1161\u11A8"},
		{"decomposed-rune-at-long-boundary", "\u00E1" + strings.Repeat("\u0301", 29) + "\u0323", "a\u0323" + acuteRun},
		{"invalid-byte-boundary", "a\u0301\xff\u0323", "a\u0301\xff\u0323"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalNameKey(tc.input)
			if got != tc.want {
				t.Fatalf("key=%q want=%q", got, tc.want)
			}
			if strings.Count(got, "\u034F") != strings.Count(tc.input, "\u034F") {
				t.Fatal("changed literal grapheme joiner count")
			}
		})
	}
}

func TestLongCanonicalNamePolicyAndLiteralJoiner(t *testing.T) {
	a := "a" + strings.Repeat("\u0301", 30) + "\u0323"
	b := "a\u0323" + strings.Repeat("\u0301", 30)
	for _, sensitive := range []bool{false, true} {
		r, root, home := pathFixture(t)
		r.normalizationSensitive = func(string) (bool, error) { return sensitive, nil }
		if err := os.Symlink(root+"/"+a, home+"/.agentdeck"); err != nil {
			t.Fatal(err)
		}
		protected, err := r.protectedPaths()
		if err != nil {
			t.Fatal(err)
		}
		candidate := root + "/" + b + "/sentinel"
		got, err := r.walk(candidate, protected, nil)
		if sensitive {
			if err != nil || got != candidate {
				t.Fatalf("sensitive name=%q err=%v", got, err)
			}
		} else if !errors.Is(err, ErrProtectedStatePath) {
			t.Fatalf("canonical alias=%q err=%v", got, err)
		}
		// An actual input CGJ is a canonical-ordering boundary. Removing all
		// joiners would incorrectly equate this name with the protected target.
		literal := root + "/a" + strings.Repeat("\u0301", 30) + "\u034F\u0323/sentinel"
		if got, err := r.walk(literal, protected, nil); err != nil || got != literal {
			t.Fatalf("literal joiner name=%q err=%v", got, err)
		}
	}
}

func TestProtectedAliasIdentityIsFreshForEachWalk(t *testing.T) {
	r, root, home := pathFixture(t)
	entrance := home + "/.agentdeck"
	if err := os.Mkdir(entrance, 0700); err != nil {
		t.Fatal(err)
	}
	canonical, err := os.Stat(entrance)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := os.Stat(home + "/.AGENTDECK")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("fixture volume is case-sensitive")
	}
	if err != nil || !os.SameFile(canonical, alias) {
		t.Fatalf("alias fixture: %v", err)
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.walk(home+"/.AGENTDECK/new", protected, nil); !errors.Is(err, ErrProtectedStatePath) {
		t.Fatal(err)
	}
	if err := os.Rename(entrance, root+"/previous-state"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(entrance, 0700); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(entrance)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(canonical, current) {
		t.Fatal("replacement fixture reused original directory")
	}
	r.lstat = func(path string) (fs.FileInfo, error) {
		parent, err := os.Stat(filepath.Dir(path))
		if err == nil && os.SameFile(parent, current) {
			t.Fatalf("accessed replacement state child: %s", path)
		}
		return os.Lstat(path)
	}
	if _, err := r.walk(home+"/.AGENTDECK/new", protected, nil); !errors.Is(err, ErrProtectedStatePath) {
		t.Fatal(err)
	}
}

func TestCaseSensitiveDistinctDirectoriesRemainAllowed(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-root", true: "existing-root"}[existing], func(t *testing.T) {
			r, root, home := pathFixture(t)
			if existing {
				if err := os.Mkdir(home+"/.agentdeck", 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Model a separate case-sensitive filesystem using real, distinct
			// directory identities. This runs even on an insensitive host volume.
			other := root + "/distinct-directory"
			otherAccount := root + "/distinct-account"
			for _, dir := range []string{other, otherAccount + "/.agentdeck"} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dir+"/sentinel", []byte("allowed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			r.lstat = func(path string) (fs.FileInfo, error) {
				switch {
				case path == home+"/.AGENTDECK" || strings.HasPrefix(path, home+"/.AGENTDECK/"):
					path = other + strings.TrimPrefix(path, home+"/.AGENTDECK")
				case path == root+"/ACCOUNT" || strings.HasPrefix(path, root+"/ACCOUNT/"):
					path = otherAccount + strings.TrimPrefix(path, root+"/ACCOUNT")
				}
				return os.Lstat(path)
			}
			r.caseSensitive = func(string) (bool, error) { return true, nil }
			protected, err := r.protectedPaths()
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				a, _ := r.lstat(home + "/.agentdeck")
				b, _ := r.lstat(home + "/.AGENTDECK")
				if a == nil || b == nil || os.SameFile(a, b) {
					t.Fatal("case-sensitive fixture lacks distinct objects")
				}
			}
			for _, candidate := range []string{home + "/.AGENTDECK", home + "/.AGENTDECK/sentinel", root + "/ACCOUNT/.agentdeck/sentinel"} {
				if got, err := r.walk(candidate, protected, nil); err != nil || got != candidate {
					t.Fatalf("distinct directory %s: got=%s err=%v", candidate, got, err)
				}
			}
			if _, err := r.walk(home+"/.agentdeck/sentinel", protected, nil); !errors.Is(err, ErrProtectedStatePath) {
				t.Fatalf("canonical root: %v", err)
			}
		})
	}
}

func TestPhysicalWalkCounterexamples(t *testing.T) {
	r, root, home := pathFixture(t)
	entrance := filepath.Join(home, ".agentdeck")
	moved := filepath.Join(root, "moved")
	middle := filepath.Join(root, "middle")
	for _, path := range []string{moved, filepath.Join(root, "outside")} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{entrance: middle, middle: moved, filepath.Join(root, "outside/jump"): filepath.Join(home, "inner"), filepath.Join(root, "account-link"): home, filepath.Join(root, "loop-a"): "loop-b", filepath.Join(root, "loop-b"): "loop-a"} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	protected, err := r.protectedPaths()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{entrance, middle, moved} {
		if !slicesContain(protected, path) {
			t.Fatalf("missing protected entrance %s in %v", path, protected)
		}
	}
	jump := root + "/outside/jump/../.agentdeck/sentinel"
	cases := []struct {
		name, path, want    string
		refused, unresolved bool
	}{
		{"entrance", entrance, "", true, false}, {"target", moved, "", true, false}, {"chain", middle, "", true, false},
		{"entrance-child", entrance + "/sentinel", "", true, false}, {"target-child", moved + "/sentinel", "", true, false},
		{"link-parent", jump, "", true, false}, {"ancestor-link", root + "/account-link/.agentdeck/sentinel", "", true, false},
		{"missing-parent", root + "/missing/../safe", "", false, true}, {"loop", root + "/loop-a/child", "", false, true},
		{"ancestor-allowed", home, home, false, false}, {"sibling-allowed", home + "/.agentdeck-other", home + "/.agentdeck-other", false, false},
		{"physical-return", root + "/account-link/new", home + "/new", false, false},
		{"root-parent", root + "/outside/..", root, false, false},
	}
	// A counted seam proves ordering at the primitive, not OS sandbox acceptance.
	targetInfo, err := os.Stat(moved)
	if err != nil {
		t.Fatal(err)
	}
	r.lstat = func(path string) (fs.FileInfo, error) {
		parent, err := os.Stat(filepath.Dir(path))
		if err == nil && os.SameFile(parent, targetInfo) {
			t.Fatalf("protected child metadata lookup: %s", path)
		}
		return os.Lstat(path)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.walk(tc.path, protected, nil)
			if tc.refused {
				if !errors.Is(err, ErrProtectedStatePath) {
					t.Fatalf("got=%s err=%v", got, err)
				}
			} else if tc.unresolved {
				if err == nil || errors.Is(err, ErrProtectedStatePath) {
					t.Fatalf("got=%s err=%v", got, err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got=%s err=%v want=%s", got, err, tc.want)
			}
		})
	}
	cleaned := filepath.Clean(jump)
	if got, err := r.walk(cleaned, protected, nil); err != nil || protectedPath(got, protected) {
		t.Fatalf("Clean counterexample no longer demonstrates escape: %s %v", got, err)
	}
}

func TestProtectedPathsMissingAndFailures(t *testing.T) {
	t.Run("missing-root", func(t *testing.T) {
		r, _, home := pathFixture(t)
		p, err := r.protectedPaths()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.walk(home+"/.agentdeck/new", p, nil); !errors.Is(err, ErrProtectedStatePath) {
			t.Fatal(err)
		}
	})
	t.Run("account-ancestor-link", func(t *testing.T) {
		r, root, home := pathFixture(t)
		alias := root + "/account-alias"
		if err := os.Symlink(home, alias); err != nil {
			t.Fatal(err)
		}
		r.accountHome = func() (string, error) { return alias, nil }
		p, err := r.protectedPaths()
		if err != nil || !slicesContain(p, home+"/.agentdeck") {
			t.Fatalf("%v %v", p, err)
		}
	})
	for _, kind := range []string{"account-failure", "entrance-permission", "entrance-loop"} {
		t.Run(kind, func(t *testing.T) {
			r, _, home := pathFixture(t)
			switch kind {
			case "account-failure":
				r.accountHome = func() (string, error) { return "", fs.ErrPermission }
			case "entrance-permission":
				r.lstat = func(path string) (fs.FileInfo, error) {
					if path == home+"/.agentdeck" {
						return nil, fs.ErrPermission
					}
					return os.Lstat(path)
				}
			case "entrance-loop":
				if err := os.Symlink(".agentdeck", home+"/.agentdeck"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.protectedPaths(); err == nil || err.Error() != "real_root_unresolved" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	t.Run("candidate-permission", func(t *testing.T) {
		r, root, _ := pathFixture(t)
		r.lstat = func(path string) (fs.FileInfo, error) {
			if path == root+"/denied" {
				return nil, fs.ErrPermission
			}
			return os.Lstat(path)
		}
		if _, err := r.walk(root+"/denied", nil, nil); !errors.Is(err, fs.ErrPermission) {
			t.Fatal(err)
		}
	})
}

type fixedInstallation struct {
	identity InstallationIdentity
	home     string
}

func (f *fixedInstallation) Resolve() InstallationIdentity { return f.identity }
func (f *fixedInstallation) AccountHome() (string, error)  { return f.home, nil }

func TestInstallationResolverDefaultsAndFreshObservation(t *testing.T) {
	SetInstallationResolver(nil)
	t.Cleanup(func() { SetInstallationResolver(nil) })
	if got := CurrentInstallation(); got.Installed || got.Reason != "receipt_missing" {
		t.Fatalf("default=%+v", got)
	}
	resolver := &fixedInstallation{InstallationIdentity{Installed: true, Channel: "make"}, t.TempDir()}
	SetInstallationResolver(resolver)
	if !CurrentInstallation().Installed {
		t.Fatal("injection not used")
	}
	resolver.identity = InstallationIdentity{Reason: "receipt_other_executable"}
	if CurrentInstallation().Installed {
		t.Fatal("identity cached")
	}
	if home, err := AccountHome(); err != nil || home != resolver.home {
		t.Fatalf("home=%s err=%v", home, err)
	}
}

func TestRealAccountHomeIgnoresEnvironment(t *testing.T) {
	if os.Getenv("AGENTDECK_ACCOUNT_PROBE") == "1" {
		home, err := RealAccountHome()
		if err != nil {
			t.Fatal(err)
		}
		if home != os.Getenv("AGENTDECK_EXPECTED_ACCOUNT") {
			t.Fatalf("home=%q", home)
		}
		return
	}
	want, err := RealAccountHome()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRealAccountHomeIgnoresEnvironment$")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "HOME=") && !strings.HasPrefix(value, "USER=") && !strings.HasPrefix(value, "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	fake := t.TempDir()
	cmd.Env = append(cmd.Env, "HOME="+fake, "USER=not-the-account", "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="+fake, "AGENTDECK_ACCOUNT_PROBE=1", "AGENTDECK_EXPECTED_ACCOUNT="+want)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("account probe: %s: %v", out, err)
	}
}
