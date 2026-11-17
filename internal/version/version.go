// Package version holds the application version.
//
// The single source of truth is the VERSION file at the repository root.
// Release and Taskfile builds inject it with
//
//	go build -ldflags "-X exbasicstudio/internal/version.Version=$(cat VERSION)"
//
// and the same value is propagated to the installers and package metadata.
package version

// Version is overridden at link time; the default marks an untagged build.
var Version = "0.0.0-dev"
