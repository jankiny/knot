CREATE TABLE source_roots (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'current_work',
            'active_work',
            'work_archive',
            'journal',
            'reference',
            'private',
            'external_offline'
        )
    ),
    path TEXT NOT NULL,
    path_key TEXT NOT NULL UNIQUE,
    scope_type TEXT NOT NULL CHECK (
        scope_type IN ('global', 'department', 'project')
    ),
    scope_id TEXT,
    scope_name TEXT,
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    recursive INTEGER NOT NULL CHECK (recursive IN (0, 1)),
    local_access TEXT NOT NULL CHECK (
        local_access IN ('none', 'read', 'read_write')
    ),
    ai_access TEXT NOT NULL CHECK (
        ai_access IN ('none', 'metadata', 'content')
    ),
    availability TEXT NOT NULL CHECK (
        availability IN (
            'online',
            'offline',
            'missing',
            'permission_denied',
            'unknown'
        )
    ),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (scope_type = 'global' AND scope_id IS NULL AND scope_name IS NULL)
        OR scope_type IN ('department', 'project')
    )
);

CREATE INDEX idx_source_roots_enabled_kind
    ON source_roots(enabled, kind);

CREATE INDEX idx_source_roots_scope
    ON source_roots(scope_type, scope_id);
