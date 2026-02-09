// Package build holds build-time version information injected via ldflags.
//
// To inject values at build time:
//
//	go build -ldflags "-X github.com/haivivi/giztoy/go/cmd/giztoy/internal/build.Version=v1.0.0 \
//	  -X github.com/haivivi/giztoy/go/cmd/giztoy/internal/build.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/haivivi/giztoy/go/cmd/giztoy/internal/build.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package build

import (
	"fmt"
	"runtime"
)

// These variables are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("giztoy %s (%s) built %s %s/%s",
		Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
