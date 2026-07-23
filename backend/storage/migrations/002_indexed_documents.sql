CREATE TABLE indexed_documents (
    id TEXT PRIMARY KEY,
    source_root_id TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    relative_path_key TEXT NOT NULL,
    document_type TEXT NOT NULL,
    title TEXT NOT NULL,
    task_date TEXT,
    modified_at TEXT NOT NULL,
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    project_id TEXT,
    content_hash TEXT,
    content_excerpt TEXT,
    index_status TEXT NOT NULL CHECK (
        index_status IN ('ready', 'stale', 'deleted', 'error')
    ),
    ai_access_effective TEXT NOT NULL CHECK (
        ai_access_effective IN ('none', 'metadata', 'content')
    ),
    document_ai_access TEXT CHECK (
        document_ai_access IS NULL
        OR document_ai_access IN ('none', 'metadata', 'content')
    ),
    policy_reasons TEXT NOT NULL DEFAULT '[]',
    adapter_name TEXT NOT NULL,
    adapter_version INTEGER NOT NULL CHECK (adapter_version > 0),
    last_error_code TEXT,
    last_seen_scan_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(source_root_id, relative_path_key),
    FOREIGN KEY(source_root_id) REFERENCES source_roots(id) ON DELETE CASCADE
);

CREATE INDEX idx_indexed_documents_source_status
    ON indexed_documents(source_root_id, index_status);

CREATE INDEX idx_indexed_documents_type_date
    ON indexed_documents(document_type, task_date);

CREATE INDEX idx_indexed_documents_project
    ON indexed_documents(project_id, task_date);
