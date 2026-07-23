//go:build windows

package safepath

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func canonicalExistingPath(value string) (string, error) {
	pathPointer, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		pathPointer,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, 512)
	for {
		length, err := windows.GetFinalPathNameByHandle(
			handle,
			&buffer[0],
			uint32(len(buffer)),
			0,
		)
		if err != nil {
			return "", err
		}
		if length < uint32(len(buffer)) {
			resolved := windows.UTF16ToString(buffer[:length])
			return filepath.Clean(normalizeWindowsFinalPath(resolved)), nil
		}
		buffer = make([]uint16, int(length)+1)
	}
}

func normalizeWindowsFinalPath(value string) string {
	switch {
	case strings.HasPrefix(value, `\\?\UNC\`):
		return `\\` + strings.TrimPrefix(value, `\\?\UNC\`)
	case strings.HasPrefix(value, `\\?\`):
		return strings.TrimPrefix(value, `\\?\`)
	default:
		return value
	}
}
