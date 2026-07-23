// Package sources implements the persistent SourceRoot registry.
package sources

import (
	"errors"

	"knot-backend/policy"
)

type Kind string

const (
	KindCurrentWork     Kind = "current_work"
	KindActiveWork      Kind = "active_work"
	KindWorkArchive     Kind = "work_archive"
	KindJournal         Kind = "journal"
	KindReference       Kind = "reference"
	KindPrivate         Kind = "private"
	KindExternalOffline Kind = "external_offline"
)

type ScopeType string

const (
	ScopeGlobal     ScopeType = "global"
	ScopeDepartment ScopeType = "department"
	ScopeProject    ScopeType = "project"
)

type LocalAccess = policy.LocalAccess

const (
	LocalAccessNone      = policy.LocalAccessNone
	LocalAccessRead      = policy.LocalAccessRead
	LocalAccessReadWrite = policy.LocalAccessReadWrite
)

type AIAccess = policy.AIAccess

const (
	AIAccessNone     = policy.AIAccessNone
	AIAccessMetadata = policy.AIAccessMetadata
	AIAccessContent  = policy.AIAccessContent
)

type Availability string

const (
	AvailabilityOnline           Availability = "online"
	AvailabilityOffline          Availability = "offline"
	AvailabilityMissing          Availability = "missing"
	AvailabilityPermissionDenied Availability = "permission_denied"
	AvailabilityUnknown          Availability = "unknown"
)

// SourceRoot is the stable public resource contract. Database-only fields are
// intentionally hidden from JSON.
type SourceRoot struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Kind         Kind         `json:"kind"`
	Path         string       `json:"path"`
	ScopeType    ScopeType    `json:"scope_type"`
	ScopeID      *string      `json:"scope_id"`
	ScopeName    *string      `json:"scope_name"`
	Enabled      bool         `json:"enabled"`
	Recursive    bool         `json:"recursive"`
	LocalAccess  LocalAccess  `json:"local_access"`
	AIAccess     AIAccess     `json:"ai_access"`
	Availability Availability `json:"availability"`

	pathKey   string
	createdAt string
	updatedAt string
}

type Input struct {
	Name        string      `json:"name"`
	Kind        Kind        `json:"kind"`
	Path        string      `json:"path"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     *string     `json:"scope_id"`
	ScopeName   *string     `json:"scope_name"`
	Enabled     *bool       `json:"enabled"`
	Recursive   *bool       `json:"recursive"`
	LocalAccess LocalAccess `json:"local_access"`
	AIAccess    AIAccess    `json:"ai_access"`
}

type LegacyOwner struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ArchivePath string `json:"archive_path"`
}

type LegacyImportInput struct {
	FolderPath  string        `json:"folder_path"`
	ScanPath    string        `json:"scan_path"`
	Departments []LegacyOwner `json:"departments"`
	Projects    []LegacyOwner `json:"projects"`
}

type ImportSkip struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type LegacyImportResult struct {
	Imported []SourceRoot `json:"imported"`
	Existing []SourceRoot `json:"existing"`
	Skipped  []ImportSkip `json:"skipped"`
}

var (
	ErrNotFound     = errors.New("source root not found")
	ErrPathConflict = errors.New("source root path already exists")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
