//go:build windows

package bridgecheck

import "golang.org/x/sys/windows/registry"

// Configured reports whether ANTHROPIC_BASE_URL is set in the user's persisted
// environment (HKCU\Environment), which is what scrub-setup writes on Windows.
func Configured() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(envVar)
	return err == nil && v != ""
}
