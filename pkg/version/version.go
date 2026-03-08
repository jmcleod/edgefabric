// Package version provides build-time version information.
package version

import (
	"fmt"
	"runtime"
)

// Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("edgefabric %s (commit: %s, built: %s, go: %s)",
		Version, Commit, BuildTime, runtime.Version())
}
