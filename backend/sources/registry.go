package sources

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Registry struct {
	repository *Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewRegistry(repository *Repository) *Registry {
	return &Registry{
		repository: repository,
		now:        time.Now,
		newID:      newSourceRootID,
	}
}

func (r *Registry) List(ctx context.Context) ([]SourceRoot, error) {
	roots, err := r.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range roots {
		refreshed, err := r.refreshAvailability(ctx, roots[index])
		if err != nil {
			return nil, err
		}
		roots[index] = refreshed
	}
	return roots, nil
}

func (r *Registry) Get(ctx context.Context, id string) (SourceRoot, error) {
	root, err := r.repository.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return SourceRoot{}, err
	}
	return r.refreshAvailability(ctx, root)
}

func (r *Registry) Create(ctx context.Context, input Input) (SourceRoot, error) {
	root, err := r.prepare(input)
	if err != nil {
		return SourceRoot{}, err
	}
	root.ID, err = r.newID()
	if err != nil {
		return SourceRoot{}, fmt.Errorf("generate source root id: %w", err)
	}
	now := r.now().UTC().Format(time.RFC3339Nano)
	root.createdAt = now
	root.updatedAt = now

	if err := r.repository.Create(ctx, root); err != nil {
		return SourceRoot{}, err
	}
	return root, nil
}

func (r *Registry) Update(ctx context.Context, id string, input Input) (SourceRoot, error) {
	existing, err := r.repository.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return SourceRoot{}, err
	}
	root, err := r.prepare(input)
	if err != nil {
		return SourceRoot{}, err
	}
	root.ID = existing.ID
	root.createdAt = existing.createdAt
	root.updatedAt = r.now().UTC().Format(time.RFC3339Nano)

	if err := r.repository.Update(ctx, root); err != nil {
		return SourceRoot{}, err
	}
	return root, nil
}

func (r *Registry) Delete(ctx context.Context, id string) error {
	return r.repository.Delete(ctx, strings.TrimSpace(id))
}

func (r *Registry) ImportLegacy(
	ctx context.Context,
	input LegacyImportInput,
) (LegacyImportResult, error) {
	result := LegacyImportResult{
		Imported: make([]SourceRoot, 0),
		Existing: make([]SourceRoot, 0),
		Skipped:  make([]ImportSkip, 0),
	}

	candidates := buildLegacyCandidates(input)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.input.Path) == "" {
			continue
		}

		root, err := r.Create(ctx, candidate.input)
		if err == nil {
			result.Imported = append(result.Imported, root)
			continue
		}

		if errors.Is(err, ErrPathConflict) {
			_, pathKey, normalizeErr := NormalizePath(candidate.input.Path)
			if normalizeErr != nil {
				result.Skipped = append(result.Skipped, ImportSkip{
					Source: candidate.source,
					Path:   candidate.input.Path,
					Reason: normalizeErr.Error(),
				})
				continue
			}
			existing, findErr := r.repository.GetByPathKey(ctx, pathKey)
			if findErr != nil {
				return LegacyImportResult{}, findErr
			}
			result.Existing = append(result.Existing, existing)
			continue
		}

		var validationError *ValidationError
		if errors.As(err, &validationError) {
			result.Skipped = append(result.Skipped, ImportSkip{
				Source: candidate.source,
				Path:   candidate.input.Path,
				Reason: validationError.Error(),
			})
			continue
		}
		return LegacyImportResult{}, err
	}

	return result, nil
}

func (r *Registry) prepare(input Input) (SourceRoot, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return SourceRoot{}, &ValidationError{Field: "name", Message: "is required"}
	}

	if !validKind(input.Kind) {
		return SourceRoot{}, &ValidationError{Field: "kind", Message: "is not supported"}
	}

	path, pathKey, err := NormalizePath(input.Path)
	if err != nil {
		return SourceRoot{}, err
	}

	scopeType := input.ScopeType
	if scopeType == "" {
		scopeType = ScopeGlobal
	}
	if !validScopeType(scopeType) {
		return SourceRoot{}, &ValidationError{Field: "scope_type", Message: "is not supported"}
	}
	scopeID := trimOptionalString(input.ScopeID)
	scopeName := trimOptionalString(input.ScopeName)
	if scopeType == ScopeGlobal {
		scopeID = nil
		scopeName = nil
	}

	localAccess := input.LocalAccess
	if localAccess == "" {
		localAccess = LocalAccessRead
	}
	if !validLocalAccess(localAccess) {
		return SourceRoot{}, &ValidationError{Field: "local_access", Message: "is not supported"}
	}

	aiAccess := input.AIAccess
	if aiAccess == "" {
		if input.Kind == KindPrivate {
			aiAccess = AIAccessNone
		} else {
			aiAccess = AIAccessContent
		}
	}
	if !validAIAccess(aiAccess) {
		return SourceRoot{}, &ValidationError{Field: "ai_access", Message: "is not supported"}
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	recursive := true
	if input.Recursive != nil {
		recursive = *input.Recursive
	}

	return SourceRoot{
		Name:         name,
		Kind:         input.Kind,
		Path:         path,
		pathKey:      pathKey,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		ScopeName:    scopeName,
		Enabled:      enabled,
		Recursive:    recursive,
		LocalAccess:  localAccess,
		AIAccess:     aiAccess,
		Availability: DetectAvailability(path),
	}, nil
}

func (r *Registry) refreshAvailability(
	ctx context.Context,
	root SourceRoot,
) (SourceRoot, error) {
	availability := DetectAvailability(root.Path)
	if availability == root.Availability {
		return root, nil
	}
	root.Availability = availability
	root.updatedAt = r.now().UTC().Format(time.RFC3339Nano)
	if err := r.repository.UpdateAvailability(
		ctx,
		root.ID,
		root.Availability,
		root.updatedAt,
	); err != nil {
		return SourceRoot{}, err
	}
	return root, nil
}

type legacyCandidate struct {
	source string
	input  Input
}

func buildLegacyCandidates(input LegacyImportInput) []legacyCandidate {
	candidates := []legacyCandidate{
		{
			source: "folderPath",
			input: Input{
				Name:        "当前工作",
				Kind:        KindCurrentWork,
				Path:        input.FolderPath,
				ScopeType:   ScopeGlobal,
				LocalAccess: LocalAccessRead,
				AIAccess:    AIAccessContent,
			},
		},
		{
			source: "scanPath",
			input: Input{
				Name:        "扫描工作",
				Kind:        KindActiveWork,
				Path:        input.ScanPath,
				ScopeType:   ScopeGlobal,
				LocalAccess: LocalAccessRead,
				AIAccess:    AIAccessContent,
			},
		},
	}

	for _, department := range input.Departments {
		departmentID := strings.TrimSpace(department.ID)
		departmentName := strings.TrimSpace(department.Name)
		candidates = append(candidates, legacyCandidate{
			source: "departments",
			input: Input{
				Name:        legacyArchiveName(departmentName),
				Kind:        KindWorkArchive,
				Path:        department.ArchivePath,
				ScopeType:   ScopeDepartment,
				ScopeID:     optionalString(departmentID),
				ScopeName:   optionalString(departmentName),
				LocalAccess: LocalAccessRead,
				AIAccess:    AIAccessContent,
			},
		})
	}

	for _, project := range input.Projects {
		projectID := strings.TrimSpace(project.ID)
		projectName := strings.TrimSpace(project.Name)
		candidates = append(candidates, legacyCandidate{
			source: "projects",
			input: Input{
				Name:        legacyArchiveName(projectName),
				Kind:        KindWorkArchive,
				Path:        project.ArchivePath,
				ScopeType:   ScopeProject,
				ScopeID:     optionalString(projectID),
				ScopeName:   optionalString(projectName),
				LocalAccess: LocalAccessRead,
				AIAccess:    AIAccessContent,
			},
		})
	}
	return candidates
}

func legacyArchiveName(name string) string {
	if name == "" {
		return "工作归档"
	}
	return name + "归档"
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	return optionalString(strings.TrimSpace(*value))
}

func newSourceRootID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "root_" + hex.EncodeToString(value), nil
}

func validKind(value Kind) bool {
	switch value {
	case KindCurrentWork,
		KindActiveWork,
		KindWorkArchive,
		KindJournal,
		KindReference,
		KindPrivate,
		KindExternalOffline:
		return true
	default:
		return false
	}
}

func validScopeType(value ScopeType) bool {
	switch value {
	case ScopeGlobal, ScopeDepartment, ScopeProject:
		return true
	default:
		return false
	}
}

func validLocalAccess(value LocalAccess) bool {
	return value.Valid()
}

func validAIAccess(value AIAccess) bool {
	return value.Valid()
}
