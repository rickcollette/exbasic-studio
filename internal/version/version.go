// Package version holds the application version and fix-build revision.
//
// The sources of truth are VERSION and BUILD_NUMBER at the repository root.
// Release and Taskfile builds inject both with
//
//	go build -ldflags "-X exbasicstudio/internal/version.Version=$(cat VERSION) -X exbasicstudio/internal/version.Build=$(cat BUILD_NUMBER)"
//
// and the same value is propagated to the installers and package metadata.
package version

import "fmt"

// Version is overridden at link time; the default marks an untagged build.
var Version = "0.0.0-dev"

// Build is incremented for fixes that do not change the product version.
var Build = "0"

// Display returns the user-facing product and build identity.
func Display() string {
	if Build == "" || Build == "0" {
		return Version
	}
	return fmt.Sprintf("%s (build %s)", Version, Build)
}
