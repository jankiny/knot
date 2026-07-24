// Package annualsummary generates a policy-checked personal annual summary
// outline from an already confirmed ContextManifest and records a local audit.
package annualsummary

import (
	"context"
	"errors"

	"knot-backend/policy"
)

const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"

	ErrorCodeGatewayFailure     = "ai_gateway_failed"
	ErrorCodeInvalidModelOutput = "invalid_model_output"
)

var (
	ErrRunNotFound     = errors.New("AI run not found")
	ErrManifestInvalid = errors.New("context manifest is invalid")
)

type AIConfig struct {
	ConfigID string `json:"config_id"`
	APIURL   string `json:"api_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
}

type GenerateRequest struct {
	EvidenceIDs []string `json:"evidence_ids"`
	ConfirmSend bool     `json:"confirm_send"`
	AI          AIConfig `json:"ai"`
}

type ModelEvidence struct {
	ID                string          `json:"evidence_id"`
	SourceType        string          `json:"source_type"`
	Title             string          `json:"title"`
	Date              string          `json:"date"`
	Project           string          `json:"project"`
	Excerpt           string          `json:"excerpt"`
	AIAccessEffective policy.AIAccess `json:"ai_access_effective"`
}

type ModelInput struct {
	TaskType    string          `json:"task_type"`
	Query       string          `json:"query"`
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	Evidence    []ModelEvidence `json:"evidence"`
}

type Completion struct {
	Content      string
	InputTokens  *int
	OutputTokens *int
}

type Gateway interface {
	Complete(context.Context, AIConfig, ModelInput) (Completion, error)
}

type Section struct {
	Heading     string   `json:"heading"`
	WordCount   int      `json:"word_count"`
	Outline     []string `json:"outline"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Achievement struct {
	Statement   string   `json:"statement"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Result struct {
	Title              string        `json:"title"`
	TargetWordCount    int           `json:"target_word_count"`
	Sections           []Section     `json:"sections"`
	VerifiedResults    []Achievement `json:"verified_results"`
	MissingInformation []string      `json:"missing_information"`
	Markdown           string        `json:"markdown"`
}

type GenerateResponse struct {
	RunID         string `json:"run_id"`
	ManifestID    string `json:"manifest_id"`
	EvidenceCount int    `json:"evidence_count"`
	InputTokens   *int   `json:"input_tokens"`
	OutputTokens  *int   `json:"output_tokens"`
	Result        Result `json:"result"`
}

type Run struct {
	ID                    string  `json:"id"`
	ContextManifestID     string  `json:"context_manifest_id"`
	ModelConfigID         string  `json:"model_config_id"`
	Model                 string  `json:"model"`
	TaskType              string  `json:"task_type"`
	Status                string  `json:"status"`
	SelectedEvidenceCount int     `json:"selected_evidence_count"`
	InputTokens           *int    `json:"input_tokens"`
	OutputTokens          *int    `json:"output_tokens"`
	Result                *Result `json:"result,omitempty"`
	ErrorCode             string  `json:"error_code,omitempty"`
	ErrorMessage          string  `json:"error_message,omitempty"`
	CreatedAt             string  `json:"created_at"`
	CompletedAt           string  `json:"completed_at,omitempty"`
}

type RunSource struct {
	Position          int             `json:"position"`
	EvidenceID        string          `json:"evidence_id"`
	DocumentID        string          `json:"document_id"`
	SourceRootID      string          `json:"source_root_id"`
	SourceType        string          `json:"source_type"`
	Title             string          `json:"title"`
	Date              string          `json:"date"`
	Project           string          `json:"project"`
	AIAccessEffective policy.AIAccess `json:"ai_access_effective"`
}

type ValidationError struct {
	Field   string
	Message string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Message
}

type GenerationError struct {
	RunID string
	Code  string
	Err   error
}

func (err *GenerationError) Error() string {
	if err == nil || err.Err == nil {
		return "annual summary generation failed"
	}
	return err.Err.Error()
}

func (err *GenerationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}
