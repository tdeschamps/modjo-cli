// Package version exposes build metadata injected via -ldflags at release time
// and the `modjo version` / `modjo --version` output.
package version

import (
	"fmt"
	"runtime"
)

// These are overridden at build time:
//
//	-ldflags "-X .../version.Version=v1.2.3 -X .../version.Commit=abc -X .../version.Date=..."
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line version summary.
func String() string {
	return fmt.Sprintf("modjo %s (%s) built %s %s/%s",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
