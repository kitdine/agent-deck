//go:build !darwin && !linux

package scanruntime

import "os/exec"

func detachWorkerProcess(*exec.Cmd) {}

func lowerProcessPriority() {}

func currentProcessResources() processResources { return processResources{} }
