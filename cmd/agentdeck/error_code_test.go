package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/errdefs"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestWrappedErrorCodeAndExitCodeMatrix(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantExit int
	}{
		{"extension read only", extension.ErrReadOnly, "extension_read_only", 1},
		{"extension not found", store.ErrExtensionNotFound, "extension_not_found", 1},
		{"invalid backup", backup.ErrInvalidArchive, "invalid_backup", 1},
		{"restore target not empty", backup.ErrTargetNotEmpty, "restore_target_not_empty", 1},
		{"backup destination exists", backup.ErrDestinationExists, "backup_exists", 1},
		{"credential key missing", credentialvault.ErrKeyMissing, "credential_key_missing", 1},
		{"credential key permissions", credentialvault.ErrKeyPermissions, "credential_key_permissions", 1},
		{"credential key machine mismatch", credentialvault.ErrKeyMachineMismatch, "credential_key_machine_mismatch", 1},
		{"credential key version unsupported", credentialvault.ErrKeyVersionUnsupported, "credential_key_version_unsupported", 1},
		{"credential ciphertext invalid", credentialvault.ErrCiphertextInvalid, "credential_ciphertext_invalid", 1},
		{"machine identity missing", credentialvault.ErrMachineIdentityMissing, "machine_identity_unavailable", 1},
		{"provider not found", errdefs.NewNotFound(store.CodeProviderNotFound, "synthetic provider", errors.New("synthetic cause")), "provider_not_found", 1},
		{"credential not found", errdefs.NewNotFound(store.CodeCredentialNotFound, "synthetic credential", errors.New("synthetic cause")), "credential_not_found", 1},
		{"backup not found", errdefs.NewNotFound(backup.CodeArchiveNotFound, "synthetic backup", errors.New("synthetic cause")), "backup_not_found", 1},
		{"backup unreadable", errdefs.NewNotFound(backup.CodeArchiveUnreadable, "synthetic backup", errors.New("synthetic cause")), "backup_unreadable", 1},
		{"session not found", errdefs.NewNotFound(session.CodeSessionNotFound, "synthetic session", errors.New("synthetic cause")), "session_not_found", 1},
		{"state busy", store.ErrStateBusy, "state_busy", 1},
		{"unsupported desktop wire version", desktop.ErrUnsupportedWireVersion, "unsupported_wire_version", 2},
		{"invalid desktop recent limit", desktop.ErrInvalidRecentLimit, "invalid_recent_limit", 2},
		{"input error", &inputError{err: errors.New("synthetic invalid input")}, "invalid_argument", 2},
		{"invalid provider", provider.ErrInvalidProvider, "invalid_argument", 2},
		{"invalid multiplier", provider.ErrInvalidMultiplier, "invalid_argument", 2},
		{"unknown runtime error", errors.New("synthetic runtime failure"), "runtime_error", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("outer context: %w", fmt.Errorf("inner context: %w", tt.err))
			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("errorCode() = %q, want %q", got, tt.wantCode)
			}
			if got := errorExitCode(err); got != tt.wantExit {
				t.Fatalf("errorExitCode() = %d, want %d", got, tt.wantExit)
			}
		})
	}
}
