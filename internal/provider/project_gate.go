package provider

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kitdine/agent-deck/internal/platform"
)

const ProjectAttributionGateFilename = "project-attribution.enabled"

type ProjectAttributionGateStatus struct {
	Required   bool `json:"required"`
	Present    bool `json:"present"`
	Consistent bool `json:"consistent"`
}

func ProjectAttributionGatePath(stateRoot string) string {
	return filepath.Join(stateRoot, ProjectAttributionGateFilename)
}

func (s Service) ProjectAttributionGateStatus(ctx context.Context) (ProjectAttributionGateStatus, error) {
	required, err := s.projectAttributionGateRequired(ctx)
	if err != nil {
		return ProjectAttributionGateStatus{}, err
	}
	status := ProjectAttributionGateStatus{Required: required}
	info, err := os.Lstat(ProjectAttributionGatePath(s.StateRoot))
	if errors.Is(err, fs.ErrNotExist) {
		status.Consistent = !required
		return status, nil
	}
	if err != nil {
		return status, fmt.Errorf("inspect project attribution gate: %w", err)
	}
	status.Present = true
	if !info.Mode().IsRegular() {
		return status, errors.New("project attribution gate is not a regular file")
	}
	if info.Mode().Perm() != platform.FileMode {
		return status, fmt.Errorf(
			"project attribution gate mode is %04o, want %04o",
			info.Mode().Perm(),
			platform.FileMode,
		)
	}
	status.Consistent = required
	return status, nil
}

func (s Service) RefreshProjectAttributionGate(ctx context.Context) error {
	required, err := s.projectAttributionGateRequired(ctx)
	if err != nil {
		return err
	}
	if s.StateRoot == "" {
		return errors.New("state directory is unavailable")
	}
	path := ProjectAttributionGatePath(s.StateRoot)
	if !required {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove project attribution gate: %w", err)
		}
		return nil
	}

	temporary, err := os.CreateTemp(s.StateRoot, "."+ProjectAttributionGateFilename+".tmp-*")
	if err != nil {
		return fmt.Errorf("create project attribution gate: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(platform.FileMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set project attribution gate mode: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync project attribution gate: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close project attribution gate: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace project attribution gate: %w", err)
	}
	return nil
}

func (s Service) projectAttributionGateRequired(ctx context.Context) (bool, error) {
	for _, client := range []Client{ClientCodex, ClientClaude} {
		eligibility, err := s.ProjectRouteEligibility(ctx, client)
		if err != nil {
			return false, err
		}
		if eligibility == ProjectRouteEligible {
			return true, nil
		}
	}
	return false, nil
}
