package annualsummary

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

func (repository *Repository) Start(
	ctx context.Context,
	run Run,
	sources []RunSource,
) error {
	if repository == nil || repository.db == nil {
		return errors.New("AI run repository is unavailable")
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin AI run audit: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ai_runs (
			id, context_manifest_id, model_config_id, model, task_type,
			status, selected_evidence_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		run.ID,
		run.ContextManifestID,
		run.ModelConfigID,
		run.Model,
		run.TaskType,
		StatusRunning,
		run.SelectedEvidenceCount,
		run.CreatedAt,
	); err != nil {
		return fmt.Errorf("start AI run audit: %w", err)
	}

	for _, source := range sources {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_run_evidence (
				run_id, position, evidence_id, document_id, source_root_id,
				source_type, title, evidence_date, project, ai_access_effective
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			run.ID,
			source.Position,
			source.EvidenceID,
			source.DocumentID,
			source.SourceRootID,
			source.SourceType,
			source.Title,
			source.Date,
			source.Project,
			source.AIAccessEffective,
		); err != nil {
			return fmt.Errorf("save AI run evidence audit: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit AI run audit: %w", err)
	}
	return nil
}

func (repository *Repository) Complete(
	ctx context.Context,
	runID string,
	result Result,
	inputTokens *int,
	outputTokens *int,
	completedAt string,
) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode AI run result: %w", err)
	}
	return repository.finish(
		ctx,
		runID,
		StatusSuccess,
		inputTokens,
		outputTokens,
		string(payload),
		"",
		"",
		completedAt,
	)
}

func (repository *Repository) Fail(
	ctx context.Context,
	runID string,
	code string,
	message string,
	inputTokens *int,
	outputTokens *int,
	completedAt string,
) error {
	return repository.finish(
		ctx,
		runID,
		StatusFailed,
		inputTokens,
		outputTokens,
		"",
		code,
		message,
		completedAt,
	)
}

func (repository *Repository) finish(
	ctx context.Context,
	runID string,
	status string,
	inputTokens *int,
	outputTokens *int,
	resultJSON string,
	errorCode string,
	errorMessage string,
	completedAt string,
) error {
	if repository == nil || repository.db == nil {
		return errors.New("AI run repository is unavailable")
	}
	var resultValue any
	if resultJSON != "" {
		resultValue = resultJSON
	}
	var errorCodeValue any
	if errorCode != "" {
		errorCodeValue = errorCode
	}
	var errorMessageValue any
	if errorMessage != "" {
		errorMessageValue = errorMessage
	}
	result, err := repository.db.ExecContext(ctx, `
		UPDATE ai_runs
		SET
			status = ?,
			input_tokens = ?,
			output_tokens = ?,
			result_json = ?,
			error_code = ?,
			error_message = ?,
			completed_at = ?
		WHERE id = ? AND status = ?
	`,
		status,
		nullableInt(inputTokens),
		nullableInt(outputTokens),
		resultValue,
		errorCodeValue,
		errorMessageValue,
		completedAt,
		runID,
		StatusRunning,
	)
	if err != nil {
		return fmt.Errorf("finish AI run audit: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read AI run audit update count: %w", err)
	}
	if updated != 1 {
		return ErrRunNotFound
	}
	return nil
}

func (repository *Repository) Get(
	ctx context.Context,
	runID string,
) (Run, error) {
	if repository == nil || repository.db == nil {
		return Run{}, errors.New("AI run repository is unavailable")
	}
	var run Run
	var inputTokens sql.NullInt64
	var outputTokens sql.NullInt64
	var resultJSON sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var completedAt sql.NullString
	err := repository.db.QueryRowContext(ctx, `
		SELECT
			id, context_manifest_id, model_config_id, model, task_type,
			status, selected_evidence_count, input_tokens, output_tokens,
			result_json, error_code, error_message, created_at, completed_at
		FROM ai_runs
		WHERE id = ?
	`, runID).Scan(
		&run.ID,
		&run.ContextManifestID,
		&run.ModelConfigID,
		&run.Model,
		&run.TaskType,
		&run.Status,
		&run.SelectedEvidenceCount,
		&inputTokens,
		&outputTokens,
		&resultJSON,
		&errorCode,
		&errorMessage,
		&run.CreatedAt,
		&completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("get AI run audit: %w", err)
	}
	run.InputTokens = intPointer(inputTokens)
	run.OutputTokens = intPointer(outputTokens)
	run.ErrorCode = errorCode.String
	run.ErrorMessage = errorMessage.String
	run.CompletedAt = completedAt.String
	if resultJSON.Valid {
		var result Result
		if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
			return Run{}, fmt.Errorf("decode AI run result: %w", err)
		}
		run.Result = &result
	}
	return run, nil
}

func (repository *Repository) ListSources(
	ctx context.Context,
	runID string,
) ([]RunSource, error) {
	if repository == nil || repository.db == nil {
		return nil, errors.New("AI run repository is unavailable")
	}
	var exists int
	if err := repository.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM ai_runs WHERE id = ?",
		runID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check AI run audit: %w", err)
	}
	if exists == 0 {
		return nil, ErrRunNotFound
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT
			position, evidence_id, document_id, source_root_id, source_type,
			title, evidence_date, project, ai_access_effective
		FROM ai_run_evidence
		WHERE run_id = ?
		ORDER BY position
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list AI run evidence audit: %w", err)
	}
	defer rows.Close()

	sources := make([]RunSource, 0)
	for rows.Next() {
		var source RunSource
		if err := rows.Scan(
			&source.Position,
			&source.EvidenceID,
			&source.DocumentID,
			&source.SourceRootID,
			&source.SourceType,
			&source.Title,
			&source.Date,
			&source.Project,
			&source.AIAccessEffective,
		); err != nil {
			return nil, fmt.Errorf("scan AI run evidence audit: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI run evidence audit: %w", err)
	}
	return sources, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}
