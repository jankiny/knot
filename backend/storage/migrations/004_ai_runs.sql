CREATE TABLE ai_runs (
    id TEXT PRIMARY KEY,
    context_manifest_id TEXT NOT NULL,
    model_config_id TEXT NOT NULL,
    model TEXT NOT NULL,
    task_type TEXT NOT NULL CHECK (
        task_type IN ('personal_annual_summary')
    ),
    status TEXT NOT NULL CHECK (
        status IN ('running', 'success', 'failed')
    ),
    selected_evidence_count INTEGER NOT NULL CHECK (
        selected_evidence_count > 0
    ),
    input_tokens INTEGER CHECK (
        input_tokens IS NULL OR input_tokens >= 0
    ),
    output_tokens INTEGER CHECK (
        output_tokens IS NULL OR output_tokens >= 0
    ),
    result_json TEXT,
    error_code TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE ai_run_evidence (
    run_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    evidence_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    source_root_id TEXT NOT NULL,
    source_type TEXT NOT NULL,
    title TEXT NOT NULL,
    evidence_date TEXT NOT NULL,
    project TEXT NOT NULL,
    ai_access_effective TEXT NOT NULL CHECK (
        ai_access_effective IN ('metadata', 'content')
    ),
    PRIMARY KEY(run_id, evidence_id),
    UNIQUE(run_id, position),
    FOREIGN KEY(run_id)
        REFERENCES ai_runs(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_ai_runs_context_created
    ON ai_runs(context_manifest_id, created_at);

CREATE INDEX idx_ai_runs_status_created
    ON ai_runs(status, created_at);

CREATE INDEX idx_ai_run_evidence_evidence
    ON ai_run_evidence(evidence_id);
