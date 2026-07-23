package sources

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"knot-backend/appdata"
	"knot-backend/storage"
)

func TestRegistryCRUDPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	directory := sourceTestDirectory(t, "registry_crud")
	onlinePath := filepath.Join(directory.Path, "journal")
	if err := os.MkdirAll(onlinePath, 0o700); err != nil {
		t.Fatalf("create source directory: %v", err)
	}

	db, err := storage.Open(ctx, directory)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	registry := NewRegistry(NewRepository(db))

	created, err := registry.Create(ctx, Input{
		Name:        "工作日志",
		Kind:        KindJournal,
		Path:        onlinePath,
		ScopeType:   ScopeGlobal,
		LocalAccess: LocalAccessRead,
		AIAccess:    AIAccessContent,
	})
	if err != nil {
		t.Fatalf("create source root: %v", err)
	}
	if created.ID == "" || created.Availability != AvailabilityOnline {
		t.Fatalf("unexpected created root: %+v", created)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened, err := storage.Open(ctx, directory)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()
	registry = NewRegistry(NewRepository(reopened))

	roots, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("list roots: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != created.ID {
		t.Fatalf("expected persisted root %q, got %+v", created.ID, roots)
	}

	disabled := false
	recursive := false
	updated, err := registry.Update(ctx, created.ID, Input{
		Name:        "年度工作日志",
		Kind:        KindJournal,
		Path:        onlinePath,
		ScopeType:   ScopeGlobal,
		Enabled:     &disabled,
		Recursive:   &recursive,
		LocalAccess: LocalAccessRead,
		AIAccess:    AIAccessMetadata,
	})
	if err != nil {
		t.Fatalf("update source root: %v", err)
	}
	if updated.Name != "年度工作日志" || updated.Enabled || updated.Recursive {
		t.Fatalf("unexpected updated root: %+v", updated)
	}

	if err := registry.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete source root: %v", err)
	}
	if _, err := registry.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestRegistryAcceptsMissingSourceWithoutFailing(t *testing.T) {
	registry, closeDatabase := sourceTestRegistry(t, "missing_source")
	defer closeDatabase()

	missing := filepath.Join(sourceTestDirectory(t, "missing_path").Path, "offline-drive")
	root, err := registry.Create(context.Background(), Input{
		Name:        "离线资料",
		Kind:        KindExternalOffline,
		Path:        missing,
		ScopeType:   ScopeGlobal,
		LocalAccess: LocalAccessRead,
		AIAccess:    AIAccessNone,
	})
	if err != nil {
		t.Fatalf("missing source must be recorded: %v", err)
	}
	if root.Availability != AvailabilityMissing {
		t.Fatalf("expected missing availability, got %q", root.Availability)
	}
}

func TestRegistryRejectsRelativeAndDuplicatePaths(t *testing.T) {
	registry, closeDatabase := sourceTestRegistry(t, "path_validation")
	defer closeDatabase()

	if _, err := registry.Create(context.Background(), Input{
		Name: "Relative",
		Kind: KindReference,
		Path: filepath.Join("relative", "notes"),
	}); err == nil {
		t.Fatal("expected relative path to be rejected")
	}

	directory := sourceTestDirectory(t, "duplicate_source").Path
	_, err := registry.Create(context.Background(), Input{
		Name: "First",
		Kind: KindReference,
		Path: directory,
	})
	if err != nil {
		t.Fatalf("create first source: %v", err)
	}

	duplicatePath := directory
	if runtime.GOOS == "windows" {
		duplicatePath = strings.ToLower(filepath.ToSlash(directory))
	}
	_, err = registry.Create(context.Background(), Input{
		Name: "Second",
		Kind: KindJournal,
		Path: duplicatePath,
	})
	if !errors.Is(err, ErrPathConflict) {
		t.Fatalf("expected duplicate path conflict, got %v", err)
	}
}

func TestNormalizePathTreatsWindowsCaseAndSeparatorsAsEquivalent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path normalization")
	}

	normalizedA, keyA, err := NormalizePath(`C:\Workspace\Notes\Journal\`)
	if err != nil {
		t.Fatalf("normalize first path: %v", err)
	}
	normalizedB, keyB, err := NormalizePath(`c:/workspace/notes/journal`)
	if err != nil {
		t.Fatalf("normalize second path: %v", err)
	}
	if keyA != keyB {
		t.Fatalf("expected equal path keys, got %q and %q", keyA, keyB)
	}
	if filepath.Clean(normalizedA) == "" || filepath.Clean(normalizedB) == "" {
		t.Fatal("expected non-empty normalized paths")
	}
}

func TestLegacyImportIsIdempotentAndPreservesScopes(t *testing.T) {
	registry, closeDatabase := sourceTestRegistry(t, "legacy_import")
	defer closeDatabase()

	current := sourceTestDirectory(t, "legacy_current").Path
	departmentArchive := sourceTestDirectory(t, "legacy_department").Path
	projectArchive := sourceTestDirectory(t, "legacy_project").Path
	input := LegacyImportInput{
		FolderPath: current,
		ScanPath:   current,
		Departments: []LegacyOwner{
			{ID: "dept-1", Name: "办公室", ArchivePath: departmentArchive},
		},
		Projects: []LegacyOwner{
			{ID: "project-1", Name: "平台建设", ArchivePath: projectArchive},
		},
	}

	first, err := registry.ImportLegacy(context.Background(), input)
	if err != nil {
		t.Fatalf("first legacy import: %v", err)
	}
	if len(first.Imported) != 3 || len(first.Existing) != 1 {
		t.Fatalf("expected 3 imports and one in-request duplicate, got %+v", first)
	}

	second, err := registry.ImportLegacy(context.Background(), input)
	if err != nil {
		t.Fatalf("second legacy import: %v", err)
	}
	if len(second.Imported) != 0 || len(second.Existing) != 4 {
		t.Fatalf("expected idempotent second import, got %+v", second)
	}

	roots, err := registry.List(context.Background())
	if err != nil {
		t.Fatalf("list imported roots: %v", err)
	}
	if len(roots) != 3 {
		t.Fatalf("expected 3 unique roots, got %d", len(roots))
	}

	var departmentFound bool
	var projectFound bool
	for _, root := range roots {
		if root.ScopeType == ScopeDepartment &&
			root.ScopeID != nil && *root.ScopeID == "dept-1" &&
			root.ScopeName != nil && *root.ScopeName == "办公室" {
			departmentFound = true
		}
		if root.ScopeType == ScopeProject &&
			root.ScopeID != nil && *root.ScopeID == "project-1" &&
			root.ScopeName != nil && *root.ScopeName == "平台建设" {
			projectFound = true
		}
	}
	if !departmentFound || !projectFound {
		t.Fatalf("expected department and project scopes, got %+v", roots)
	}
}

func sourceTestRegistry(t *testing.T, name string) (*Registry, func()) {
	t.Helper()
	db, err := storage.Open(context.Background(), sourceTestDirectory(t, name))
	if err != nil {
		t.Fatalf("open source test database: %v", err)
	}
	return NewRegistry(NewRepository(db)), func() {
		_ = db.Close()
	}
}

func sourceTestDirectory(t *testing.T, name string) appdata.Directory {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	path, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage1",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve source test directory: %v", err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("reset source test directory: %v", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("create source test directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(path)
	})
	return appdata.Directory{Path: path, Source: appdata.SourceEnvironment}
}
