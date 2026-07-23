package indexer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"knot-backend/extractors"
	"knot-backend/policy"
	"knot-backend/safepath"
	"knot-backend/sources"
)

var errScanLimit = errors.New("scan limit reached")

type sourceUnavailableError struct {
	reason string
	err    error
}

func (scanError *sourceUnavailableError) Error() string {
	if scanError.err != nil {
		return scanError.err.Error()
	}
	return scanError.reason
}

func (scanError *sourceUnavailableError) Unwrap() error {
	return scanError.err
}

var ignoredDirectoryNames = map[string]struct{}{
	".cache":        {},
	".git":          {},
	".mypy_cache":   {},
	".next":         {},
	".nuxt":         {},
	".output":       {},
	".pytest_cache": {},
	".ruff_cache":   {},
	".turbo":        {},
	".venv":         {},
	".vite":         {},
	"__pycache__":   {},
	"build":         {},
	"coverage":      {},
	"dist":          {},
	"node_modules":  {},
	"target":        {},
	"venv":          {},
}

type Scanner struct {
	registry      *sources.Registry
	resolver      *safepath.Resolver
	repository    *Repository
	adapters      []Adapter
	limits        Limits
	now           func() time.Time
	newDocumentID func() (string, error)
	newScanID     func() (string, error)
}

func NewScanner(
	registry *sources.Registry,
	resolver *safepath.Resolver,
	repository *Repository,
	adapters ...Adapter,
) *Scanner {
	if len(adapters) == 0 {
		adapters = []Adapter{
			NewMarkdownAdapter(),
			NewTextAdapter(),
			NewFileAdapter(),
		}
	}
	return &Scanner{
		registry:   registry,
		resolver:   resolver,
		repository: repository,
		adapters:   adapters,
		limits:     DefaultLimits(),
		now:        time.Now,
		newDocumentID: func() (string, error) {
			return newIdentifier("doc")
		},
		newScanID: func() (string, error) {
			return newIdentifier("scan")
		},
	}
}

func (scanner *Scanner) ScanSource(
	ctx context.Context,
	sourceRootID string,
	options ScanOptions,
) (ScanResult, error) {
	if err := scanner.validate(); err != nil {
		return ScanResult{}, err
	}
	scanContext, cancel := context.WithTimeout(ctx, scanner.limits.MaxDuration)
	defer cancel()

	root, err := scanner.registry.Get(scanContext, strings.TrimSpace(sourceRootID))
	if err != nil {
		return ScanResult{}, err
	}
	budget := &scanBudget{limits: scanner.limits}
	return scanner.scanRoot(scanContext, root, options, budget)
}

func (scanner *Scanner) ScanAll(
	ctx context.Context,
	options ScanOptions,
) (ScanAllResult, error) {
	if err := scanner.validate(); err != nil {
		return ScanAllResult{}, err
	}
	scanContext, cancel := context.WithTimeout(ctx, scanner.limits.MaxDuration)
	defer cancel()

	roots, err := scanner.registry.List(scanContext)
	if err != nil {
		return ScanAllResult{}, err
	}

	result := ScanAllResult{
		Status:  ScanStatusCompleted,
		Results: make([]ScanResult, 0, len(roots)),
	}
	budget := &scanBudget{limits: scanner.limits}
	for _, root := range roots {
		if !root.Enabled {
			continue
		}
		rootResult, scanErr := scanner.scanRoot(scanContext, root, options, budget)
		result.Results = append(result.Results, rootResult)
		result.SourceRoots++
		if rootResult.Status != ScanStatusCompleted {
			result.Status = ScanStatusPartial
		}
		if scanErr != nil {
			if errors.Is(scanErr, context.Canceled) ||
				errors.Is(scanErr, context.DeadlineExceeded) {
				return result, scanErr
			}
			return result, scanErr
		}
		if rootResult.Status == ScanStatusPartial {
			break
		}
	}
	return result, nil
}

func (scanner *Scanner) scanRoot(
	ctx context.Context,
	root sources.SourceRoot,
	options ScanOptions,
	budget *scanBudget,
) (ScanResult, error) {
	startedAt := scanner.now().UTC()
	result := ScanResult{
		SourceRootID: root.ID,
		Status:       ScanStatusCompleted,
		StartedAt:    startedAt.Format(time.RFC3339Nano),
		Issues:       make([]ScanIssue, 0),
	}
	finish := func() {
		result.FinishedAt = scanner.now().UTC().Format(time.RFC3339Nano)
	}

	scanID, err := scanner.newScanID()
	if err != nil {
		finish()
		return result, fmt.Errorf("generate index scan id: %w", err)
	}

	rootResolution, err := scanner.resolver.Resolve(ctx, safepath.Request{
		SourceRootID: root.ID,
		RelativePath: ".",
	})
	if err != nil {
		reason := reasonFromError(err)
		clearExcerpt := reason == string(policy.ReasonRootDisabled) ||
			reason == string(policy.ReasonLocalAccessDenied)
		if _, staleErr := scanner.repository.MarkSourceStale(
			ctx,
			root.ID,
			reason,
			clearExcerpt,
			scanner.now().UTC().Format(time.RFC3339Nano),
		); staleErr != nil {
			finish()
			return result, staleErr
		}
		result.Status = ScanStatusUnavailable
		result.addIssue(".", reason, scanner.limits.MaxReportedIssues)
		finish()
		return result, nil
	}

	walkErr := filepath.WalkDir(
		rootResolution.AbsolutePath,
		func(filePath string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				relativePath := relativePathForIssue(root.Path, filePath)
				result.addIssue(
					relativePath,
					ReasonReadError,
					scanner.limits.MaxReportedIssues,
				)
				if normalizeFilesystemPath(filePath) ==
					normalizeFilesystemPath(rootResolution.AbsolutePath) {
					return &sourceUnavailableError{
						reason: ReasonReadError,
						err:    walkErr,
					}
				}
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			relativePath, err := filepath.Rel(root.Path, filePath)
			if err != nil {
				result.addIssue(
					".",
					string(policy.ReasonPathOutsideRoot),
					scanner.limits.MaxReportedIssues,
				)
				return nil
			}
			relativePath = filepath.ToSlash(relativePath)
			if relativePath == "." {
				return nil
			}

			if entry.IsDir() {
				if isIgnoredDirectory(entry.Name()) {
					result.IgnoredDirectories++
					return filepath.SkipDir
				}
				if !root.Recursive {
					return filepath.SkipDir
				}
				if _, err := scanner.resolver.Resolve(ctx, safepath.Request{
					SourceRootID: root.ID,
					RelativePath: relativePath,
				}); err != nil {
					reason := reasonFromError(err)
					if isSourceUnavailableReason(reason) {
						return &sourceUnavailableError{
							reason: reason,
							err:    err,
						}
					}
					result.addIssue(
						relativePath,
						reason,
						scanner.limits.MaxReportedIssues,
					)
					return filepath.SkipDir
				}
				return nil
			}

			if err := budget.acquireFile(); err != nil {
				return err
			}
			result.FilesSeen++

			resolved, err := scanner.resolver.Resolve(ctx, safepath.Request{
				SourceRootID: root.ID,
				RelativePath: relativePath,
			})
			if err != nil {
				reason := reasonFromError(err)
				if isSourceUnavailableReason(reason) {
					return &sourceUnavailableError{
						reason: reason,
						err:    err,
					}
				}
				result.addIssue(
					relativePath,
					reason,
					scanner.limits.MaxReportedIssues,
				)
				info, infoErr := entry.Info()
				if infoErr == nil {
					if saveErr := scanner.saveFailure(
						ctx,
						root,
						relativePath,
						info,
						scanID,
						reason,
						nil,
					); saveErr != nil {
						return saveErr
					}
				}
				return nil
			}

			info, err := os.Stat(resolved.CanonicalPath)
			if err != nil {
				result.addIssue(
					relativePath,
					ReasonReadError,
					scanner.limits.MaxReportedIssues,
				)
				entryInfo, infoErr := entry.Info()
				if infoErr == nil {
					if saveErr := scanner.saveFailure(
						ctx,
						root,
						relativePath,
						entryInfo,
						scanID,
						ReasonReadError,
						&resolved.Policy,
					); saveErr != nil {
						return saveErr
					}
				}
				return nil
			}
			if info.IsDir() {
				return nil
			}

			adapter := scanner.adapterFor(relativePath)
			if adapter == nil {
				return fmt.Errorf("no source adapter accepts %q", relativePath)
			}
			relativePathKey := normalizeRelativePathKey(relativePath)
			existing, existingErr := scanner.repository.GetByPathKey(
				ctx,
				root.ID,
				relativePathKey,
			)
			if existingErr != nil && !errors.Is(existingErr, ErrDocumentNotFound) {
				return existingErr
			}
			hasExisting := existingErr == nil

			currentDecision := resolved.Policy
			if hasExisting && existing.documentAIAccess != nil {
				currentDecision, err = restrictPolicy(
					resolved.Policy,
					existing.documentAIAccess,
					resolved.AbsolutePath,
				)
				if err != nil {
					return err
				}
			}

			modifiedAt := info.ModTime().UTC().Format(time.RFC3339Nano)
			if !options.Force &&
				hasExisting &&
				existing.IndexStatus == IndexStatusReady &&
				existing.ModifiedAt == modifiedAt &&
				existing.FileSize == info.Size() &&
				existing.adapterName == adapter.Name() &&
				existing.adapterVersion == adapter.Version() &&
				existing.AIAccessEffective == currentDecision.AIAccess {
				existing.RelativePath = relativePath
				existing.ModifiedAt = modifiedAt
				existing.FileSize = info.Size()
				existing.AIAccessEffective = currentDecision.AIAccess
				existing.PolicyReasons = currentDecision.Reasons
				if err := scanner.repository.Touch(
					ctx,
					existing,
					scanID,
					scanner.now().UTC().Format(time.RFC3339Nano),
				); err != nil {
					return err
				}
				result.Unchanged++
				if !currentDecision.AllowsAIContent() {
					result.MetadataOnly++
				}
				return nil
			}

			extraction := metadataExtraction(relativePath, adapter)
			decision := currentDecision
			lastErrorCode := ""
			shouldRead := resolved.Policy.AllowsAIContent() ||
				(adapter.ReadsContentForMetadata() &&
					resolved.Policy.AllowsAIMetadata())

			if info.Size() > scanner.limits.MaxFileBytes {
				lastErrorCode = string(policy.ReasonSizeLimit)
				if adapter.Name() == "work_record" {
					deny := policy.AIAccessNone
					decision, err = restrictPolicy(
						resolved.Policy,
						&deny,
						resolved.AbsolutePath,
					)
					if err != nil {
						return err
					}
					extraction.DeclaredAIAccess = string(policy.AIAccessNone)
				}
				result.addIssue(
					relativePath,
					string(policy.ReasonSizeLimit),
					scanner.limits.MaxReportedIssues,
				)
			} else if shouldRead {
				if err := budget.acquireBytes(info.Size()); err != nil {
					return err
				}
				result.BytesRead += info.Size()
				extraction, err = adapter.Extract(ctx, AdapterInput{
					SourceRoot:      root,
					RelativePath:    relativePath,
					AbsolutePath:    resolved.CanonicalPath,
					FileInfo:        info,
					MaxExcerptRunes: scanner.limits.MaxExcerptRunes,
				})
				if err != nil {
					if errors.Is(err, context.Canceled) ||
						errors.Is(err, context.DeadlineExceeded) {
						return err
					}
					reason := adapterFailureReason(err)
					result.addIssue(
						relativePath,
						reason,
						scanner.limits.MaxReportedIssues,
					)
					return scanner.saveFailure(
						ctx,
						root,
						relativePath,
						info,
						scanID,
						reason,
						&resolved.Policy,
					)
				}

				var declaredAccess *policy.AIAccess
				if strings.TrimSpace(extraction.DeclaredAIAccess) != "" {
					access, present, parseErr := policy.ParseLegacyAIAccess(
						extraction.DeclaredAIAccess,
					)
					if parseErr != nil {
						access = policy.AIAccessNone
						present = true
						lastErrorCode = string(policy.ReasonAIAccessDenied)
						result.addIssue(
							relativePath,
							string(policy.ReasonAIAccessDenied),
							scanner.limits.MaxReportedIssues,
						)
					}
					if present {
						declaredAccess = &access
					}
				}
				if declaredAccess != nil {
					decision, err = restrictPolicy(
						resolved.Policy,
						declaredAccess,
						resolved.AbsolutePath,
					)
					if err != nil {
						return err
					}
				}
			} else {
				if err := budget.acquireBytes(info.Size()); err != nil {
					return err
				}
				result.BytesRead += info.Size()
				data, readErr := readBoundedFile(ctx, resolved.CanonicalPath)
				if readErr != nil {
					if errors.Is(readErr, context.Canceled) ||
						errors.Is(readErr, context.DeadlineExceeded) {
						return readErr
					}
					reason := adapterFailureReason(readErr)
					result.addIssue(
						relativePath,
						reason,
						scanner.limits.MaxReportedIssues,
					)
					return scanner.saveFailure(
						ctx,
						root,
						relativePath,
						info,
						scanID,
						reason,
						&resolved.Policy,
					)
				}
				extraction.ContentHash = contentHash(data)
			}

			if !decision.AllowsAIContent() {
				extraction.ContentExcerpt = ""
				result.MetadataOnly++
			}
			document, err := scanner.buildDocument(
				root,
				relativePath,
				relativePathKey,
				info,
				extraction,
				decision,
				adapter,
				scanID,
				lastErrorCode,
				existing,
				hasExisting,
			)
			if err != nil {
				return err
			}
			if err := scanner.repository.Save(ctx, document); err != nil {
				return err
			}
			if hasExisting {
				result.Updated++
			} else {
				result.Indexed++
			}
			return nil
		},
	)

	switch {
	case walkErr == nil:
		deleted, err := scanner.repository.MarkMissingDeleted(
			ctx,
			root.ID,
			scanID,
			scanner.now().UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			finish()
			return result, err
		}
		result.Deleted = deleted
	case errors.Is(walkErr, errScanLimit):
		result.Status = ScanStatusPartial
		result.addIssue(".", ReasonScanLimit, scanner.limits.MaxReportedIssues)
	case isSourceUnavailableError(walkErr):
		var unavailableError *sourceUnavailableError
		_ = errors.As(walkErr, &unavailableError)
		clearExcerpt := unavailableError.reason ==
			string(policy.ReasonRootDisabled) ||
			unavailableError.reason == string(policy.ReasonLocalAccessDenied)
		if _, err := scanner.repository.MarkSourceStale(
			ctx,
			root.ID,
			unavailableError.reason,
			clearExcerpt,
			scanner.now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			finish()
			return result, err
		}
		result.Status = ScanStatusUnavailable
		result.addIssue(
			".",
			unavailableError.reason,
			scanner.limits.MaxReportedIssues,
		)
	case errors.Is(walkErr, context.Canceled),
		errors.Is(walkErr, context.DeadlineExceeded):
		result.Status = ScanStatusPartial
		result.addIssue(".", ReasonCancelled, scanner.limits.MaxReportedIssues)
		finish()
		return result, walkErr
	default:
		finish()
		return result, fmt.Errorf("walk source root: %w", walkErr)
	}

	finish()
	return result, nil
}

func (scanner *Scanner) buildDocument(
	root sources.SourceRoot,
	relativePath string,
	relativePathKey string,
	info fs.FileInfo,
	extraction Extraction,
	decision policy.Decision,
	adapter Adapter,
	scanID string,
	lastErrorCode string,
	existing IndexedDocument,
	hasExisting bool,
) (IndexedDocument, error) {
	now := scanner.now().UTC().Format(time.RFC3339Nano)
	documentID := existing.ID
	createdAt := existing.createdAt
	if !hasExisting {
		var err error
		documentID, err = scanner.newDocumentID()
		if err != nil {
			return IndexedDocument{}, fmt.Errorf("generate document id: %w", err)
		}
		createdAt = now
	}

	var projectID *string
	if root.ScopeType == sources.ScopeProject && root.ScopeID != nil {
		value := *root.ScopeID
		projectID = &value
	}
	var declaredAccess *policy.AIAccess
	if strings.TrimSpace(extraction.DeclaredAIAccess) != "" {
		access, present, parseErr := policy.ParseLegacyAIAccess(
			extraction.DeclaredAIAccess,
		)
		if parseErr != nil {
			access = policy.AIAccessNone
			present = true
		}
		if present {
			declaredAccess = &access
		}
	}

	return IndexedDocument{
		ID:                documentID,
		SourceRootID:      root.ID,
		RelativePath:      relativePath,
		DocumentType:      extraction.DocumentType,
		Title:             extraction.Title,
		TaskDate:          extraction.TaskDate,
		ModifiedAt:        info.ModTime().UTC().Format(time.RFC3339Nano),
		FileSize:          info.Size(),
		ProjectID:         projectID,
		ContentHash:       extraction.ContentHash,
		ContentExcerpt:    extraction.ContentExcerpt,
		IndexStatus:       IndexStatusReady,
		AIAccessEffective: decision.AIAccess,
		PolicyReasons:     decision.Reasons,
		LastErrorCode:     lastErrorCode,
		relativePathKey:   relativePathKey,
		documentAIAccess:  declaredAccess,
		adapterName:       adapter.Name(),
		adapterVersion:    adapter.Version(),
		lastSeenScanID:    scanID,
		createdAt:         createdAt,
		updatedAt:         now,
	}, nil
}

func (scanner *Scanner) saveFailure(
	ctx context.Context,
	root sources.SourceRoot,
	relativePath string,
	info fs.FileInfo,
	scanID string,
	reason string,
	baseDecision *policy.Decision,
) error {
	adapter := scanner.adapterFor(relativePath)
	if adapter == nil {
		adapter = NewFileAdapter()
	}
	relativePathKey := normalizeRelativePathKey(relativePath)
	existing, err := scanner.repository.GetByPathKey(
		ctx,
		root.ID,
		relativePathKey,
	)
	hasExisting := err == nil
	if err != nil && !errors.Is(err, ErrDocumentNotFound) {
		return err
	}

	now := scanner.now().UTC().Format(time.RFC3339Nano)
	documentID := existing.ID
	createdAt := existing.createdAt
	if !hasExisting {
		documentID, err = scanner.newDocumentID()
		if err != nil {
			return fmt.Errorf("generate failed document id: %w", err)
		}
		createdAt = now
	}
	reasons := []policy.ReasonCode{policy.ReasonCode(reason)}
	if baseDecision != nil && len(baseDecision.Reasons) > 0 {
		reasons = append([]policy.ReasonCode{}, baseDecision.Reasons...)
		if !containsReason(reasons, policy.ReasonCode(reason)) {
			reasons = append(reasons, policy.ReasonCode(reason))
		}
	}

	var projectID *string
	if root.ScopeType == sources.ScopeProject && root.ScopeID != nil {
		value := *root.ScopeID
		projectID = &value
	}
	return scanner.repository.Save(ctx, IndexedDocument{
		ID:                documentID,
		SourceRootID:      root.ID,
		RelativePath:      relativePath,
		DocumentType:      documentTypeForAdapter(adapter),
		Title:             fallbackTitle("", relativePath),
		ModifiedAt:        info.ModTime().UTC().Format(time.RFC3339Nano),
		FileSize:          info.Size(),
		ProjectID:         projectID,
		IndexStatus:       IndexStatusError,
		AIAccessEffective: policy.AIAccessNone,
		PolicyReasons:     reasons,
		LastErrorCode:     reason,
		relativePathKey:   relativePathKey,
		adapterName:       adapter.Name(),
		adapterVersion:    adapter.Version(),
		lastSeenScanID:    scanID,
		createdAt:         createdAt,
		updatedAt:         now,
	})
}

func (scanner *Scanner) adapterFor(relativePath string) Adapter {
	for _, adapter := range scanner.adapters {
		if adapter != nil && adapter.Matches(relativePath) {
			return adapter
		}
	}
	return nil
}

func (scanner *Scanner) validate() error {
	switch {
	case scanner == nil:
		return errors.New("index scanner is unavailable")
	case scanner.registry == nil:
		return errors.New("source registry is unavailable")
	case scanner.resolver == nil:
		return errors.New("safe path resolver is unavailable")
	case scanner.repository == nil || scanner.repository.db == nil:
		return errors.New("index repository is unavailable")
	case scanner.limits.MaxFiles <= 0,
		scanner.limits.MaxTotalBytes <= 0,
		scanner.limits.MaxFileBytes <= 0,
		scanner.limits.MaxDuration <= 0,
		scanner.limits.MaxExcerptRunes <= 0,
		scanner.limits.MaxReportedIssues <= 0:
		return errors.New("index scan limits must all be positive")
	default:
		return nil
	}
}

func restrictPolicy(
	base policy.Decision,
	documentAccess *policy.AIAccess,
	filePath string,
) (policy.Decision, error) {
	access := policy.Access{
		LocalAccess: base.LocalAccess,
		AIAccess:    base.AIAccess,
	}
	return policy.Evaluate(policy.Evaluation{
		Defaults: access,
		Source:   access,
		Document: &policy.Override{AIAccess: documentAccess},
		Path:     filePath,
	})
}

func metadataExtraction(relativePath string, adapter Adapter) Extraction {
	return Extraction{
		DocumentType: documentTypeForAdapter(adapter),
		Title:        fallbackTitle("", relativePath),
		TaskDate:     extractors.NormalizeDate(relativePath),
	}
}

func documentTypeForAdapter(adapter Adapter) string {
	switch adapter.Name() {
	case "work_record":
		return DocumentTypeWorkRecord
	case "markdown":
		return extractors.DocumentTypeMarkdown
	case "text":
		return DocumentTypeText
	default:
		return DocumentTypeFile
	}
}

func reasonFromError(err error) string {
	if reason, ok := safepath.Reason(err); ok {
		return string(reason)
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return ReasonCancelled
	}
	return ReasonReadError
}

func adapterFailureReason(err error) string {
	var extractionError *adapterError
	if errors.As(err, &extractionError) {
		return extractionError.code
	}
	return ReasonReadError
}

func (result *ScanResult) addIssue(
	relativePath string,
	reason string,
	limit int,
) {
	if len(result.Issues) >= limit {
		result.OmittedIssues++
		return
	}
	result.Issues = append(result.Issues, ScanIssue{
		RelativePath: relativePath,
		Reason:       reason,
	})
}

type scanBudget struct {
	limits Limits
	files  int
	bytes  int64
}

func (budget *scanBudget) acquireFile() error {
	if budget.files >= budget.limits.MaxFiles {
		return errScanLimit
	}
	budget.files++
	return nil
}

func (budget *scanBudget) acquireBytes(count int64) error {
	if count < 0 || count > budget.limits.MaxTotalBytes-budget.bytes {
		return errScanLimit
	}
	budget.bytes += count
	return nil
}

func isIgnoredDirectory(name string) bool {
	_, ignored := ignoredDirectoryNames[strings.ToLower(strings.TrimSpace(name))]
	return ignored
}

func isSourceUnavailableReason(reason string) bool {
	switch policy.ReasonCode(reason) {
	case policy.ReasonRootDisabled,
		policy.ReasonRootOffline,
		policy.ReasonLocalAccessDenied:
		return true
	default:
		return reason == ReasonReadError
	}
}

func isSourceUnavailableError(err error) bool {
	var unavailableError *sourceUnavailableError
	return errors.As(err, &unavailableError)
}

func normalizeFilesystemPath(value string) string {
	normalized := filepath.Clean(value)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func normalizeRelativePathKey(value string) string {
	key := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

func relativePathForIssue(rootPath, filePath string) string {
	relative, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return "."
	}
	return filepath.ToSlash(relative)
}

func containsReason(reasons []policy.ReasonCode, expected policy.ReasonCode) bool {
	for _, reason := range reasons {
		if reason == expected {
			return true
		}
	}
	return false
}

func newIdentifier(prefix string) (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(value), nil
}
