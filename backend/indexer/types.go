// Package indexer maintains the bounded, rebuildable local document index.
package indexer

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"knot-backend/policy"
	"knot-backend/sources"
)

const (
	DocumentTypeWorkRecord = "work_record"
	DocumentTypeText       = "text"
	DocumentTypeFile       = "file"

	IndexStatusReady   = "ready"
	IndexStatusStale   = "stale"
	IndexStatusDeleted = "deleted"
	IndexStatusError   = "error"

	ScanStatusCompleted   = "completed"
	ScanStatusPartial     = "partial"
	ScanStatusUnavailable = "unavailable"

	ReasonScanLimit  = "scan_limit"
	ReasonCancelled  = "cancelled"
	ReasonReadError  = "read_error"
	ReasonParseError = "parse_error"
)

var ErrDocumentNotFound = errors.New("indexed document not found")

type IndexedDocument struct {
	ID                string              `json:"id"`
	SourceRootID      string              `json:"source_root_id"`
	RelativePath      string              `json:"relative_path"`
	DocumentType      string              `json:"document_type"`
	Title             string              `json:"title"`
	TaskDate          string              `json:"task_date,omitempty"`
	ModifiedAt        string              `json:"modified_at"`
	FileSize          int64               `json:"file_size"`
	ProjectID         *string             `json:"project_id"`
	ContentHash       string              `json:"content_hash,omitempty"`
	ContentExcerpt    string              `json:"content_excerpt,omitempty"`
	IndexStatus       string              `json:"index_status"`
	AIAccessEffective policy.AIAccess     `json:"ai_access_effective"`
	PolicyReasons     []policy.ReasonCode `json:"policy_reasons"`
	LastErrorCode     string              `json:"last_error_code,omitempty"`

	relativePathKey  string
	documentAIAccess *policy.AIAccess
	adapterName      string
	adapterVersion   int
	lastSeenScanID   string
	createdAt        string
	updatedAt        string
}

type Limits struct {
	MaxFiles          int
	MaxTotalBytes     int64
	MaxFileBytes      int64
	MaxDuration       time.Duration
	MaxExcerptRunes   int
	MaxReportedIssues int
}

func DefaultLimits() Limits {
	return Limits{
		MaxFiles:          10_000,
		MaxTotalBytes:     512 << 20,
		MaxFileBytes:      10 << 20,
		MaxDuration:       30 * time.Second,
		MaxExcerptRunes:   1_200,
		MaxReportedIssues: 100,
	}
}

type ScanOptions struct {
	Force bool `json:"force"`
}

type ScanIssue struct {
	RelativePath string `json:"relative_path"`
	Reason       string `json:"reason"`
}

type ScanResult struct {
	SourceRootID       string      `json:"source_root_id"`
	Status             string      `json:"status"`
	StartedAt          string      `json:"started_at"`
	FinishedAt         string      `json:"finished_at"`
	FilesSeen          int         `json:"files_seen"`
	BytesRead          int64       `json:"bytes_read"`
	Indexed            int         `json:"indexed"`
	Updated            int         `json:"updated"`
	Unchanged          int         `json:"unchanged"`
	Deleted            int         `json:"deleted"`
	MetadataOnly       int         `json:"metadata_only"`
	IgnoredDirectories int         `json:"ignored_directories"`
	Issues             []ScanIssue `json:"issues"`
	OmittedIssues      int         `json:"omitted_issues"`
}

type ScanAllResult struct {
	Status      string       `json:"status"`
	SourceRoots int          `json:"source_roots"`
	Results     []ScanResult `json:"results"`
}

type AdapterInput struct {
	SourceRoot      sources.SourceRoot
	RelativePath    string
	AbsolutePath    string
	FileInfo        fs.FileInfo
	MaxExcerptRunes int
}

type Extraction struct {
	DocumentType     string
	Title            string
	TaskDate         string
	ContentHash      string
	ContentExcerpt   string
	DeclaredAIAccess string
}

type Adapter interface {
	Name() string
	Version() int
	Matches(relativePath string) bool
	ReadsContentForMetadata() bool
	Extract(context.Context, AdapterInput) (Extraction, error)
}

type WorkRecordData struct {
	RawContent []byte
	Title      string
	TaskDate   string
	Content    string
	AIAccess   string
}

type WorkRecordReader func(string) (WorkRecordData, error)
