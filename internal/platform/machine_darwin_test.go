//go:build darwin

package platform

import "testing"

func TestPlatformUUIDPattern(t *testing.T) {
	output := []byte(`    "IOPlatformUUID" = "00000000-1111-2222-3333-444444444444"`)
	matches := platformUUIDPattern.FindSubmatch(output)
	if len(matches) != 2 || string(matches[1]) != "00000000-1111-2222-3333-444444444444" {
		t.Fatalf("platform UUID matches = %q", matches)
	}
}
