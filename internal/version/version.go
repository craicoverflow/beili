// Package version exposes the build version, injected at link time with
// -ldflags "-X github.com/craicoverflow/beili/internal/version.Version=v1.2.3".
package version

// Version is the git tag the binary was built from, or "dev" for local builds.
var Version = "dev"
