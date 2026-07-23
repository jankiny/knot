// Package safepath resolves registered source-root IDs and relative paths
// without allowing callers to supply or escape to arbitrary absolute paths.
package safepath

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"knot-backend/policy"
	"knot-backend/sources"
)

type SourceRootRegistry interface {
	Get(context.Context, string) (sources.SourceRoot, error)
	List(context.Context) ([]sources.SourceRoot, error)
}

type Request struct {
	SourceRootID string `json:"source_root_id"`
	RelativePath string `json:"relative_path"`
}

type Result struct {
	SourceRootID  string          `json:"source_root_id"`
	RelativePath  string          `json:"relative_path"`
	AbsolutePath  string          `json:"-"`
	CanonicalPath string          `json:"-"`
	PathKey       string          `json:"-"`
	Policy        policy.Decision `json:"policy"`
}

type ResolveError struct {
	Code policy.ReasonCode
	Err  error
}

func (err *ResolveError) Error() string {
	if err.Err == nil {
		return string(err.Code)
	}
	return fmt.Sprintf("%s: %v", err.Code, err.Err)
}

func (err *ResolveError) Unwrap() error {
	return err.Err
}

func Reason(err error) (policy.ReasonCode, bool) {
	var resolveError *ResolveError
	if errors.As(err, &resolveError) {
		return resolveError.Code, true
	}
	return "", false
}

type Resolver struct {
	registry SourceRootRegistry
}

func NewResolver(registry SourceRootRegistry) *Resolver {
	return &Resolver{registry: registry}
}

func (resolver *Resolver) Resolve(
	ctx context.Context,
	request Request,
) (Result, error) {
	relativePath, nativeRelativePath, err := normalizeRelativePath(request.RelativePath)
	if err != nil {
		return Result{}, outsideRootError(err)
	}

	sourceRootID := strings.TrimSpace(request.SourceRootID)
	if sourceRootID == "" {
		return Result{}, outsideRootError(errors.New("source_root_id is required"))
	}
	if resolver == nil || resolver.registry == nil {
		return Result{}, outsideRootError(errors.New("source root registry is unavailable"))
	}

	root, err := resolver.registry.Get(ctx, sourceRootID)
	if err != nil {
		if errors.Is(err, sources.ErrNotFound) {
			return Result{}, outsideRootError(errors.New("source root is not registered"))
		}
		return Result{}, outsideRootError(fmt.Errorf("look up source root: %w", err))
	}
	if !filepath.IsAbs(root.Path) {
		return Result{}, outsideRootError(errors.New("registered source root is not absolute"))
	}

	candidate := filepath.Clean(filepath.Join(root.Path, nativeRelativePath))
	if !pathWithinRoot(root.Path, candidate) {
		return Result{}, outsideRootError(errors.New("relative path escapes the source root"))
	}

	decision, err := policy.Evaluate(policy.Evaluation{
		Defaults: policy.Access{
			LocalAccess: policy.LocalAccessReadWrite,
			AIAccess:    policy.AIAccessContent,
		},
		Source: policy.Access{
			LocalAccess: root.LocalAccess,
			AIAccess:    root.AIAccess,
		},
		Path:         candidate,
		RootDisabled: !root.Enabled,
		RootOffline:  root.Availability != sources.AvailabilityOnline,
	})
	if err != nil {
		return Result{}, &ResolveError{
			Code: policy.ReasonLocalAccessDenied,
			Err:  fmt.Errorf("evaluate source root policy: %w", err),
		}
	}
	if !decision.AllowsLocalRead() {
		return Result{}, &ResolveError{
			Code: localDenialReason(decision),
			Err:  errors.New("source root does not allow local reads"),
		}
	}

	canonicalRoot, err := canonicalExistingPath(root.Path)
	if err != nil {
		return Result{}, &ResolveError{
			Code: policy.ReasonRootOffline,
			Err:  fmt.Errorf("resolve source root: %w", err),
		}
	}
	canonicalRoot = filepath.Clean(canonicalRoot)

	canonicalCandidate, err := resolveExistingPrefix(candidate)
	if err != nil {
		return Result{}, &ResolveError{
			Code: policy.ReasonSymlinkEscape,
			Err:  fmt.Errorf("resolve candidate path: %w", err),
		}
	}
	if !pathWithinRoot(canonicalRoot, canonicalCandidate) {
		return Result{}, &ResolveError{
			Code: policy.ReasonSymlinkEscape,
			Err:  errors.New("resolved path escapes the source root"),
		}
	}

	return Result{
		SourceRootID:  root.ID,
		RelativePath:  relativePath,
		AbsolutePath:  candidate,
		CanonicalPath: canonicalCandidate,
		PathKey:       normalizePathKey(canonicalCandidate),
		Policy:        decision,
	}, nil
}

// ResolveRegisteredAbsolute is a compatibility bridge for legacy handlers
// that still receive an absolute path. The path is accepted only when it can
// be expressed relative to a registered source root, then it is resolved
// through the same ID-based boundary and link checks as Resolve.
func (resolver *Resolver) ResolveRegisteredAbsolute(
	ctx context.Context,
	absolutePath string,
) (Result, error) {
	candidate := strings.TrimSpace(absolutePath)
	if candidate == "" || strings.ContainsRune(candidate, '\x00') ||
		!filepath.IsAbs(candidate) {
		return Result{}, outsideRootError(
			errors.New("legacy path must be a valid absolute path"),
		)
	}
	if resolver == nil || resolver.registry == nil {
		return Result{}, outsideRootError(errors.New("source root registry is unavailable"))
	}

	roots, err := resolver.registry.List(ctx)
	if err != nil {
		return Result{}, outsideRootError(fmt.Errorf("list source roots: %w", err))
	}

	candidate = filepath.Clean(candidate)
	var matchedRoot *sources.SourceRoot
	for index := range roots {
		root := &roots[index]
		if !pathWithinRoot(root.Path, candidate) {
			continue
		}
		if matchedRoot == nil || len(filepath.Clean(root.Path)) > len(filepath.Clean(matchedRoot.Path)) {
			matchedRoot = root
		}
	}
	if matchedRoot == nil {
		return Result{}, outsideRootError(
			errors.New("path is not inside a registered source root"),
		)
	}

	relativePath, err := filepath.Rel(matchedRoot.Path, candidate)
	if err != nil {
		return Result{}, outsideRootError(fmt.Errorf("make source-relative path: %w", err))
	}
	return resolver.Resolve(ctx, Request{
		SourceRootID: matchedRoot.ID,
		RelativePath: filepath.ToSlash(relativePath),
	})
}

func normalizeRelativePath(value string) (string, string, error) {
	candidate := strings.TrimSpace(value)
	if strings.ContainsRune(candidate, '\x00') {
		return "", "", errors.New("relative path contains an invalid null character")
	}
	if candidate == "" || candidate == "." {
		return ".", ".", nil
	}

	slashed := strings.ReplaceAll(candidate, `\`, "/")
	if strings.HasPrefix(slashed, "/") || hasWindowsVolumePrefix(slashed) {
		return "", "", errors.New("relative path must not be absolute or volume-relative")
	}

	cleaned := path.Clean(slashed)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", errors.New("relative path escapes the source root")
	}
	if cleaned == "." {
		return ".", ".", nil
	}
	return cleaned, filepath.FromSlash(cleaned), nil
}

func hasWindowsVolumePrefix(value string) bool {
	return len(value) >= 2 &&
		((value[0] >= 'a' && value[0] <= 'z') ||
			(value[0] >= 'A' && value[0] <= 'Z')) &&
		value[1] == ':'
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

// resolveExistingPrefix resolves links in every existing path component and
// safely appends a missing suffix. filepath.EvalSymlinks resolves Windows
// reparse points, including directory junctions, as well as symbolic links.
func resolveExistingPrefix(candidate string) (string, error) {
	current := filepath.Clean(candidate)
	missingParts := make([]string, 0)

	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, resolveErr := canonicalExistingPath(current)
			if resolveErr != nil {
				return "", resolveErr
			}
			for index := len(missingParts) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missingParts[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missingParts = append(missingParts, filepath.Base(current))
		current = parent
	}
}

func normalizePathKey(value string) string {
	normalized := filepath.ToSlash(filepath.Clean(value))
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func localDenialReason(decision policy.Decision) policy.ReasonCode {
	for _, code := range []policy.ReasonCode{
		policy.ReasonRootDisabled,
		policy.ReasonRootOffline,
		policy.ReasonLocalAccessDenied,
	} {
		if decision.HasReason(code) {
			return code
		}
	}
	return policy.ReasonLocalAccessDenied
}

func outsideRootError(err error) error {
	return &ResolveError{Code: policy.ReasonPathOutsideRoot, Err: err}
}
