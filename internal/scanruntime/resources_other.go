//go:build !darwin && !linux

package scanruntime

func currentProcessResources() processResources { return processResources{} }
