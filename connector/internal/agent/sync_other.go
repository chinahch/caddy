//go:build !linux

package agent

// The worker only runs on Linux. Other targets support config/unit validation.
func syncDirectory(path string) error { return nil }
