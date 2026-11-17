//go:build !windows

package project

import "os"

// isLink reports symbolic links.
func isLink(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }

// transientReplaceError is Windows-only: a rename over an open file succeeds
// elsewhere.
func transientReplaceError(error) bool { return false }
