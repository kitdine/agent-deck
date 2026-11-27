//go:build darwin

package quota

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
	"time"
)

// --- Darwin ACLs on the HOME fallback chain (CQP-R3-F1).
//
// Mode bits alone do not say who may modify a Darwin directory or file: an
// ACL can grant other principals entry replacement (add_file, delete_child,
// ...), content writes, or security changes on a 0755 object. These tests add
// ACLs with /bin/chmod +a to freshly created synthetic paths under t.TempDir
// only, and strip them again (chmod -N) before the temp tree is removed. No
// real home, installed App, or real claude binary is touched.

// addDarwinACL appends one ACL entry to path without following a symlink, and
// removes every ACL entry from path again at cleanup — registered after the
// path's t.TempDir, so it runs before that directory is deleted.
func addDarwinACL(t *testing.T, path, entry string) {
	t.Helper()
	if out, err := exec.Command("/bin/chmod", "-h", "+a", entry, path).CombinedOutput(); err != nil {
		t.Fatalf("/bin/chmod -h +a %q %s: %v: %s", entry, path, err, out)
	}
	t.Cleanup(func() {
		if out, err := exec.Command("/bin/chmod", "-h", "-N", path).CombinedOutput(); err != nil {
			t.Errorf("/bin/chmod -h -N %s: %v: %s", path, err, out)
		}
	})
}

// nativeInstallLayout builds $HOME/.local/bin/claude, with $HOME, .local and
// .local/bin pinned to 0755 so only an ACL could make them replaceable.
func nativeInstallLayout(t *testing.T) (home, local, bin, leaf string) {
	t.Helper()
	home, _ = isolateClaudeDiscovery(t)
	if err := os.Chmod(home, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	local = filepath.Join(home, ".local")
	bin = filepath.Join(local, "bin")
	mkdirMode(t, local, 0o755)
	mkdirMode(t, bin, 0o755)
	leaf = writeFakeClaude(t, bin, fakeClaudeUsageScript, 0o755)
	return home, local, bin, leaf
}

func TestProbeClaudeProseRejectsDarwinACLMutationGrants(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, home, local, bin, leaf string)
	}{
		{"R3 reproduction: .local grants entry replacement", func(t *testing.T, _, local, _, _ string) {
			addDarwinACL(t, local, "everyone allow list,search,add_file,add_subdirectory,delete_child")
		}},
		{"home grants add_file", func(t *testing.T, home, _, _, _ string) {
			addDarwinACL(t, home, "everyone allow add_file")
		}},
		{"install directory grants delete_child", func(t *testing.T, _, _, bin, _ string) {
			addDarwinACL(t, bin, "everyone allow delete_child")
		}},
		{"install directory grants add_subdirectory", func(t *testing.T, _, _, bin, _ string) {
			addDarwinACL(t, bin, "everyone allow add_subdirectory")
		}},
		{"leaf grants write", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow write")
		}},
		{"leaf grants append", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow append")
		}},
		{"leaf grants delete", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow delete")
		}},
		{"leaf grants writesecurity", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow writesecurity")
		}},
		{"leaf grants chown", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow chown")
		}},
		{"leaf grants writeattr", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow writeattr")
		}},
		{"leaf grants writeextattr", func(t *testing.T, _, _, _, leaf string) {
			addDarwinACL(t, leaf, "everyone allow writeextattr")
		}},
		{"allow listed after a deny still grants", func(t *testing.T, _, local, _, _ string) {
			addDarwinACL(t, local, "everyone deny delete")
			addDarwinACL(t, local, "everyone allow add_file,delete_child")
		}},
		{"inherit-only grant fails closed", func(t *testing.T, _, _, bin, _ string) {
			addDarwinACL(t, bin, "everyone allow add_file,delete_child,file_inherit,only_inherit")
		}},
		{"named-user grant fails closed", func(t *testing.T, _, _, _, leaf string) {
			current, err := user.Current()
			if err != nil || current.Username == "" {
				t.Skipf("current user name unavailable: %v", err)
			}
			addDarwinACL(t, leaf, "user:"+current.Username+" allow write")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, local, bin, leaf := nativeInstallLayout(t)
			tc.setup(t, home, local, bin, leaf)
			// Precondition: the mode bits alone are the trusted 0755/owner
			// shape, so only the ACL can be what makes this chain unsafe.
			for _, path := range []string{local, bin, leaf} {
				info, err := os.Lstat(path)
				if err != nil || info.Mode().Perm() != 0o755 || !ownedByRootOrCurrentUser(info) {
					t.Fatalf("precondition: %s = %v (err %v), want 0755 owned by the current user", path, info, err)
				}
			}

			_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
			requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
		})
	}
}

// ACLs on the symlink hops are judged too: the native installer's link, its
// target's directory, and the link object itself.
func TestProbeClaudeProseRejectsDarwinACLOnSymlinkChain(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, versions, link string)
	}{
		{"target directory grants add_file", func(t *testing.T, versions, _ string) {
			addDarwinACL(t, versions, "everyone allow add_file,delete_child")
		}},
		{"link itself grants delete", func(t *testing.T, _, link string) {
			addDarwinACL(t, link, "everyone allow delete")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := isolateClaudeDiscovery(t)
			versions := filepath.Join(home, ".local", "share", "claude", "versions")
			target := writeFakeClaude(t, versions, fakeClaudeUsageScript, 0o755)
			link := filepath.Join(home, ".local", "bin", "claude")
			symlinkOrFatal(t, target, link)
			tc.setup(t, versions, link)

			_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
			requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
		})
	}
}

// XNU lets a parent ACL's delete_child grant (or a delete grant on the entry)
// authorize removal before the sticky-bit rule is consulted, so the sticky
// exception must not survive such an ACL.
func TestProbeClaudeProseStickyExceptionRejectsDarwinACLDeleteChild(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	home := filepath.Join(base, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	previous := claudeProseCommandArgs
	claudeProseCommandArgs = []string{"claude", "-p", "/usage"}
	t.Cleanup(func() { claudeProseCommandArgs = previous })
	writeFakeClaude(t, filepath.Join(home, ".local", "bin"), fakeClaudeUsageScript, 0o755)
	addDarwinACL(t, base, "everyone allow delete_child")

	_, err := ProbeClaudeProse(context.Background(), time.Now(), 5*time.Second)
	requireClaudeFailureKind(t, err, ClaudeFailureSpawn)
}

// Deny entries (macOS puts "everyone deny delete" on home directories) and
// read-only grants do not let anyone else modify the chain and stay trusted.
func TestProbeClaudeProseKeepsHarmlessDarwinACLs(t *testing.T) {
	t.Run("deny and read-only grants on a plain install", func(t *testing.T) {
		home, local, bin, leaf := nativeInstallLayout(t)
		addDarwinACL(t, home, "everyone deny delete")
		addDarwinACL(t, local, "everyone deny delete,add_file,add_subdirectory,delete_child")
		addDarwinACL(t, bin, "everyone deny delete")
		addDarwinACL(t, bin, "everyone allow list,search,readattr,readextattr,readsecurity")
		addDarwinACL(t, leaf, "everyone deny delete,write,append")
		addDarwinACL(t, leaf, "everyone allow read,execute,readattr,readextattr,readsecurity")

		if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
			t.Fatalf("harmless ACLs must stay trusted: %v", err)
		}
	})
	t.Run("deny on the installer symlink and its target", func(t *testing.T) {
		home, _ := isolateClaudeDiscovery(t)
		versions := filepath.Join(home, ".local", "share", "claude", "versions")
		target := writeFakeClaude(t, versions, fakeClaudeUsageScript, 0o755)
		link := filepath.Join(home, ".local", "bin", "claude")
		symlinkOrFatal(t, target, link)
		addDarwinACL(t, link, "everyone deny delete")
		addDarwinACL(t, versions, "everyone deny delete")
		addDarwinACL(t, target, "everyone allow read,readattr,readsecurity")

		if _, err := ProbeClaudeProse(context.Background(), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), 5*time.Second); err != nil {
			t.Fatalf("harmless ACLs on the symlink chain must stay trusted: %v", err)
		}
	})
}

// --- ACL decoding and policy, at the unit level.

const (
	testACEAllow        = 1      // KAUTH_ACE_PERMIT
	testACEDeny         = 2      // KAUTH_ACE_DENY
	testACEAudit        = 3      // KAUTH_ACE_AUDIT
	testACEAlarm        = 4      // KAUTH_ACE_ALARM
	testACEInherited    = 1 << 4 // KAUTH_ACE_INHERITED
	testACEOnlyInherit  = 1 << 8 // KAUTH_ACE_ONLY_INHERIT
	testRightsR3Example = 0x6e   // list,add_file,search,add_subdirectory,delete_child
)

type testACE struct{ flags, rights uint32 }

// buildFilesec encodes a kauth_filesec the way getattrlist returns it (host
// byte order) with the given entry count field and entries.
func buildFilesec(count uint32, entries []testACE) []byte {
	b := make([]byte, claudeKauthFilesecHeaderSize+len(entries)*claudeKauthACESize)
	binary.NativeEndian.PutUint32(b[0:4], claudeKauthFilesecMagic)
	binary.NativeEndian.PutUint32(b[36:40], count)
	everyone := []byte{0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0, 0, 0, 0x0c}
	for i, e := range entries {
		ace := b[claudeKauthFilesecHeaderSize+i*claudeKauthACESize:]
		copy(ace[0:16], everyone)
		binary.NativeEndian.PutUint32(ace[16:20], e.flags)
		binary.NativeEndian.PutUint32(ace[20:24], e.rights)
	}
	return b
}

func TestClaudeFilesecPolicyPerRightBit(t *testing.T) {
	// Every right is either known read-only or rejects; a deny entry never
	// rejects, whatever it names.
	for bit := range 32 {
		rights := uint32(1) << bit
		want := rights&claudeACLReadOnlyRights != 0
		if got := claudeFilesecGrantsNoMutation(buildFilesec(1, []testACE{{testACEAllow, rights}})); got != want {
			t.Errorf("allow bit %d: trusted = %v, want %v", bit, got, want)
		}
		if !claudeFilesecGrantsNoMutation(buildFilesec(1, []testACE{{testACEDeny, rights}})) {
			t.Errorf("deny bit %d rejected; deny entries only remove rights", bit)
		}
	}
	// The modifying rights named in sys/kauth.h must stay outside the
	// read-only set, whatever it is later changed to.
	modifying := map[string]uint32{
		"WRITE_DATA/ADD_FILE": 1 << 2, "DELETE": 1 << 4, "APPEND_DATA/ADD_SUBDIRECTORY": 1 << 5,
		"DELETE_CHILD": 1 << 6, "WRITE_ATTRIBUTES": 1 << 8, "WRITE_EXTATTRIBUTES": 1 << 10,
		"WRITE_SECURITY": 1 << 12, "TAKE_OWNERSHIP": 1 << 13, "GENERIC_ALL": 1 << 21, "GENERIC_WRITE": 1 << 23,
	}
	for name, right := range modifying {
		if claudeFilesecGrantsNoMutation(buildFilesec(1, []testACE{{testACEAllow, right}})) {
			t.Errorf("allow %s was trusted", name)
		}
	}
}

func TestClaudeFilesecPolicyEntriesAndShape(t *testing.T) {
	readOnly := uint32(claudeACLReadOnlyRights)
	cases := []struct {
		name string
		blob []byte
		want bool
	}{
		{"no entries", buildFilesec(0, nil), true},
		{"NOACL sentinel", buildFilesec(claudeKauthFilesecNoACL, nil), true},
		{"all read-only rights", buildFilesec(1, []testACE{{testACEAllow, readOnly}}), true},
		{"deny then read-only allow", buildFilesec(2, []testACE{{testACEDeny, 0xffffffff}, {testACEAllow, readOnly}}), true},
		{"R3 reviewer entry", buildFilesec(1, []testACE{{testACEAllow, testRightsR3Example}}), false},
		{"modifying grant after a deny", buildFilesec(2, []testACE{{testACEDeny, 1 << 2}, {testACEAllow, 1 << 2}}), false},
		{"inherited grant", buildFilesec(1, []testACE{{testACEAllow | testACEInherited, 1 << 2}}), false},
		{"inherit-only grant", buildFilesec(1, []testACE{{testACEAllow | testACEOnlyInherit, 1 << 6}}), false},
		{"audit entry naming a write", buildFilesec(1, []testACE{{testACEAudit, 1 << 2}}), false},
		{"alarm entry naming a write", buildFilesec(1, []testACE{{testACEAlarm, 1 << 2}}), false},
		{"unknown kind naming a write", buildFilesec(1, []testACE{{0xf, 1 << 2}}), false},
		{"unknown kind read-only", buildFilesec(1, []testACE{{0xf, 1 << 1}}), true},
		{"truncated header", buildFilesec(0, nil)[:claudeKauthFilesecHeaderSize-1], false},
		{"wrong magic", func() []byte {
			b := buildFilesec(0, nil)
			binary.NativeEndian.PutUint32(b[0:4], 0x01020304)
			return b
		}(), false},
		{"count larger than entries", buildFilesec(2, []testACE{{testACEDeny, 0}}), false},
		{"count smaller than entries", buildFilesec(1, []testACE{{testACEDeny, 0}, {testACEDeny, 0}}), false},
		{"count above the kernel maximum", buildFilesec(claudeKauthACLMaxEntries+1, make([]testACE, claudeKauthACLMaxEntries+1)), false},
		{"NOACL sentinel with trailing entries", buildFilesec(claudeKauthFilesecNoACL, []testACE{{testACEDeny, 0}}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := claudeFilesecGrantsNoMutation(tc.blob); got != tc.want {
				t.Fatalf("trusted = %v, want %v", got, tc.want)
			}
		})
	}
}

// buildAttrResult lays out a getattrlist result buffer: total length,
// returned attribute_set_t.commonattr, an attrreference_t and its data.
func buildAttrResult(size int, total uint32, returned uint32, offset int32, length uint32, data []byte) []byte {
	buf := make([]byte, size)
	if size >= claudeAttrRefOffset+claudeAttrReferenceSize {
		binary.NativeEndian.PutUint32(buf[claudeAttrSetOffset:], returned)
		binary.NativeEndian.PutUint32(buf[claudeAttrRefOffset:], uint32(offset))
		binary.NativeEndian.PutUint32(buf[claudeAttrRefOffset+4:], length)
		copy(buf[claudeAttrRefOffset+claudeAttrReferenceSize:], data)
	}
	if size >= 4 {
		binary.NativeEndian.PutUint32(buf[0:4], total)
	}
	return buf
}

func TestClaudeExtendedSecurityResultDecoding(t *testing.T) {
	blob := buildFilesec(1, []testACE{{testACEDeny, 1 << 4}})
	full := uint32(claudeAttrRefOffset + claudeAttrReferenceSize + len(blob))
	const withACL = 0x80000000 | 0x00400000
	cases := []struct {
		name                string
		buf                 []byte
		wantPresent, wantOK bool
	}{
		{"no ACL: attribute not returned", buildAttrResult(claudeACLBufferSize, 32, 0x80000000, 0, 0, nil), false, true},
		{"empty reference", buildAttrResult(claudeACLBufferSize, 32, withACL, 8, 0, nil), false, true},
		{"well-formed ACL", buildAttrResult(claudeACLBufferSize, full, withACL, 8, uint32(len(blob)), blob), true, true},
		{"buffer shorter than the fixed header", make([]byte, claudeAttrRefOffset-1), false, false},
		{"total below the fixed header", buildAttrResult(claudeACLBufferSize, claudeAttrRefOffset-1, 0x80000000, 0, 0, nil), false, false},
		{"total beyond the buffer", buildAttrResult(claudeACLBufferSize, claudeACLBufferSize+1, withACL, 8, uint32(len(blob)), blob), false, false},
		{"returned without room for the reference", buildAttrResult(claudeACLBufferSize, claudeAttrRefOffset, withACL, 8, 1, nil), false, false},
		{"offset overlapping the reference", buildAttrResult(claudeACLBufferSize, full, withACL, 4, uint32(len(blob)), blob), false, false},
		{"negative offset", buildAttrResult(claudeACLBufferSize, full, withACL, -24, uint32(len(blob)), blob), false, false},
		{"data past the total", buildAttrResult(claudeACLBufferSize, full, withACL, 8, uint32(len(blob))+1, blob), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, present, ok := claudeExtendedSecurity(tc.buf)
			if present != tc.wantPresent || ok != tc.wantOK {
				t.Fatalf("present, ok = %v, %v; want %v, %v", present, ok, tc.wantPresent, tc.wantOK)
			}
			if tc.wantPresent && !bytes.Equal(got, blob) {
				t.Fatalf("filesec = %x, want %x", got, blob)
			}
		})
	}
}

// The reader against real objects: it judges the object itself (a symlink's
// own ACL, not its target's) and fails closed when the ACL cannot be read.
func TestClaudeACLGrantsNoMutationOnRealObjects(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !claudeACLGrantsNoMutation(file) {
		t.Fatalf("file without an ACL was not trusted")
	}
	addDarwinACL(t, file, "everyone deny delete")
	if !claudeACLGrantsNoMutation(file) {
		t.Fatalf("deny-only ACL was not trusted")
	}
	addDarwinACL(t, file, "everyone allow write")
	if claudeACLGrantsNoMutation(file) {
		t.Fatalf("ACL granting write was trusted")
	}

	link := filepath.Join(dir, "link")
	symlinkOrFatal(t, file, link)
	if !claudeACLGrantsNoMutation(link) {
		t.Fatalf("symlink without its own ACL was judged by its target's ACL")
	}

	unreadable := filepath.Join(dir, "unreadable")
	if err := os.WriteFile(unreadable, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	addDarwinACL(t, unreadable, "everyone deny readsecurity")
	if claudeACLGrantsNoMutation(unreadable) {
		t.Fatalf("object whose ACL cannot be read was trusted")
	}

	if claudeACLGrantsNoMutation(filepath.Join(dir, "missing")) {
		t.Fatalf("missing object was trusted")
	}
	if claudeACLGrantsNoMutation(dir + "/bad\x00name") {
		t.Fatalf("path with a NUL byte was trusted")
	}
}
