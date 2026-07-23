package safepath

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"knot-backend/policy"
	"knot-backend/sources"
)

type fakeRegistry struct {
	roots map[string]sources.SourceRoot
}

func (registry fakeRegistry) Get(
	_ context.Context,
	id string,
) (sources.SourceRoot, error) {
	root, ok := registry.roots[id]
	if !ok {
		return sources.SourceRoot{}, sources.ErrNotFound
	}
	return root, nil
}

func (registry fakeRegistry) List(
	_ context.Context,
) ([]sources.SourceRoot, error) {
	roots := make([]sources.SourceRoot, 0, len(registry.roots))
	for _, root := range registry.roots {
		roots = append(roots, root)
	}
	return roots, nil
}

func TestResolverResolvesRegisteredRelativePath(t *testing.T) {
	rootPath := stage2TestDirectory(t, "resolve_registered")
	nestedPath := filepath.Join(rootPath, "周", "2026")
	if err := os.MkdirAll(nestedPath, 0o700); err != nil {
		t.Fatalf("create nested path: %v", err)
	}

	resolver := testResolver(rootPath, sources.LocalAccessRead, sources.AIAccessContent)
	result, err := resolver.Resolve(context.Background(), Request{
		SourceRootID: "root_test",
		RelativePath: `周\2026\report.md`,
	})
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if result.SourceRootID != "root_test" ||
		result.RelativePath != "周/2026/report.md" ||
		!pathWithinRoot(rootPath, result.AbsolutePath) ||
		!pathWithinRoot(rootPath, result.CanonicalPath) {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.Policy.AllowsLocalRead() || !result.Policy.AllowsAIContent() {
		t.Fatalf("unexpected policy: %+v", result.Policy)
	}
}

func TestResolverRejectsTraversalAbsolutePathsAndForgedRootIDs(t *testing.T) {
	rootPath := stage2TestDirectory(t, "reject_unsafe")
	resolver := testResolver(rootPath, sources.LocalAccessRead, sources.AIAccessContent)

	for _, relativePath := range []string{
		"..",
		"../outside.txt",
		`safe\..\..\outside.txt`,
		`C:\Windows\system.ini`,
		`C:Windows\system.ini`,
		`\\server\share\file.txt`,
		"/etc/passwd",
		`\Windows\system.ini`,
	} {
		t.Run(relativePath, func(t *testing.T) {
			_, err := resolver.Resolve(context.Background(), Request{
				SourceRootID: "root_test",
				RelativePath: relativePath,
			})
			assertResolveReason(t, err, policy.ReasonPathOutsideRoot)
		})
	}

	_, err := resolver.Resolve(context.Background(), Request{
		SourceRootID: "root_forged",
		RelativePath: "report.md",
	})
	assertResolveReason(t, err, policy.ReasonPathOutsideRoot)
}

func TestResolverBridgesOnlyRegisteredLegacyAbsolutePaths(t *testing.T) {
	testPath := stage2TestDirectory(t, "legacy_bridge")
	rootPath := filepath.Join(testPath, "root")
	nestedRootPath := filepath.Join(rootPath, "nested")
	outsidePath := filepath.Join(testPath, "outside")
	for _, directory := range []string{rootPath, nestedRootPath, outsidePath} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create directory: %v", err)
		}
	}

	parentRoot := onlineRoot(rootPath, sources.LocalAccessRead, sources.AIAccessContent)
	parentRoot.ID = "root_parent"
	nestedRoot := onlineRoot(
		nestedRootPath,
		sources.LocalAccessReadWrite,
		sources.AIAccessMetadata,
	)
	nestedRoot.ID = "root_nested"
	resolver := NewResolver(fakeRegistry{roots: map[string]sources.SourceRoot{
		parentRoot.ID: parentRoot,
		nestedRoot.ID: nestedRoot,
	}})

	result, err := resolver.ResolveRegisteredAbsolute(
		context.Background(),
		filepath.Join(nestedRootPath, "report.md"),
	)
	if err != nil {
		t.Fatalf("resolve registered absolute path: %v", err)
	}
	if result.SourceRootID != nestedRoot.ID || result.RelativePath != "report.md" {
		t.Fatalf("expected most specific source root, got %+v", result)
	}

	_, err = resolver.ResolveRegisteredAbsolute(
		context.Background(),
		filepath.Join(outsidePath, "report.md"),
	)
	assertResolveReason(t, err, policy.ReasonPathOutsideRoot)
}

func TestResolverEnforcesRootStateAndLocalAccess(t *testing.T) {
	rootPath := stage2TestDirectory(t, "root_policy")
	tests := []struct {
		name   string
		change func(*sources.SourceRoot)
		reason policy.ReasonCode
	}{
		{
			name: "disabled root",
			change: func(root *sources.SourceRoot) {
				root.Enabled = false
			},
			reason: policy.ReasonRootDisabled,
		},
		{
			name: "offline root",
			change: func(root *sources.SourceRoot) {
				root.Availability = sources.AvailabilityOffline
			},
			reason: policy.ReasonRootOffline,
		},
		{
			name: "local access none",
			change: func(root *sources.SourceRoot) {
				root.LocalAccess = sources.LocalAccessNone
			},
			reason: policy.ReasonLocalAccessDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := onlineRoot(rootPath, sources.LocalAccessRead, sources.AIAccessContent)
			test.change(&root)
			resolver := NewResolver(fakeRegistry{roots: map[string]sources.SourceRoot{
				root.ID: root,
			}})
			_, err := resolver.Resolve(context.Background(), Request{
				SourceRootID: root.ID,
				RelativePath: ".",
			})
			assertResolveReason(t, err, test.reason)
		})
	}
}

func TestResolverAllowsLocalNoAIPathAndReturnsAIDenialReasons(t *testing.T) {
	rootPath := stage2TestDirectory(t, "PhotoLibrary_NoAI")
	resolver := testResolver(rootPath, sources.LocalAccessReadWrite, sources.AIAccessContent)

	result, err := resolver.Resolve(context.Background(), Request{
		SourceRootID: "root_test",
		RelativePath: "2026/city",
	})
	if err != nil {
		t.Fatalf("resolve NoAI path for local use: %v", err)
	}
	if !result.Policy.AllowsLocalWrite() || result.Policy.AllowsAIMetadata() {
		t.Fatalf("unexpected NoAI policy: %+v", result.Policy)
	}
	if !result.Policy.HasReason(policy.ReasonSensitivePathMarker) ||
		!result.Policy.HasReason(policy.ReasonAIAccessDenied) {
		t.Fatalf("missing structured denial reasons: %+v", result.Policy.Reasons)
	}
}

func TestResolverMetadataSourceDoesNotAllowAIContent(t *testing.T) {
	rootPath := stage2TestDirectory(t, "metadata_source")
	resolver := testResolver(rootPath, sources.LocalAccessRead, sources.AIAccessMetadata)

	result, err := resolver.Resolve(context.Background(), Request{
		SourceRootID: "root_test",
		RelativePath: "report.md",
	})
	if err != nil {
		t.Fatalf("resolve metadata path: %v", err)
	}
	if !result.Policy.AllowsAIMetadata() || result.Policy.AllowsAIContent() ||
		!result.Policy.HasReason(policy.ReasonMetadataOnly) {
		t.Fatalf("unexpected metadata policy: %+v", result.Policy)
	}
}

func TestResolverRejectsLinkEscape(t *testing.T) {
	testPath := stage2TestDirectory(t, "symlink_escape")
	rootPath := filepath.Join(testPath, "root")
	outsidePath := filepath.Join(testPath, "outside")
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		t.Fatalf("create root: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o700); err != nil {
		t.Fatalf("create outside: %v", err)
	}

	linkPath := filepath.Join(rootPath, "escape")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("symbolic links are unavailable in this environment: %v", err)
		}
		output, junctionErr := exec.Command(
			"cmd.exe",
			"/c",
			"mklink",
			"/J",
			linkPath,
			outsidePath,
		).CombinedOutput()
		if junctionErr != nil {
			t.Skipf(
				"symbolic links and junctions are unavailable: symlink=%v junction=%v output=%s",
				err,
				junctionErr,
				strings.TrimSpace(string(output)),
			)
		}
	}
	t.Cleanup(func() {
		_ = os.Remove(linkPath)
	})

	resolver := testResolver(rootPath, sources.LocalAccessRead, sources.AIAccessContent)
	_, err := resolver.Resolve(context.Background(), Request{
		SourceRootID: "root_test",
		RelativePath: "escape/private.txt",
	})
	assertResolveReason(t, err, policy.ReasonSymlinkEscape)
}

func TestNormalizePathKeyIsWindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path normalization")
	}
	left := normalizePathKey(`C:\Workspace\Notes\Report.md`)
	right := normalizePathKey(`c:/workspace/notes/report.md`)
	if left != right {
		t.Fatalf("expected equivalent keys, got %q and %q", left, right)
	}
}

func testResolver(
	rootPath string,
	localAccess sources.LocalAccess,
	aiAccess sources.AIAccess,
) *Resolver {
	root := onlineRoot(rootPath, localAccess, aiAccess)
	return NewResolver(fakeRegistry{roots: map[string]sources.SourceRoot{
		root.ID: root,
	}})
}

func onlineRoot(
	rootPath string,
	localAccess sources.LocalAccess,
	aiAccess sources.AIAccess,
) sources.SourceRoot {
	return sources.SourceRoot{
		ID:           "root_test",
		Name:         "Test root",
		Kind:         sources.KindReference,
		Path:         rootPath,
		ScopeType:    sources.ScopeGlobal,
		Enabled:      true,
		Recursive:    true,
		LocalAccess:  localAccess,
		AIAccess:     aiAccess,
		Availability: sources.AvailabilityOnline,
	}
}

func assertResolveReason(
	t *testing.T,
	err error,
	want policy.ReasonCode,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q error", want)
	}
	got, ok := Reason(err)
	if !ok || got != want {
		t.Fatalf("expected reason %q, got %q (%v)", want, got, err)
	}
}

func stage2TestDirectory(t *testing.T, name string) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	testPath, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage2",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve test directory: %v", err)
	}
	if err := os.RemoveAll(testPath); err != nil {
		t.Fatalf("reset test directory: %v", err)
	}
	if err := os.MkdirAll(testPath, 0o700); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(testPath)
	})
	return testPath
}

var _ SourceRootRegistry = fakeRegistry{}
