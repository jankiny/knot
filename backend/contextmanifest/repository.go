package contextmanifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) Save(
	ctx context.Context,
	manifest ContextManifest,
	snapshotHash string,
	snapshots []documentSnapshot,
) error {
	payload, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode context manifest: %w", err)
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin context manifest save: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO context_manifests (
			id, task_type, query, period_start, period_end,
			manifest_json, snapshot_hash, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		manifest.ID,
		manifest.TaskType,
		manifest.Query,
		manifest.PeriodStart,
		manifest.PeriodEnd,
		string(payload),
		snapshotHash,
		manifest.CreatedAt,
		manifest.ExpiresAt,
	); err != nil {
		return fmt.Errorf("save context manifest: %w", err)
	}

	for _, snapshot := range snapshots {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO context_manifest_documents (
				manifest_id, document_id, source_root_id, relative_path,
				content_hash, modified_at, file_size, ai_access_effective
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			manifest.ID,
			snapshot.DocumentID,
			snapshot.SourceRootID,
			snapshot.RelativePath,
			snapshot.ContentHash,
			snapshot.ModifiedAt,
			snapshot.FileSize,
			snapshot.AIAccessEffective,
		); err != nil {
			return fmt.Errorf("save context document snapshot: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit context manifest save: %w", err)
	}
	return nil
}

func (repository *Repository) Get(
	ctx context.Context,
	manifestID string,
) (ContextManifest, string, []documentSnapshot, error) {
	var payload string
	var snapshotHash string
	err := repository.db.QueryRowContext(ctx, `
		SELECT manifest_json, snapshot_hash
		FROM context_manifests
		WHERE id = ?
	`, manifestID).Scan(&payload, &snapshotHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ContextManifest{}, "", nil, ErrManifestNotFound
	}
	if err != nil {
		return ContextManifest{}, "", nil, fmt.Errorf(
			"get context manifest: %w",
			err,
		)
	}

	var manifest ContextManifest
	if err := json.Unmarshal([]byte(payload), &manifest); err != nil {
		return ContextManifest{}, "", nil, fmt.Errorf(
			"decode context manifest: %w",
			err,
		)
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT
			document_id, source_root_id, relative_path, content_hash,
			modified_at, file_size, ai_access_effective
		FROM context_manifest_documents
		WHERE manifest_id = ?
		ORDER BY document_id
	`, manifestID)
	if err != nil {
		return ContextManifest{}, "", nil, fmt.Errorf(
			"list context document snapshots: %w",
			err,
		)
	}
	defer rows.Close()

	snapshots := make([]documentSnapshot, 0)
	for rows.Next() {
		var snapshot documentSnapshot
		if err := rows.Scan(
			&snapshot.DocumentID,
			&snapshot.SourceRootID,
			&snapshot.RelativePath,
			&snapshot.ContentHash,
			&snapshot.ModifiedAt,
			&snapshot.FileSize,
			&snapshot.AIAccessEffective,
		); err != nil {
			return ContextManifest{}, "", nil, fmt.Errorf(
				"scan context document snapshot: %w",
				err,
			)
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return ContextManifest{}, "", nil, fmt.Errorf(
			"iterate context document snapshots: %w",
			err,
		)
	}
	return manifest, snapshotHash, snapshots, nil
}
