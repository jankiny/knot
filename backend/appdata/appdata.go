// Package appdata provides the single application data directory resolver used
// by the backend. Callers must not infer an application data path themselves.
package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvDataDir overrides the application data directory for development and
	// tests. Electron sets it to app.getPath("userData") for normal launches.
	EnvDataDir = "KNOT_DATA_DIR"

	defaultDirectoryName = "Knot"
)

// Source describes how the application data directory was selected.
type Source string

const (
	SourceEnvironment Source = "environment"
	SourceSystem      Source = "system_config"
)

// Directory is the resolved absolute application data directory and its source.
type Directory struct {
	Path   string
	Source Source
}

// Resolve returns the backend application data directory.
//
// KNOT_DATA_DIR takes precedence. When it is unset, Resolve uses the operating
// system user configuration directory with a "Knot" child directory.
func Resolve() (Directory, error) {
	return resolve(os.Getenv, os.UserConfigDir)
}

func resolve(
	getenv func(string) string,
	userConfigDir func() (string, error),
) (Directory, error) {
	if configured := strings.TrimSpace(getenv(EnvDataDir)); configured != "" {
		return newDirectory(configured, SourceEnvironment)
	}

	configRoot, err := userConfigDir()
	if err != nil {
		return Directory{}, fmt.Errorf("resolve system user configuration directory: %w", err)
	}

	return newDirectory(filepath.Join(configRoot, defaultDirectoryName), SourceSystem)
}

func newDirectory(path string, source Source) (Directory, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return Directory{}, fmt.Errorf("application data directory must be absolute")
	}

	return Directory{
		Path:   cleaned,
		Source: source,
	}, nil
}
