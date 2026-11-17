//go:build windows

package project

import (
	"errors"
	"os"
	"syscall"
)

// isLink reports symbolic links and every other NTFS reparse point, such as
// directory junctions and volume mount points.
func isLink(info os.FileInfo) bool {
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return true
	}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

// transientReplaceError reports the errors Windows returns while another
// program briefly holds the file being replaced.
func transientReplaceError(err error) bool {
	const sharingViolation = syscall.Errno(32)
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, sharingViolation)
}
