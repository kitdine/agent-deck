//go:build !darwin

package quota

import "runtime"

// claudeACLGrantsNoMutation is the counterpart of the Darwin ACL check
// (claude_exec_darwin.go) on other platforms.
//
// On Linux, the named-user and named-group entries of a POSIX ACL are bounded
// by its mask, and the mask is what the group permission bits report, so the
// walk's group/other-write mode checks already reject an ACL that grants
// writes. No other platform's ACL model is verified here, so the HOME fallback
// fails closed on them; platforms without Unix ownership are rejected earlier.
func claudeACLGrantsNoMutation(string) bool {
	return runtime.GOOS == "linux"
}
