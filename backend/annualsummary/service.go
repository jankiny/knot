package annualsummary

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"knot-backend/contextmanifest"
	"knot-backend/policy"
)

type Service struct {
	resolver   *contextmanifest.Resolver
	repository *Repository
	gateway    Gateway
	now        func() time.Time
	newRunID   func() (string, error)
}

func NewService(
	resolver *contextmanifest.Resolver,
	repository *Repository,
	gateway Gateway,
) *Service {
	return &Service{
		resolver:   resolver,
		repository: repository,
		gateway:    gateway,
		now:        time.Now,
		newRunID: func() (string, error) {
			value := make([]byte, 12)
			if _, err := rand.Read(value); err != nil {
				return "", err
			}
			return "airun_" + hex.EncodeToString(value), nil
		},
	}
}

func (service *Service) Generate(
	ctx context.Context,
	manifestID string,
	request GenerateRequest,
) (GenerateResponse, error) {
	if err := service.validate(); err != nil {
		return GenerateResponse{}, err
	}
	manifestID = strings.TrimSpace(manifestID)
	if manifestID == "" {
		return GenerateResponse{}, &ValidationError{
			Field:   "manifest_id",
			Message: "is required",
		}
	}
	if !request.ConfirmSend {
		return GenerateResponse{}, &ValidationError{
			Field:   "confirm_send",
			Message: "must be true before evidence can be sent to the configured model",
		}
	}
	if err := validateAIConfig(request.AI); err != nil {
		return GenerateResponse{}, err
	}
	selectedIDs, err := validateEvidenceIDs(request.EvidenceIDs)
	if err != nil {
		return GenerateResponse{}, err
	}

	manifest, err := service.resolver.Get(ctx, manifestID)
	if err != nil {
		return GenerateResponse{}, err
	}
	if !manifest.Valid || manifest.RequiresRediscovery ||
		manifest.Status != contextmanifest.StatusReady {
		return GenerateResponse{}, fmt.Errorf(
			"%w: %s",
			ErrManifestInvalid,
			strings.Join(manifest.InvalidatedReasons, ","),
		)
	}

	selectedEvidence, err := selectEvidence(manifest.Evidence, selectedIDs)
	if err != nil {
		return GenerateResponse{}, err
	}
	runID, err := service.newRunID()
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("generate AI run id: %w", err)
	}
	createdAt := service.now().UTC().Format(time.RFC3339Nano)
	run := Run{
		ID:                    runID,
		ContextManifestID:     manifest.ID,
		ModelConfigID:         strings.TrimSpace(request.AI.ConfigID),
		Model:                 strings.TrimSpace(request.AI.Model),
		TaskType:              manifest.TaskType,
		Status:                StatusRunning,
		SelectedEvidenceCount: len(selectedEvidence),
		CreatedAt:             createdAt,
	}
	runSources := makeRunSources(selectedEvidence)
	if err := service.repository.Start(ctx, run, runSources); err != nil {
		return GenerateResponse{}, err
	}

	modelInput := makeModelInput(manifest, selectedEvidence)
	completion, err := service.gateway.Complete(ctx, request.AI, modelInput)
	if err != nil {
		return GenerateResponse{}, service.failRun(
			ctx,
			runID,
			ErrorCodeGatewayFailure,
			"AI gateway request failed.",
			nil,
			nil,
			err,
		)
	}

	result, err := parseModelResult(completion.Content, selectedIDs)
	if err != nil {
		return GenerateResponse{}, service.failRun(
			ctx,
			runID,
			ErrorCodeInvalidModelOutput,
			"Model output failed local JSON or evidence validation.",
			completion.InputTokens,
			completion.OutputTokens,
			err,
		)
	}
	appendUnavailableGaps(&result, manifest.UnavailableSources)
	result.Markdown = renderMarkdown(
		result,
		manifest.PeriodStart,
		manifest.PeriodEnd,
		len(selectedEvidence),
	)
	completedAt := service.now().UTC().Format(time.RFC3339Nano)
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := service.repository.Complete(
		finalizeCtx,
		runID,
		result,
		completion.InputTokens,
		completion.OutputTokens,
		completedAt,
	); err != nil {
		return GenerateResponse{}, fmt.Errorf("record successful AI run: %w", err)
	}
	return GenerateResponse{
		RunID:         runID,
		ManifestID:    manifest.ID,
		EvidenceCount: len(selectedEvidence),
		InputTokens:   completion.InputTokens,
		OutputTokens:  completion.OutputTokens,
		Result:        result,
	}, nil
}

func (service *Service) failRun(
	ctx context.Context,
	runID string,
	code string,
	message string,
	inputTokens *int,
	outputTokens *int,
	generationErr error,
) error {
	completedAt := service.now().UTC().Format(time.RFC3339Nano)
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	auditErr := service.repository.Fail(
		finalizeCtx,
		runID,
		code,
		message,
		inputTokens,
		outputTokens,
		completedAt,
	)
	if auditErr != nil {
		generationErr = fmt.Errorf(
			"%v; record failed AI run: %w",
			generationErr,
			auditErr,
		)
	}
	return &GenerationError{
		RunID: runID,
		Code:  code,
		Err:   generationErr,
	}
}

func (service *Service) validate() error {
	switch {
	case service == nil:
		return errors.New("annual summary service is unavailable")
	case service.resolver == nil:
		return errors.New("context resolver is unavailable")
	case service.repository == nil || service.repository.db == nil:
		return errors.New("AI run repository is unavailable")
	case service.gateway == nil:
		return errors.New("AI gateway is unavailable")
	default:
		return nil
	}
}

func validateAIConfig(config AIConfig) error {
	switch {
	case !config.Enabled:
		return &ValidationError{
			Field:   "ai.enabled",
			Message: "must be true",
		}
	case strings.TrimSpace(config.ConfigID) == "":
		return &ValidationError{
			Field:   "ai.config_id",
			Message: "is required",
		}
	case strings.TrimSpace(config.APIURL) == "":
		return &ValidationError{
			Field:   "ai.api_url",
			Message: "is required",
		}
	case strings.TrimSpace(config.Model) == "":
		return &ValidationError{
			Field:   "ai.model",
			Message: "is required",
		}
	case strings.TrimSpace(config.APIKey) == "":
		return &ValidationError{
			Field:   "ai.api_key",
			Message: "is required",
		}
	default:
		return validateAIDestination(config.APIURL)
	}
}

func validateEvidenceIDs(values []string) (map[string]struct{}, error) {
	if len(values) == 0 {
		return nil, &ValidationError{
			Field:   "evidence_ids",
			Message: "must contain at least one confirmed evidence ID",
		}
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, &ValidationError{
				Field:   "evidence_ids",
				Message: "must not contain empty values",
			}
		}
		if _, exists := result[value]; exists {
			return nil, &ValidationError{
				Field:   "evidence_ids",
				Message: "must not contain duplicates",
			}
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func selectEvidence(
	available []contextmanifest.EvidenceItem,
	selectedIDs map[string]struct{},
) ([]contextmanifest.EvidenceItem, error) {
	selected := make([]contextmanifest.EvidenceItem, 0, len(selectedIDs))
	found := make(map[string]struct{}, len(selectedIDs))
	for _, evidence := range available {
		if _, ok := selectedIDs[evidence.ID]; !ok {
			continue
		}
		if evidence.AIAccessEffective != policy.AIAccessMetadata &&
			evidence.AIAccessEffective != policy.AIAccessContent {
			return nil, &ValidationError{
				Field:   "evidence_ids",
				Message: "contains evidence that is no longer allowed for AI",
			}
		}
		if evidence.AIAccessEffective == policy.AIAccessMetadata &&
			evidence.Excerpt != "" {
			return nil, errors.New("metadata evidence unexpectedly contains content")
		}
		selected = append(selected, evidence)
		found[evidence.ID] = struct{}{}
	}
	if len(found) != len(selectedIDs) {
		return nil, &ValidationError{
			Field:   "evidence_ids",
			Message: "must be a subset of the current manifest evidence",
		}
	}
	return selected, nil
}

func makeModelInput(
	manifest contextmanifest.ContextManifest,
	evidence []contextmanifest.EvidenceItem,
) ModelInput {
	items := make([]ModelEvidence, 0, len(evidence))
	for _, item := range evidence {
		excerpt := item.Excerpt
		if item.AIAccessEffective != policy.AIAccessContent {
			excerpt = ""
		}
		items = append(items, ModelEvidence{
			ID:                item.ID,
			SourceType:        item.SourceType,
			Title:             item.Title,
			Date:              item.Date,
			Project:           item.Project,
			Excerpt:           excerpt,
			AIAccessEffective: item.AIAccessEffective,
		})
	}
	return ModelInput{
		TaskType:    manifest.TaskType,
		Query:       manifest.Query,
		PeriodStart: manifest.PeriodStart,
		PeriodEnd:   manifest.PeriodEnd,
		Evidence:    items,
	}
}

func makeRunSources(
	evidence []contextmanifest.EvidenceItem,
) []RunSource {
	sources := make([]RunSource, 0, len(evidence))
	for position, item := range evidence {
		sources = append(sources, RunSource{
			Position:          position,
			EvidenceID:        item.ID,
			DocumentID:        item.DocumentID,
			SourceRootID:      item.SourceRootID,
			SourceType:        item.SourceType,
			Title:             item.Title,
			Date:              item.Date,
			Project:           item.Project,
			AIAccessEffective: item.AIAccessEffective,
		})
	}
	return sources
}

type modelResult struct {
	Title              string        `json:"title"`
	TargetWordCount    int           `json:"target_word_count"`
	Sections           []Section     `json:"sections"`
	VerifiedResults    []Achievement `json:"verified_results"`
	MissingInformation []string      `json:"missing_information"`
}

func parseModelResult(
	content string,
	selectedIDs map[string]struct{},
) (Result, error) {
	content = stripJSONCodeFence(content)
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	var model modelResult
	if err := decoder.Decode(&model); err != nil {
		return Result{}, fmt.Errorf("decode annual summary JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Result{}, errors.New("annual summary JSON has trailing values")
	}
	if strings.TrimSpace(model.Title) == "" ||
		utf8.RuneCountInString(model.Title) > 200 {
		return Result{}, errors.New("annual summary title is missing or too long")
	}
	if model.TargetWordCount < 1200 || model.TargetWordCount > 1800 {
		return Result{}, errors.New(
			"annual summary target_word_count must be between 1200 and 1800",
		)
	}
	if len(model.Sections) == 0 || len(model.Sections) > 20 {
		return Result{}, errors.New("annual summary sections must contain 1 to 20 items")
	}
	if model.VerifiedResults == nil || model.MissingInformation == nil {
		return Result{}, errors.New(
			"annual summary verified_results and missing_information are required",
		)
	}

	allocatedWords := 0
	for index := range model.Sections {
		section := &model.Sections[index]
		section.Heading = strings.TrimSpace(section.Heading)
		if section.Heading == "" ||
			utf8.RuneCountInString(section.Heading) > 200 {
			return Result{}, errors.New("annual summary section heading is invalid")
		}
		if section.WordCount <= 0 || section.WordCount > 1800 {
			return Result{}, errors.New("annual summary section word_count is invalid")
		}
		allocatedWords += section.WordCount
		if len(section.Outline) == 0 || len(section.Outline) > 20 {
			return Result{}, errors.New("annual summary section outline is invalid")
		}
		for outlineIndex := range section.Outline {
			section.Outline[outlineIndex] = strings.TrimSpace(
				section.Outline[outlineIndex],
			)
			if section.Outline[outlineIndex] == "" ||
				utf8.RuneCountInString(section.Outline[outlineIndex]) > 500 {
				return Result{}, errors.New("annual summary outline item is invalid")
			}
		}
		references, err := validateReferences(section.EvidenceIDs, selectedIDs)
		if err != nil {
			return Result{}, fmt.Errorf(
				"section %q evidence_ids: %w",
				section.Heading,
				err,
			)
		}
		section.EvidenceIDs = references
	}
	if allocatedWords < model.TargetWordCount-200 ||
		allocatedWords > model.TargetWordCount+200 {
		return Result{}, errors.New(
			"annual summary section word allocation does not match target_word_count",
		)
	}

	if len(model.VerifiedResults) > 50 ||
		len(model.MissingInformation) > 50 {
		return Result{}, errors.New("annual summary result list is too large")
	}
	for index := range model.VerifiedResults {
		item := &model.VerifiedResults[index]
		item.Statement = strings.TrimSpace(item.Statement)
		if item.Statement == "" ||
			utf8.RuneCountInString(item.Statement) > 500 {
			return Result{}, errors.New("verified result statement is invalid")
		}
		references, err := validateReferences(item.EvidenceIDs, selectedIDs)
		if err != nil {
			return Result{}, fmt.Errorf("verified result evidence_ids: %w", err)
		}
		item.EvidenceIDs = references
	}
	for index := range model.MissingInformation {
		model.MissingInformation[index] = strings.TrimSpace(
			model.MissingInformation[index],
		)
		if model.MissingInformation[index] == "" ||
			utf8.RuneCountInString(model.MissingInformation[index]) > 500 {
			return Result{}, errors.New("missing information item is invalid")
		}
	}
	return Result{
		Title:              strings.TrimSpace(model.Title),
		TargetWordCount:    model.TargetWordCount,
		Sections:           model.Sections,
		VerifiedResults:    model.VerifiedResults,
		MissingInformation: model.MissingInformation,
	}, nil
}

func validateReferences(
	values []string,
	selectedIDs map[string]struct{},
) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New("must contain at least one evidence ID")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := selectedIDs[value]; !ok {
			return nil, fmt.Errorf("contains unknown evidence ID %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func appendUnavailableGaps(
	result *Result,
	unavailable []contextmanifest.UnavailableSource,
) {
	existing := make(map[string]struct{}, len(result.MissingInformation))
	for _, item := range result.MissingInformation {
		existing[item] = struct{}{}
	}
	sorted := append([]contextmanifest.UnavailableSource(nil), unavailable...)
	sort.Slice(sorted, func(left, right int) bool {
		return sorted[left].SourceRootID < sorted[right].SourceRootID
	})
	for _, source := range sorted {
		gap := fmt.Sprintf(
			"资料源“%s”当前不可用（%s），相关资料未纳入本次大纲。",
			source.Name,
			source.ReasonCode,
		)
		if _, duplicate := existing[gap]; duplicate {
			continue
		}
		existing[gap] = struct{}{}
		result.MissingInformation = append(result.MissingInformation, gap)
	}
}

func stripJSONCodeFence(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "```") {
		value = strings.TrimPrefix(value, "```json")
		value = strings.TrimPrefix(value, "```JSON")
		value = strings.TrimPrefix(value, "```")
		value = strings.TrimSuffix(value, "```")
	}
	return strings.TrimSpace(value)
}
