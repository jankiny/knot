package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveUsesEnvironmentOverride(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	configured := filepath.Join(workingDirectory, "..", "..", "data", "_test_appdata")

	directory, err := resolve(
		func(key string) string {
			if key != EnvDataDir {
				t.Fatalf("unexpected environment key: %s", key)
			}
			return configured
		},
		func() (string, error) {
			t.Fatal("system configuration fallback must not be used")
			return "", nil
		},
	)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if directory.Path != filepath.Clean(configured) {
		t.Fatalf("expected %q, got %q", filepath.Clean(configured), directory.Path)
	}
	if directory.Source != SourceEnvironment {
		t.Fatalf("expected environment source, got %q", directory.Source)
	}
}

func TestResolveUsesSystemConfigurationFallback(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	configRoot := filepath.Join(workingDirectory, "system-config")

	directory, err := resolve(
		func(string) string { return "" },
		func() (string, error) { return configRoot, nil },
	)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	expected := filepath.Join(configRoot, defaultDirectoryName)
	if directory.Path != expected {
		t.Fatalf("expected %q, got %q", expected, directory.Path)
	}
	if directory.Source != SourceSystem {
		t.Fatalf("expected system source, got %q", directory.Source)
	}
}

func TestResolveRejectsRelativeEnvironmentOverride(t *testing.T) {
	_, err := resolve(
		func(string) string { return filepath.Join("data", "_test_appdata") },
		func() (string, error) {
			t.Fatal("system configuration fallback must not replace an invalid override")
			return "", nil
		},
	)
	if err == nil {
		t.Fatal("expected relative application data directory to be rejected")
	}
}

func TestResolveReturnsSystemConfigurationError(t *testing.T) {
	expected := errors.New("configuration directory unavailable")

	_, err := resolve(
		func(string) string { return "" },
		func() (string, error) { return "", expected },
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped configuration error, got %v", err)
	}
}

func TestResolveCleansWindowsEnvironmentPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path behavior")
	}

	tests := []struct {
		name       string
		configured string
		expected   string
	}{
		{
			name:       "backslashes and parent segment",
			configured: `C:\Workspace\20_Dev\app-knot\data\ignored\..\_test_appdata`,
			expected:   `C:\Workspace\20_Dev\app-knot\data\_test_appdata`,
		},
		{
			name:       "forward slashes and Unicode",
			configured: `D:/用户数据/Knot`,
			expected:   `D:\用户数据\Knot`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory, err := resolve(
				func(string) string { return test.configured },
				func() (string, error) {
					t.Fatal("system configuration fallback must not be used")
					return "", nil
				},
			)
			if err != nil {
				t.Fatalf("resolve failed: %v", err)
			}
			if directory.Path != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, directory.Path)
			}
		})
	}
}
