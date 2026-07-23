//go:build !windows

package safepath

import "path/filepath"

func canonicalExistingPath(value string) (string, error) {
	return filepath.EvalSymlinks(value)
}
