// Package version exposes the application version string.
package version

// Version is the application version. It is injected at build time via
//
//	-ldflags "-X github.com/salehkreiner/scrubadubber/internal/version.Version=v1.0.0"
//
// and defaults to "dev" for local (untagged) builds.
var Version = "dev"
