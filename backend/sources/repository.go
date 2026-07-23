package sources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]SourceRoot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, name, kind, path, path_key,
			scope_type, scope_id, scope_name,
			enabled, recursive, local_access, ai_access, availability,
			created_at, updated_at
		FROM source_roots
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list source roots: %w", err)
	}
	defer rows.Close()

	roots := make([]SourceRoot, 0)
	for rows.Next() {
		root, err := scanSourceRoot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan source root: %w", err)
		}
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source roots: %w", err)
	}
	return roots, nil
}

func (r *Repository) Get(ctx context.Context, id string) (SourceRoot, error) {
	root, err := scanSourceRoot(r.db.QueryRowContext(ctx, `
		SELECT
			id, name, kind, path, path_key,
			scope_type, scope_id, scope_name,
			enabled, recursive, local_access, ai_access, availability,
			created_at, updated_at
		FROM source_roots
		WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return SourceRoot{}, ErrNotFound
	}
	if err != nil {
		return SourceRoot{}, fmt.Errorf("get source root: %w", err)
	}
	return root, nil
}

func (r *Repository) GetByPathKey(ctx context.Context, pathKey string) (SourceRoot, error) {
	root, err := scanSourceRoot(r.db.QueryRowContext(ctx, `
		SELECT
			id, name, kind, path, path_key,
			scope_type, scope_id, scope_name,
			enabled, recursive, local_access, ai_access, availability,
			created_at, updated_at
		FROM source_roots
		WHERE path_key = ?
	`, pathKey))
	if errors.Is(err, sql.ErrNoRows) {
		return SourceRoot{}, ErrNotFound
	}
	if err != nil {
		return SourceRoot{}, fmt.Errorf("get source root by path: %w", err)
	}
	return root, nil
}

func (r *Repository) Create(ctx context.Context, root SourceRoot) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO source_roots (
			id, name, kind, path, path_key,
			scope_type, scope_id, scope_name,
			enabled, recursive, local_access, ai_access, availability,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		root.ID,
		root.Name,
		root.Kind,
		root.Path,
		root.pathKey,
		root.ScopeType,
		root.ScopeID,
		root.ScopeName,
		root.Enabled,
		root.Recursive,
		root.LocalAccess,
		root.AIAccess,
		root.Availability,
		root.createdAt,
		root.updatedAt,
	)
	if isUniquePathError(err) {
		return ErrPathConflict
	}
	if err != nil {
		return fmt.Errorf("create source root: %w", err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, root SourceRoot) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE source_roots
		SET
			name = ?,
			kind = ?,
			path = ?,
			path_key = ?,
			scope_type = ?,
			scope_id = ?,
			scope_name = ?,
			enabled = ?,
			recursive = ?,
			local_access = ?,
			ai_access = ?,
			availability = ?,
			updated_at = ?
		WHERE id = ?
	`,
		root.Name,
		root.Kind,
		root.Path,
		root.pathKey,
		root.ScopeType,
		root.ScopeID,
		root.ScopeName,
		root.Enabled,
		root.Recursive,
		root.LocalAccess,
		root.AIAccess,
		root.Availability,
		root.updatedAt,
		root.ID,
	)
	if isUniquePathError(err) {
		return ErrPathConflict
	}
	if err != nil {
		return fmt.Errorf("update source root: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated source root count: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateAvailability(
	ctx context.Context,
	id string,
	availability Availability,
	updatedAt string,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE source_roots
		SET availability = ?, updated_at = ?
		WHERE id = ?
	`, availability, updatedAt, id)
	if err != nil {
		return fmt.Errorf("update source root availability: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read availability update count: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM source_roots WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete source root: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted source root count: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSourceRoot(scanner rowScanner) (SourceRoot, error) {
	var root SourceRoot
	var scopeID sql.NullString
	var scopeName sql.NullString
	var enabled int
	var recursive int

	err := scanner.Scan(
		&root.ID,
		&root.Name,
		&root.Kind,
		&root.Path,
		&root.pathKey,
		&root.ScopeType,
		&scopeID,
		&scopeName,
		&enabled,
		&recursive,
		&root.LocalAccess,
		&root.AIAccess,
		&root.Availability,
		&root.createdAt,
		&root.updatedAt,
	)
	if err != nil {
		return SourceRoot{}, err
	}
	root.Enabled = enabled != 0
	root.Recursive = recursive != 0
	if scopeID.Valid {
		root.ScopeID = &scopeID.String
	}
	if scopeName.Valid {
		root.ScopeName = &scopeName.String
	}
	return root, nil
}

func isUniquePathError(err error) bool {
	return err != nil && strings.Contains(
		strings.ToLower(err.Error()),
		"unique constraint failed: source_roots.path_key",
	)
}
