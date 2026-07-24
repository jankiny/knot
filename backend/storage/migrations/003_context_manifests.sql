CREATE TABLE context_manifests (
    id TEXT PRIMARY KEY,
    task_type TEXT NOT NULL CHECK (
        task_type IN ('personal_annual_summary')
    ),
    query TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    manifest_json TEXT NOT NULL,
    snapshot_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE context_manifest_documents (
    manifest_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    source_root_id TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    modified_at TEXT NOT NULL,
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    ai_access_effective TEXT NOT NULL CHECK (
        ai_access_effective IN ('metadata', 'content')
    ),
    PRIMARY KEY(manifest_id, document_id),
    FOREIGN KEY(manifest_id)
        REFERENCES context_manifests(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_context_manifests_expires_at
    ON context_manifests(expires_at);

CREATE INDEX idx_context_manifest_documents_document
    ON context_manifest_documents(document_id);
