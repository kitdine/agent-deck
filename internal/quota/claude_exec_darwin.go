//go:build darwin

package quota

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Darwin ACL check for the HOME fallback walk (CQP-R3-F1).
//
// On Darwin, mode bits do not describe every principal that may modify an
// object: an access control list can let other users add, replace or delete
// entries in a 0755 directory, or write the content or security of a 0755
// file. The walk in resolveTrustedClaudeCandidate therefore also reads the ACL
// of every object it traverses and trusts the object only if no ACL entry
// could let anyone modify it.
//
// The ACL is read with getattrlist(2) ATTR_CMN_EXTENDED_SECURITY on the object
// itself (FSOPT_NOFOLLOW), which any user allowed to stat the object may call;
// the com.apple.system.Security extended attribute is readable by root only.
// The vendored golang.org/x/sys/unix provides the getattrlist types and
// constants but not the call, so it is made through a libSystem trampoline in
// the same form x/sys uses for its own Darwin calls (claude_exec_darwin.s):
// no cgo, no external command, no parsing of tool output.
//
// Policy, per ACL entry (sys/kauth.h):
//   - A deny entry only removes rights and is harmless.
//   - Any other entry is harmless only if every right it names is read-only
//     (claudeACLReadOnlyRights). Any other right — adding, replacing or
//     deleting entries, writing or appending content, writing attributes,
//     extended attributes or security, taking ownership, generic all/write, or
//     a bit this code does not know — makes the object untrusted.
//   - The principal is not consulted. Resolving an entry's GUID to a user or
//     group requires the membership API, so a modifying grant to any
//     principal, the current user included, fails closed.
//   - Entry order and inheritance flags are not interpreted: a modifying grant
//     fails closed even if an earlier deny entry shadows it or it is
//     inherit-only.
//
// Any failure to read or decode the ACL fails closed.

const (
	// sys/kauth.h.
	claudeKauthFilesecMagic  = 0x012cc16d // KAUTH_FILESEC_MAGIC
	claudeKauthFilesecNoACL  = 0xffffffff // KAUTH_FILESEC_NOACL
	claudeKauthACLMaxEntries = 128        // KAUTH_ACL_MAX_ENTRIES
	claudeKauthACEKindMask   = 0xf        // KAUTH_ACE_KINDMASK
	claudeKauthACEDeny       = 2          // KAUTH_ACE_DENY

	// struct kauth_filesec is fsec_magic, fsec_owner and fsec_group (guid_t),
	// then struct kauth_acl: acl_entrycount, acl_flags and acl_ace[]. Each
	// struct kauth_ace is ace_applicable (guid_t), ace_flags and ace_rights.
	// getattrlist returns it in host byte order.
	claudeKauthFilesecHeaderSize = 4 + 16 + 16 + 4 + 4
	claudeKauthACESize           = 16 + 4 + 4

	// Read-only rights: KAUTH_VNODE_READ_DATA (LIST_DIRECTORY),
	// KAUTH_VNODE_EXECUTE (SEARCH), KAUTH_VNODE_READ_ATTRIBUTES,
	// KAUTH_VNODE_READ_EXTATTRIBUTES, KAUTH_VNODE_READ_SECURITY,
	// KAUTH_VNODE_SYNCHRONIZE, KAUTH_ACE_GENERIC_EXECUTE and
	// KAUTH_ACE_GENERIC_READ.
	claudeACLReadOnlyRights = 1<<1 | 1<<3 | 1<<7 | 1<<9 | 1<<11 | 1<<20 | 1<<22 | 1<<24

	// getattrlist result for ATTR_CMN_RETURNED_ATTRS |
	// ATTR_CMN_EXTENDED_SECURITY: a u_int32_t total length, the returned
	// attribute_set_t (five u_int32_t), then an attrreference_t (int32_t
	// offset from the reference itself, u_int32_t length) to the variable
	// data. 4096 bytes hold the largest ACL the kernel accepts.
	claudeAttrSetOffset     = 4
	claudeAttrRefOffset     = claudeAttrSetOffset + 5*4
	claudeAttrReferenceSize = 4 + 4
	claudeACLBufferSize     = 4096
)

//go:cgo_import_dynamic libc_getattrlist getattrlist "/usr/lib/libSystem.B.dylib"

// claudeGetattrlistTrampolineAddr is set in claude_exec_darwin.s.
var claudeGetattrlistTrampolineAddr uintptr

// claudeSyscall6 is the runtime's libSystem call path, the one
// golang.org/x/sys/unix uses for its Darwin calls; package syscall exports
// syscall.syscall6 to linkname users for this purpose (go.dev/issue/67401).
//
//go:linkname claudeSyscall6 syscall.syscall6
func claudeSyscall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

func claudeGetattrlist(path string, list *unix.Attrlist, buf []byte, options uint32) error {
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := claudeSyscall6(claudeGetattrlistTrampolineAddr,
		uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(list)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(options), 0)
	runtime.KeepAlive(p)
	runtime.KeepAlive(list)
	runtime.KeepAlive(buf)
	if errno != 0 {
		return errno
	}
	return nil
}

// claudeACLGrantsNoMutation reports whether the ACL of path itself (a
// symlink's own, not its target's) lets nobody modify the object, under the
// policy above. An object without an ACL qualifies; an unreadable or
// undecodable ACL does not.
func claudeACLGrantsNoMutation(path string) bool {
	list := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  unix.ATTR_CMN_RETURNED_ATTRS | unix.ATTR_CMN_EXTENDED_SECURITY,
	}
	buf := make([]byte, claudeACLBufferSize)
	if err := claudeGetattrlist(path, &list, buf, unix.FSOPT_NOFOLLOW); err != nil {
		return false
	}
	filesec, present, ok := claudeExtendedSecurity(buf)
	if !ok {
		return false
	}
	if !present {
		return true
	}
	return claudeFilesecGrantsNoMutation(filesec)
}

// claudeExtendedSecurity extracts the kauth_filesec from a getattrlist result.
// present is false when the object has no ACL: the kernel then leaves
// ATTR_CMN_EXTENDED_SECURITY out of the returned attributes, or returns it
// empty. ok is false for any result that is truncated or inconsistent.
func claudeExtendedSecurity(buf []byte) (filesec []byte, present, ok bool) {
	if len(buf) < claudeAttrRefOffset {
		return nil, false, false
	}
	total := int64(binary.NativeEndian.Uint32(buf[0:4]))
	if total < claudeAttrRefOffset || total > int64(len(buf)) {
		return nil, false, false
	}
	returned := binary.NativeEndian.Uint32(buf[claudeAttrSetOffset : claudeAttrSetOffset+4])
	if returned&unix.ATTR_CMN_EXTENDED_SECURITY == 0 {
		return nil, false, true
	}
	if total < claudeAttrRefOffset+claudeAttrReferenceSize {
		return nil, false, false
	}
	ref := buf[claudeAttrRefOffset : claudeAttrRefOffset+claudeAttrReferenceSize]
	offset := int64(int32(binary.NativeEndian.Uint32(ref[0:4])))
	length := int64(binary.NativeEndian.Uint32(ref[4:8]))
	if length == 0 {
		return nil, false, true
	}
	start := claudeAttrRefOffset + offset
	if offset < claudeAttrReferenceSize || start+length > total {
		return nil, false, false
	}
	return buf[start : start+length], true, true
}

// claudeFilesecGrantsNoMutation applies the policy to a kauth_filesec.
func claudeFilesecGrantsNoMutation(filesec []byte) bool {
	if len(filesec) < claudeKauthFilesecHeaderSize ||
		binary.NativeEndian.Uint32(filesec[0:4]) != claudeKauthFilesecMagic {
		return false
	}
	count := binary.NativeEndian.Uint32(filesec[36:40])
	if count == claudeKauthFilesecNoACL {
		return len(filesec) == claudeKauthFilesecHeaderSize
	}
	if count > claudeKauthACLMaxEntries ||
		len(filesec) != claudeKauthFilesecHeaderSize+int(count)*claudeKauthACESize {
		return false
	}
	for i := range int(count) {
		ace := filesec[claudeKauthFilesecHeaderSize+i*claudeKauthACESize:]
		flags := binary.NativeEndian.Uint32(ace[16:20])
		rights := binary.NativeEndian.Uint32(ace[20:24])
		if flags&claudeKauthACEKindMask == claudeKauthACEDeny {
			continue
		}
		if rights&^claudeACLReadOnlyRights != 0 {
			return false
		}
	}
	return true
}
