package platform

import (
	"context"
	"errors"
	"sync"
)

// ErrStateOwnershipRefused is deliberately path-free: diagnostics must not
// disclose the receipt's executable or the account's installation layout.
type ErrStateOwnershipRefused struct {
	Resource string `json:"resource"`
	Reason   string `json:"reason"`
	Revoked  bool   `json:"revoked,omitempty"`
}

func (e *ErrStateOwnershipRefused) Error() string {
	if e.Resource == "client_config" {
		return clientConfigRefusedMessage
	}
	return "This AgentDeck is not the installed program and cannot use the real state directory."
}

func stateRefused(reason string) error {
	return &ErrStateOwnershipRefused{Resource: "state_root", Reason: reason}
}

type formulaRenewer interface {
	CanRenewFormula() (bool, error)
	RenewFormula(context.Context) (bool, error)
}

// ResolveInstallationForState is shared by default selection and the guard's
// protected-root branch. It never reads state contents. Explicit isolated roots
// bypass this function so they cannot claim an installation as a side effect.
func ResolveInstallationForState() (InstallationIdentity, error) {
	identity := CurrentInstallation()
	if identity.Installed {
		return identity, nil
	}
	installationResolver.RLock()
	renewer, ok := installationResolver.value.(formulaRenewer)
	installationResolver.RUnlock()
	if !ok {
		return identity, nil
	}
	eligible, err := renewer.CanRenewFormula()
	if err != nil || !eligible {
		return identity, nil
	}
	if _, err := renewer.RenewFormula(context.Background()); err != nil {
		reason := "receipt_invalid"
		var busy *OwnershipLockError
		if errors.As(err, &busy) {
			reason = "ownership_busy"
		}
		return InstallationIdentity{}, stateRefused(reason)
	}
	identity = CurrentInstallation()
	if !identity.Installed {
		return identity, stateRefused(identity.Reason)
	}
	return identity, nil
}

// The process session covers nested package calls and handles retained between
// rounds. No package may release it when only its own handle has closed. The CLI
// retains it through kernel process teardown; explicit release is for consumers
// that have first closed every handle in the process session.
var stateOwnership struct {
	sync.Mutex
	lock *OwnershipLock
}

// GuardStateRoot returns the physical path that every subsequent operation
// must use. Protected contents are resolved only after a shared session exists
// and the receipt has been checked inside that session.
func GuardStateRoot(root string) (string, error) {
	protected, err := ProtectedStatePaths()
	if err != nil {
		return "", stateRefused("real_root_unresolved")
	}
	physical, err := statePaths.walk(root, protected, nil)
	if err == nil {
		return physical, nil
	}
	if !errors.Is(err, ErrProtectedStatePath) {
		return "", stateRefused("state_path_unresolved")
	}
	stateOwnership.Lock()
	defer stateOwnership.Unlock()
	if stateOwnership.lock == nil {
		identity, err := ResolveInstallationForState()
		if err != nil {
			return "", err
		}
		if !identity.Installed {
			return "", stateRefused(identity.Reason)
		}
		account, err := AccountHome()
		if err != nil {
			return "", stateRefused("real_root_unresolved")
		}
		lock, err := AcquireOwnershipLock(context.Background(), account, false)
		if err != nil {
			return "", stateRefused("ownership_busy")
		}
		if identity = CurrentInstallation(); !identity.Installed {
			_ = lock.Release()
			return "", stateRefused(identity.Reason)
		}
		stateOwnership.lock = lock
	} else if identity := CurrentInstallation(); !identity.Installed {
		return "", stateRefused(identity.Reason)
	}
	physical, err = statePaths.walk(root, nil, nil)
	if err != nil {
		return "", stateRefused("state_path_unresolved")
	}
	return physical, nil
}

// RevalidateOwnership runs before a long-lived consumer begins its next round.
// On failure the caller tears down its handles before releasing the session.
func RevalidateOwnership(root string) error {
	_, err := GuardStateRoot(root)
	var refused *ErrStateOwnershipRefused
	if errors.As(err, &refused) {
		return &ErrStateOwnershipRefused{Resource: refused.Resource, Reason: refused.Reason, Revoked: true}
	}
	return err
}

// ReleaseStateOwnership ends a process session after ALL consumers, including
// background goroutines, database pools and state locks, have stopped.
func ReleaseStateOwnership() error {
	stateOwnership.Lock()
	defer stateOwnership.Unlock()
	err := stateOwnership.lock.Release()
	stateOwnership.lock = nil
	return err
}
