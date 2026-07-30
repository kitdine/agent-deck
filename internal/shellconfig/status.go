package shellconfig

import (
	"errors"
	"io/fs"
	"strconv"
)

type ConfigurationState string

const (
	ConfigurationAbsent     ConfigurationState = "absent"
	ConfigurationConfigured ConfigurationState = "configured"
	ConfigurationModified   ConfigurationState = "modified"
	ConfigurationInvalid    ConfigurationState = "invalid"
)

type ActivationState string

const (
	ActivationActive    ActivationState = "active"
	ActivationInactive  ActivationState = "inactive"
	ActivationInherited ActivationState = "inherited_from_ancestor"
)

type StatusResult struct {
	Shell         Shell              `json:"shell"`
	Path          string             `json:"path"`
	Configuration ConfigurationState `json:"configuration_state"`
	Activation    ActivationState    `json:"activation_state"`
	Error         string             `json:"error,omitempty"`
}

type StatusSummary struct {
	Results []StatusResult `json:"shells"`
}

func ActivationMarkerName(shell Shell) string {
	switch shell {
	case ShellBash:
		return "AGENTDECK_SHELL_INTEGRATION_BASH"
	case ShellFish:
		return "AGENTDECK_SHELL_INTEGRATION_FISH"
	case ShellZsh:
		return "AGENTDECK_SHELL_INTEGRATION_ZSH"
	default:
		return ""
	}
}

func (m *Manager) Status(request Request, markers map[Shell]string, parentPID int) (StatusSummary, error) {
	targets, err := m.targets(request, true)
	if err != nil {
		return StatusSummary{}, err
	}
	summary := StatusSummary{Results: make([]StatusResult, 0, len(targets))}
	activeReported := false
	for _, target := range targets {
		if request.Shell == "" && !target.selected {
			continue
		}
		configuration, inspectErr := m.configurationState(target.path, target.shell)
		activation := m.activationState(target.shell, markers[target.shell], parentPID)
		if activation == ActivationActive &&
			request.Shell == "" &&
			target.shell == ShellBash {
			invokingPath, pathErr := m.defaultPath(ShellBash, m.environment.Invocation.Login)
			if pathErr != nil || target.path != invokingPath {
				activation = ActivationInherited
			}
		}
		if activation == ActivationActive {
			if activeReported {
				activation = ActivationInherited
			} else {
				activeReported = true
			}
		}
		result := StatusResult{
			Shell:         target.shell,
			Path:          target.path,
			Configuration: configuration,
			Activation:    activation,
		}
		if inspectErr != nil {
			result.Error = inspectErr.Error()
		}
		summary.Results = append(summary.Results, result)
	}
	return summary, nil
}

func (m *Manager) configurationState(path string, shell Shell) (ConfigurationState, error) {
	contents, _, err := m.readStartup(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ConfigurationAbsent, nil
	}
	if err != nil {
		return ConfigurationInvalid, err
	}
	block := inspectManagedBlock(contents)
	switch block.state {
	case blockAbsent:
		return ConfigurationAbsent, nil
	case blockInvalid:
		if block.modified {
			return ConfigurationModified, block.err
		}
		return ConfigurationInvalid, block.err
	case blockValid:
		if err := validateManagedBlock(block, shell); err != nil {
			return ConfigurationInvalid, err
		}
		return ConfigurationConfigured, nil
	default:
		return ConfigurationInvalid, errors.New("managed shell block state is invalid")
	}
}

func (m *Manager) activationState(shell Shell, marker string, parentPID int) ActivationState {
	if marker == "" {
		return ActivationInactive
	}
	markerPID, err := strconv.Atoi(marker)
	if err == nil &&
		markerPID > 0 &&
		markerPID == parentPID &&
		m.environment.Invocation.Shell == shell {
		return ActivationActive
	}
	return ActivationInherited
}
