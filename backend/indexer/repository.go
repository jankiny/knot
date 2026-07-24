package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"knot-backend/policy"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) GetByPathKey(
	ctx context.Context,
	sourceRootID string,
	relativePathKey string,
) (IndexedDocument, error) {
	document, err := scanIndexedDocument(repository.db.QueryRowContext(ctx, `
		SELECT
			id, source_root_id, relative_path, relative_path_key,
			document_type, title, task_date, modified_at, file_size,
			project_id, content_hash, content_excerpt, index_status,
			ai_access_effective, document_ai_access, policy_reasons,
			adapter_name, adapter_version, last_error_code,
			last_seen_scan_id, created_at, updated_at
		FROM indexed_documents
		WHERE source_root_id = ? AND relative_path_key = ?
	`, sourceRootID, relativePathKey))
	if errors.Is(err, sql.ErrNoRows) {
		return IndexedDocument{}, ErrDocumentNotFound
	}
	if err != nil {
		return IndexedDocument{}, fmt.Errorf("get indexed document: %w", err)
	}
	return document, nil
}

func (repository *Repository) GetByID(
	ctx context.Context,
	documentID string,
) (IndexedDocument, error) {
	document, err := scanIndexedDocument(repository.db.QueryRowContext(ctx, `
		SELECT
			id, source_root_id, relative_path, relative_path_key,
			document_type, title, task_date, modified_at, file_size,
			project_id, content_hash, content_excerpt, index_status,
			ai_access_effective, document_ai_access, policy_reasons,
			adapter_name, adapter_version, last_error_code,
			last_seen_scan_id, created_at, updated_at
		FROM indexed_documents
		WHERE id = ?
	`, documentID))
	if errors.Is(err, sql.ErrNoRows) {
		return IndexedDocument{}, ErrDocumentNotFound
	}
	if err != nil {
		return IndexedDocument{}, fmt.Errorf("get indexed document by id: %w", err)
	}
	return document, nil
}

func (repository *Repository) ListBySource(
	ctx context.Context,
	sourceRootID string,
) ([]IndexedDocument, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT
			id, source_root_id, relative_path, relative_path_key,
			document_type, title, task_date, modified_at, file_size,
			project_id, content_hash, content_excerpt, index_status,
			ai_access_effective, document_ai_access, policy_reasons,
			adapter_name, adapter_version, last_error_code,
			last_seen_scan_id, created_at, updated_at
		FROM indexed_documents
		WHERE source_root_id = ?
		ORDER BY relative_path_key
	`, sourceRootID)
	if err != nil {
		return nil, fmt.Errorf("list indexed documents: %w", err)
	}
	defer rows.Close()

	documents := make([]IndexedDocument, 0)
	for rows.Next() {
		document, scanErr := scanIndexedDocument(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan indexed document: %w", scanErr)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed documents: %w", err)
	}
	return documents, nil
}

func (repository *Repository) Save(
	ctx context.Context,
	document IndexedDocument,
) error {
	reasons, err := json.Marshal(document.PolicyReasons)
	if err != nil {
		return fmt.Errorf("encode policy reasons: %w", err)
	}

	_, err = repository.db.ExecContext(ctx, `
		INSERT INTO indexed_documents (
			id, source_root_id, relative_path, relative_path_key,
			document_type, title, task_date, modified_at, file_size,
			project_id, content_hash, content_excerpt, index_status,
			ai_access_effective, document_ai_access, policy_reasons,
			adapter_name, adapter_version, last_error_code,
			last_seen_scan_id, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		ON CONFLICT(source_root_id, relative_path_key) DO UPDATE SET
			relative_path = excluded.relative_path,
			document_type = excluded.document_type,
			title = excluded.title,
			task_date = excluded.task_date,
			modified_at = excluded.modified_at,
			file_size = excluded.file_size,
			project_id = excluded.project_id,
			content_hash = excluded.content_hash,
			content_excerpt = excluded.content_excerpt,
			index_status = excluded.index_status,
			ai_access_effective = excluded.ai_access_effective,
			document_ai_access = excluded.document_ai_access,
			policy_reasons = excluded.policy_reasons,
			adapter_name = excluded.adapter_name,
			adapter_version = excluded.adapter_version,
			last_error_code = excluded.last_error_code,
			last_seen_scan_id = excluded.last_seen_scan_id,
			updated_at = excluded.updated_at
	`,
		document.ID,
		document.SourceRootID,
		document.RelativePath,
		document.relativePathKey,
		document.DocumentType,
		document.Title,
		nullableString(document.TaskDate),
		document.ModifiedAt,
		document.FileSize,
		document.ProjectID,
		nullableString(document.ContentHash),
		nullableString(document.ContentExcerpt),
		document.IndexStatus,
		document.AIAccessEffective,
		document.documentAIAccess,
		string(reasons),
		document.adapterName,
		document.adapterVersion,
		nullableString(document.LastErrorCode),
		document.lastSeenScanID,
		document.createdAt,
		document.updatedAt,
	)
	if err != nil {
		return fmt.Errorf("save indexed document: %w", err)
	}
	return nil
}

func (repository *Repository) Touch(
	ctx context.Context,
	document IndexedDocument,
	scanID string,
	now string,
) error {
	reasons, err := json.Marshal(document.PolicyReasons)
	if err != nil {
		return fmt.Errorf("encode policy reasons: %w", err)
	}
	clearExcerpt := document.AIAccessEffective != policy.AIAccessContent

	result, err := repository.db.ExecContext(ctx, `
		UPDATE indexed_documents
		SET
			relative_path = ?,
			modified_at = ?,
			file_size = ?,
			index_status = 'ready',
			ai_access_effective = ?,
			policy_reasons = ?,
			content_excerpt = CASE WHEN ? THEN NULL ELSE content_excerpt END,
			last_error_code = NULL,
			last_seen_scan_id = ?,
			updated_at = ?
		WHERE id = ?
	`,
		document.RelativePath,
		document.ModifiedAt,
		document.FileSize,
		document.AIAccessEffective,
		string(reasons),
		clearExcerpt,
		scanID,
		now,
		document.ID,
	)
	if err != nil {
		return fmt.Errorf("touch indexed document: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read touched document count: %w", err)
	}
	if affected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

func (repository *Repository) MarkMissingDeleted(
	ctx context.Context,
	sourceRootID string,
	scanID string,
	now string,
) (int, error) {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE indexed_documents
		SET
			index_status = 'deleted',
			content_excerpt = NULL,
			last_error_code = NULL,
			updated_at = ?
		WHERE
			source_root_id = ?
			AND last_seen_scan_id <> ?
			AND index_status <> 'deleted'
	`, now, sourceRootID, scanID)
	if err != nil {
		return 0, fmt.Errorf("mark missing documents deleted: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted document count: %w", err)
	}
	return int(affected), nil
}

func (repository *Repository) MarkSourceStale(
	ctx context.Context,
	sourceRootID string,
	reason string,
	clearExcerpt bool,
	now string,
) (int, error) {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE indexed_documents
		SET
			index_status = 'stale',
			content_excerpt = CASE WHEN ? THEN NULL ELSE content_excerpt END,
			last_error_code = ?,
			updated_at = ?
		WHERE source_root_id = ? AND index_status <> 'deleted'
	`, clearExcerpt, nullableString(reason), now, sourceRootID)
	if err != nil {
		return 0, fmt.Errorf("mark source documents stale: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read stale document count: %w", err)
	}
	return int(affected), nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanIndexedDocument(scanner rowScanner) (IndexedDocument, error) {
	var document IndexedDocument
	var taskDate sql.NullString
	var projectID sql.NullString
	var contentHash sql.NullString
	var contentExcerpt sql.NullString
	var documentAIAccess sql.NullString
	var reasons string
	var lastErrorCode sql.NullString

	err := scanner.Scan(
		&document.ID,
		&document.SourceRootID,
		&document.RelativePath,
		&document.relativePathKey,
		&document.DocumentType,
		&document.Title,
		&taskDate,
		&document.ModifiedAt,
		&document.FileSize,
		&projectID,
		&contentHash,
		&contentExcerpt,
		&document.IndexStatus,
		&document.AIAccessEffective,
		&documentAIAccess,
		&reasons,
		&document.adapterName,
		&document.adapterVersion,
		&lastErrorCode,
		&document.lastSeenScanID,
		&document.createdAt,
		&document.updatedAt,
	)
	if err != nil {
		return IndexedDocument{}, err
	}

	document.TaskDate = taskDate.String
	document.ContentHash = contentHash.String
	document.ContentExcerpt = contentExcerpt.String
	document.LastErrorCode = lastErrorCode.String
	if projectID.Valid {
		document.ProjectID = &projectID.String
	}
	if documentAIAccess.Valid {
		value := policy.AIAccess(documentAIAccess.String)
		document.documentAIAccess = &value
	}
	if err := json.Unmarshal([]byte(reasons), &document.PolicyReasons); err != nil {
		return IndexedDocument{}, fmt.Errorf("decode policy reasons: %w", err)
	}
	if document.PolicyReasons == nil {
		document.PolicyReasons = []policy.ReasonCode{}
	}
	return document, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
