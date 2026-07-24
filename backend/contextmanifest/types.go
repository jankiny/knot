// Package contextmanifest discovers and persists bounded, policy-checked
// evidence manifests without calling an AI model.
package contextmanifest

import (
	"errors"
	"time"

	"knot-backend/policy"
	"knot-backend/sources"
)

const (
	TaskTypePersonalAnnualSummary = "personal_annual_summary"

	StatusReady   = "ready"
	StatusInvalid = "invalid"

	ReasonOutsidePeriod       = "outside_period"
	ReasonIndexStale          = "index_stale"
	ReasonIndexDeleted        = "index_deleted"
	ReasonIndexError          = "index_error"
	ReasonIndexOutdated       = "index_outdated"
	ReasonReadError           = "read_error"
	ReasonTokenBudget         = "token_budget"
	ReasonCandidateLimit      = "candidate_limit"
	ReasonManifestExpired     = "manifest_expired"
	ReasonDocumentMissing     = "document_missing"
	ReasonDocumentHashChanged = "document_hash_changed"
	ReasonDocumentChanged     = "document_changed"
	ReasonPolicyChanged       = "policy_changed"
)

var ErrManifestNotFound = errors.New("context manifest not found")

type DiscoverRequest struct {
	TaskType    string `json:"task_type"`
	Query       string `json:"query"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type SourceSummary struct {
	SourceRootID       string            `json:"source_root_id"`
	Name               string            `json:"name"`
	Kind               sources.Kind      `json:"kind"`
	ScopeType          sources.ScopeType `json:"scope_type"`
	ScopeID            *string           `json:"scope_id"`
	ScopeName          *string           `json:"scope_name"`
	CandidateDocuments int               `json:"candidate_documents"`
	EvidenceItems      int               `json:"evidence_items"`
}

type EvidenceItem struct {
	ID                string          `json:"id"`
	DocumentID        string          `json:"document_id"`
	SourceRootID      string          `json:"source_root_id"`
	SourceType        string          `json:"source_type"`
	Title             string          `json:"title"`
	Date              string          `json:"date"`
	Project           string          `json:"project"`
	Excerpt           string          `json:"excerpt"`
	Reason            string          `json:"reason"`
	AIAccessEffective policy.AIAccess `json:"ai_access_effective"`
}

type ExcludedItem struct {
	SourceRootID string `json:"source_root_id"`
	DocumentID   string `json:"document_id,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Title        string `json:"title,omitempty"`
	ReasonCode   string `json:"reason_code"`
	Reason       string `json:"reason"`
}

type UnavailableSource struct {
	SourceRootID string               `json:"source_root_id"`
	Name         string               `json:"name"`
	Kind         sources.Kind         `json:"kind"`
	Availability sources.Availability `json:"availability"`
	ReasonCode   string               `json:"reason_code"`
	Reason       string               `json:"reason"`
}

type ContextManifest struct {
	ID                   string              `json:"id"`
	TaskType             string              `json:"task_type"`
	Query                string              `json:"query"`
	PeriodStart          string              `json:"period_start"`
	PeriodEnd            string              `json:"period_end"`
	Sources              []SourceSummary     `json:"sources"`
	Evidence             []EvidenceItem      `json:"evidence"`
	Excluded             []ExcludedItem      `json:"excluded"`
	ExcludedCount        int                 `json:"excluded_count"`
	OmittedExcludedCount int                 `json:"omitted_excluded_count"`
	UnavailableSources   []UnavailableSource `json:"unavailable_sources"`
	EstimatedInputTokens int                 `json:"estimated_input_tokens"`
	TokenBudget          int                 `json:"token_budget"`
	RequiresConfirmation bool                `json:"requires_confirmation"`
	Status               string              `json:"status"`
	Valid                bool                `json:"valid"`
	RequiresRediscovery  bool                `json:"requires_rediscovery"`
	InvalidatedReasons   []string            `json:"invalidated_reasons"`
	CreatedAt            string              `json:"created_at"`
	ExpiresAt            string              `json:"expires_at"`
}

type Limits struct {
	MaxDocuments     int
	MaxEvidence      int
	MaxExcluded      int
	MaxEvidenceRunes int
	MaxInputTokens   int
	MaxQueryRunes    int
	MaxDuration      time.Duration
	ManifestTTL      time.Duration
}

func DefaultLimits() Limits {
	return Limits{
		MaxDocuments:     20_000,
		MaxEvidence:      100,
		MaxExcluded:      200,
		MaxEvidenceRunes: 800,
		MaxInputTokens:   12_000,
		MaxQueryRunes:    2_000,
		MaxDuration:      30 * time.Second,
		ManifestTTL:      30 * time.Minute,
	}
}

type ValidationError struct {
	Field   string
	Message string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Message
}

type documentSnapshot struct {
	DocumentID        string
	SourceRootID      string
	RelativePath      string
	ContentHash       string
	ModifiedAt        string
	FileSize          int64
	AIAccessEffective policy.AIAccess
}
