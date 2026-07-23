package sources

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func NormalizePath(path string) (string, string, error) {
	candidate := strings.TrimSpace(path)
	if candidate == "" {
		return "", "", &ValidationError{Field: "path", Message: "is required"}
	}
	if strings.ContainsRune(candidate, '\x00') {
		return "", "", &ValidationError{Field: "path", Message: "contains an invalid null character"}
	}

	if candidate == "~" || strings.HasPrefix(candidate, "~/") || strings.HasPrefix(candidate, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", &ValidationError{Field: "path", Message: "cannot resolve the user home directory"}
		}
		if candidate == "~" {
			candidate = home
		} else {
			candidate = filepath.Join(home, candidate[2:])
		}
	} else if strings.HasPrefix(candidate, "~") {
		return "", "", &ValidationError{Field: "path", Message: "only the current user's home shortcut is supported"}
	}

	normalized := filepath.Clean(filepath.FromSlash(candidate))
	if !filepath.IsAbs(normalized) {
		return "", "", &ValidationError{Field: "path", Message: "must be absolute"}
	}

	key := filepath.ToSlash(normalized)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return normalized, key, nil
}

func DetectAvailability(path string) Availability {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return AvailabilityOnline
		}
		return AvailabilityUnknown
	}
	if errors.Is(err, fs.ErrPermission) {
		return AvailabilityPermissionDenied
	}
	if errors.Is(err, fs.ErrNotExist) {
		return AvailabilityMissing
	}
	return AvailabilityOffline
}
