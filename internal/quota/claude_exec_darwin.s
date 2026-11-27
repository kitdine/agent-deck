//go:build darwin

#include "textflag.h"

// libSystem getattrlist(2) trampoline for claude_exec_darwin.go, in the same
// form golang.org/x/sys/unix generates for its own Darwin calls. The bytes are
// identical for amd64 and arm64.
TEXT claude_getattrlist_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_getattrlist(SB)
GLOBL	·claudeGetattrlistTrampolineAddr(SB), RODATA, $8
DATA	·claudeGetattrlistTrampolineAddr(SB)/8, $claude_getattrlist_trampoline<>(SB)
