package contextmanifest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"knot-backend/indexer"
	"knot-backend/policy"
	"knot-backend/safepath"
	"knot-backend/sources"
)

var annualSummarySourceKinds = map[sources.Kind]struct{}{
	sources.KindJournal:     {},
	sources.KindCurrentWork: {},
	sources.KindActiveWork:  {},
	sources.KindWorkArchive: {},
}

var accomplishmentKeywords = []string{
	"完成",
	"推进",
	"成果",
	"交付",
	"上线",
	"落地",
	"验收",
	"优化",
	"解决",
	"负责",
	"达成",
	"completed",
	"delivered",
	"launched",
	"improved",
}

type Resolver struct {
	registry        *sources.Registry
	pathResolver    *safepath.Resolver
	indexRepository *indexer.Repository
	repository      *Repository
	limits          Limits
	now             func() time.Time
	newManifestID   func() (string, error)
}

func NewResolver(
	registry *sources.Registry,
	pathResolver *safepath.Resolver,
	indexRepository *indexer.Repository,
	repository *Repository,
) *Resolver {
	return &Resolver{
		registry:        registry,
		pathResolver:    pathResolver,
		indexRepository: indexRepository,
		repository:      repository,
		limits:          DefaultLimits(),
		now:             time.Now,
		newManifestID: func() (string, error) {
			return newIdentifier("context")
		},
	}
}

func (resolver *Resolver) Discover(
	ctx context.Context,
	request DiscoverRequest,
) (ContextManifest, error) {
	request, periodStart, periodEnd, err := resolver.validateDiscover(request)
	if err != nil {
		return ContextManifest{}, err
	}
	discoveryContext, cancel := context.WithTimeout(ctx, resolver.limits.MaxDuration)
	defer cancel()
	ctx = discoveryContext

	roots, err := resolver.registry.List(ctx)
	if err != nil {
		return ContextManifest{}, fmt.Errorf("list context source roots: %w", err)
	}
	sort.Slice(roots, func(left, right int) bool {
		return roots[left].ID < roots[right].ID
	})

	now := resolver.now().UTC()
	manifestID, err := resolver.newManifestID()
	if err != nil {
		return ContextManifest{}, fmt.Errorf("generate context manifest id: %w", err)
	}
	manifest := ContextManifest{
		ID:                   manifestID,
		TaskType:             request.TaskType,
		Query:                request.Query,
		PeriodStart:          request.PeriodStart,
		PeriodEnd:            request.PeriodEnd,
		Sources:              make([]SourceSummary, 0),
		Evidence:             make([]EvidenceItem, 0),
		Excluded:             make([]ExcludedItem, 0),
		UnavailableSources:   make([]UnavailableSource, 0),
		TokenBudget:          resolver.limits.MaxInputTokens,
		RequiresConfirmation: true,
		Status:               StatusReady,
		Valid:                true,
		RequiresRediscovery:  false,
		InvalidatedReasons:   make([]string, 0),
		CreatedAt:            now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(resolver.limits.ManifestTTL).
			Format(time.RFC3339Nano),
	}

	sourceIndexes := make(map[string]int)
	candidates := make([]candidate, 0)
	processedDocuments := 0
	documentLimitReached := false

	for _, root := range roots {
		if _, allowed := annualSummarySourceKinds[root.Kind]; !allowed {
			continue
		}
		if !root.Enabled {
			addExcluded(&manifest, resolver.limits.MaxExcluded, ExcludedItem{
				SourceRootID: root.ID,
				ReasonCode:   string(policy.ReasonRootDisabled),
				Reason:       "资料源已停用。",
			})
			continue
		}
		if root.Availability != sources.AvailabilityOnline {
			manifest.UnavailableSources = append(
				manifest.UnavailableSources,
				unavailableSource(root, policy.ReasonRootOffline),
			)
			continue
		}
		if root.LocalAccess == policy.LocalAccessNone {
			manifest.UnavailableSources = append(
				manifest.UnavailableSources,
				unavailableSource(root, policy.ReasonLocalAccessDenied),
			)
			continue
		}

		sourceIndexes[root.ID] = len(manifest.Sources)
		manifest.Sources = append(manifest.Sources, SourceSummary{
			SourceRootID: root.ID,
			Name:         root.Name,
			Kind:         root.Kind,
			ScopeType:    root.ScopeType,
			ScopeID:      cloneString(root.ScopeID),
			ScopeName:    cloneString(root.ScopeName),
		})

		documents, err := resolver.indexRepository.ListBySource(ctx, root.ID)
		if err != nil {
			return ContextManifest{}, fmt.Errorf(
				"list indexed documents for source %q: %w",
				root.ID,
				err,
			)
		}
		for _, document := range documents {
			if err := ctx.Err(); err != nil {
				return ContextManifest{}, err
			}
			if processedDocuments >= resolver.limits.MaxDocuments {
				addExcluded(&manifest, resolver.limits.MaxExcluded, ExcludedItem{
					SourceRootID: root.ID,
					ReasonCode:   ReasonCandidateLimit,
					Reason:       "候选文档数量已达到本地发现上限。",
				})
				documentLimitReached = true
				break
			}
			processedDocuments++

			statusReason, statusDescription := indexStatusExclusion(document)
			if statusReason != "" {
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					statusReason,
					statusDescription,
				)
				continue
			}

			resolved, err := resolver.pathResolver.Resolve(
				ctx,
				safepath.Request{
					SourceRootID: root.ID,
					RelativePath: document.RelativePath,
				},
			)
			if err != nil {
				reason := pathReason(err)
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					reason,
					exclusionDescription(reason),
				)
				continue
			}

			info, err := os.Stat(resolved.CanonicalPath)
			if err != nil {
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					ReasonReadError,
					"源文件当前不可读取。",
				)
				continue
			}
			if info.IsDir() || !indexMetadataMatches(document, info) {
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					ReasonIndexOutdated,
					"源文件元数据已变化，需要重新扫描资料源。",
				)
				continue
			}

			decision, err := applyDocumentPolicy(
				resolved.Policy,
				document.DocumentAIAccess(),
				resolved.AbsolutePath,
			)
			if err != nil {
				return ContextManifest{}, fmt.Errorf(
					"evaluate document policy: %w",
					err,
				)
			}
			if !decision.AllowsAIMetadata() {
				reason := deniedReason(decision)
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					reason,
					exclusionDescription(reason),
				)
				continue
			}

			documentDate, usedTaskDate, ok := effectiveDocumentDate(document)
			if !ok ||
				documentDate.Before(periodStart) ||
				documentDate.After(periodEnd) {
				addDocumentExcluded(
					&manifest,
					resolver.limits.MaxExcluded,
					document,
					ReasonOutsidePeriod,
					"文档日期不在请求时间范围内。",
				)
				continue
			}

			sourceIndex := sourceIndexes[root.ID]
			manifest.Sources[sourceIndex].CandidateDocuments++
			score, reasons := rankCandidate(
				request.Query,
				root,
				document,
				documentDate,
				usedTaskDate,
				periodEnd,
				decision,
			)
			candidates = append(candidates, candidate{
				root:       root,
				document:   document,
				decision:   decision,
				date:       documentDate.Format("2006-01-02"),
				score:      score,
				reasonText: strings.Join(reasons, "；"),
			})
		}
		if documentLimitReached {
			break
		}
	}

	sortCandidates(candidates)
	manifest.EstimatedInputTokens = estimateRequestTokens(request)
	snapshots := make([]documentSnapshot, 0)
	for _, candidate := range candidates {
		excerpt := ""
		reason := candidate.reasonText
		if candidate.decision.AllowsAIContent() {
			excerpt = trimRunes(
				strings.TrimSpace(candidate.document.ContentExcerpt),
				resolver.limits.MaxEvidenceRunes,
			)
			if excerpt == "" {
				reason += "；当前索引仅提供元数据"
			} else {
				reason += "；采用受限正文片段"
			}
		} else {
			reason += "；当前策略仅允许元数据"
		}

		evidence := EvidenceItem{
			ID: evidenceIdentifier(
				candidate.document,
				candidate.decision.AIAccess,
				excerpt,
			),
			DocumentID:        candidate.document.ID,
			SourceRootID:      candidate.root.ID,
			SourceType:        candidate.document.DocumentType,
			Title:             candidate.document.Title,
			Date:              candidate.date,
			Project:           sourceProject(candidate.root),
			Excerpt:           excerpt,
			Reason:            reason,
			AIAccessEffective: candidate.decision.AIAccess,
		}
		evidenceTokens := estimateEvidenceTokens(evidence)
		if len(manifest.Evidence) >= resolver.limits.MaxEvidence ||
			evidenceTokens >
				resolver.limits.MaxInputTokens-manifest.EstimatedInputTokens {
			addDocumentExcluded(
				&manifest,
				resolver.limits.MaxExcluded,
				candidate.document,
				ReasonTokenBudget,
				"候选排序较低，超出 evidence 数量或 token 预算。",
			)
			continue
		}

		manifest.EstimatedInputTokens += evidenceTokens
		manifest.Evidence = append(manifest.Evidence, evidence)
		sourceIndex := sourceIndexes[candidate.root.ID]
		manifest.Sources[sourceIndex].EvidenceItems++
		snapshots = append(snapshots, documentSnapshot{
			DocumentID:        candidate.document.ID,
			SourceRootID:      candidate.document.SourceRootID,
			RelativePath:      candidate.document.RelativePath,
			ContentHash:       candidate.document.ContentHash,
			ModifiedAt:        candidate.document.ModifiedAt,
			FileSize:          candidate.document.FileSize,
			AIAccessEffective: candidate.decision.AIAccess,
		})
	}

	sort.Slice(manifest.UnavailableSources, func(left, right int) bool {
		return manifest.UnavailableSources[left].SourceRootID <
			manifest.UnavailableSources[right].SourceRootID
	})
	sort.Slice(manifest.Excluded, func(left, right int) bool {
		leftKey := excludedSortKey(manifest.Excluded[left])
		rightKey := excludedSortKey(manifest.Excluded[right])
		return leftKey < rightKey
	})

	snapshotHash := calculateSnapshotHash(manifest, snapshots)
	if err := resolver.repository.Save(
		ctx,
		manifest,
		snapshotHash,
		snapshots,
	); err != nil {
		return ContextManifest{}, err
	}
	return manifest, nil
}

func (resolver *Resolver) Get(
	ctx context.Context,
	manifestID string,
) (ContextManifest, error) {
	if err := resolver.validate(); err != nil {
		return ContextManifest{}, err
	}
	validationContext, cancel := context.WithTimeout(
		ctx,
		resolver.limits.MaxDuration,
	)
	defer cancel()
	ctx = validationContext
	manifestID = strings.TrimSpace(manifestID)
	if manifestID == "" {
		return ContextManifest{}, ErrManifestNotFound
	}

	manifest, storedHash, snapshots, err := resolver.repository.Get(
		ctx,
		manifestID,
	)
	if err != nil {
		return ContextManifest{}, err
	}
	invalidated := make(map[string]struct{})
	expiresAt, err := time.Parse(time.RFC3339Nano, manifest.ExpiresAt)
	if err != nil || !resolver.now().UTC().Before(expiresAt) {
		invalidated[ReasonManifestExpired] = struct{}{}
	}
	if calculateSnapshotHash(manifest, snapshots) != storedHash {
		invalidated[ReasonDocumentChanged] = struct{}{}
	}

	for _, snapshot := range snapshots {
		if err := ctx.Err(); err != nil {
			return ContextManifest{}, err
		}
		document, err := resolver.indexRepository.GetByID(
			ctx,
			snapshot.DocumentID,
		)
		if errors.Is(err, indexer.ErrDocumentNotFound) {
			invalidated[ReasonDocumentMissing] = struct{}{}
			continue
		}
		if err != nil {
			return ContextManifest{}, err
		}
		if document.IndexStatus != indexer.IndexStatusReady ||
			document.SourceRootID != snapshot.SourceRootID ||
			document.RelativePath != snapshot.RelativePath {
			invalidated[ReasonDocumentChanged] = struct{}{}
			continue
		}
		if document.ContentHash != snapshot.ContentHash {
			invalidated[ReasonDocumentHashChanged] = struct{}{}
		}
		if document.ModifiedAt != snapshot.ModifiedAt ||
			document.FileSize != snapshot.FileSize {
			invalidated[ReasonDocumentChanged] = struct{}{}
		}

		resolved, err := resolver.pathResolver.Resolve(
			ctx,
			safepath.Request{
				SourceRootID: snapshot.SourceRootID,
				RelativePath: snapshot.RelativePath,
			},
		)
		if err != nil {
			invalidated[pathReason(err)] = struct{}{}
			continue
		}
		info, err := os.Stat(resolved.CanonicalPath)
		if err != nil {
			invalidated[ReasonDocumentMissing] = struct{}{}
			continue
		}
		if info.IsDir() || !indexMetadataMatches(document, info) {
			invalidated[ReasonDocumentChanged] = struct{}{}
		}

		decision, err := applyDocumentPolicy(
			resolved.Policy,
			document.DocumentAIAccess(),
			resolved.AbsolutePath,
		)
		if err != nil {
			return ContextManifest{}, err
		}
		if !decision.AllowsAIMetadata() ||
			decision.AIAccess != snapshot.AIAccessEffective {
			invalidated[ReasonPolicyChanged] = struct{}{}
		}
	}

	manifest.InvalidatedReasons = sortedReasonSet(invalidated)
	manifest.Valid = len(manifest.InvalidatedReasons) == 0
	manifest.RequiresRediscovery = !manifest.Valid
	if manifest.Valid {
		manifest.Status = StatusReady
	} else {
		manifest.Status = StatusInvalid
	}
	return manifest, nil
}

type candidate struct {
	root       sources.SourceRoot
	document   indexer.IndexedDocument
	decision   policy.Decision
	date       string
	score      int
	reasonText string
}

func (resolver *Resolver) validateDiscover(
	request DiscoverRequest,
) (DiscoverRequest, time.Time, time.Time, error) {
	if err := resolver.validate(); err != nil {
		return DiscoverRequest{}, time.Time{}, time.Time{}, err
	}
	request.TaskType = strings.TrimSpace(request.TaskType)
	request.Query = strings.TrimSpace(request.Query)
	request.PeriodStart = strings.TrimSpace(request.PeriodStart)
	request.PeriodEnd = strings.TrimSpace(request.PeriodEnd)

	if request.TaskType != TaskTypePersonalAnnualSummary {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "task_type",
			Message: "must be personal_annual_summary",
		}
	}
	if request.Query == "" {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "query",
			Message: "is required",
		}
	}
	if utf8.RuneCountInString(request.Query) > resolver.limits.MaxQueryRunes {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "query",
			Message: "is too long",
		}
	}
	periodStart, err := parseDate(request.PeriodStart)
	if err != nil {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "period_start",
			Message: "must be a valid YYYY-MM-DD date",
		}
	}
	periodEnd, err := parseDate(request.PeriodEnd)
	if err != nil {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "period_end",
			Message: "must be a valid YYYY-MM-DD date",
		}
	}
	if periodStart.After(periodEnd) {
		return DiscoverRequest{}, time.Time{}, time.Time{}, &ValidationError{
			Field:   "period_start",
			Message: "must not be after period_end",
		}
	}
	return request, periodStart, periodEnd, nil
}

func (resolver *Resolver) validate() error {
	switch {
	case resolver == nil:
		return errors.New("context resolver is unavailable")
	case resolver.registry == nil:
		return errors.New("source registry is unavailable")
	case resolver.pathResolver == nil:
		return errors.New("safe path resolver is unavailable")
	case resolver.indexRepository == nil:
		return errors.New("index repository is unavailable")
	case resolver.repository == nil || resolver.repository.db == nil:
		return errors.New("context manifest repository is unavailable")
	case resolver.limits.MaxDocuments <= 0,
		resolver.limits.MaxEvidence <= 0,
		resolver.limits.MaxExcluded <= 0,
		resolver.limits.MaxEvidenceRunes <= 0,
		resolver.limits.MaxInputTokens <= 0,
		resolver.limits.MaxQueryRunes <= 0,
		resolver.limits.MaxDuration <= 0,
		resolver.limits.ManifestTTL <= 0:
		return errors.New("context discovery limits must all be positive")
	default:
		return nil
	}
}

func indexStatusExclusion(document indexer.IndexedDocument) (string, string) {
	switch document.IndexStatus {
	case indexer.IndexStatusReady:
		return "", ""
	case indexer.IndexStatusStale:
		return ReasonIndexStale, "索引文档已过期，需要重新扫描资料源。"
	case indexer.IndexStatusDeleted:
		return ReasonIndexDeleted, "索引记录对应的文件已删除或移动。"
	default:
		return ReasonIndexError, "索引文档不可用，需要修复扫描问题。"
	}
}

func indexMetadataMatches(
	document indexer.IndexedDocument,
	info os.FileInfo,
) bool {
	return document.FileSize == info.Size() &&
		document.ModifiedAt ==
			info.ModTime().UTC().Format(time.RFC3339Nano)
}

func effectiveDocumentDate(
	document indexer.IndexedDocument,
) (time.Time, bool, bool) {
	if value, err := parseDate(document.TaskDate); err == nil {
		return value, true, true
	}
	modifiedAt, err := time.Parse(time.RFC3339Nano, document.ModifiedAt)
	if err != nil {
		return time.Time{}, false, false
	}
	value, err := parseDate(modifiedAt.UTC().Format("2006-01-02"))
	return value, false, err == nil
}

func parseDate(value string) (time.Time, error) {
	if len(value) != len("2006-01-02") {
		return time.Time{}, errors.New("invalid date length")
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, errors.New("invalid date")
	}
	return parsed, nil
}

func rankCandidate(
	query string,
	root sources.SourceRoot,
	document indexer.IndexedDocument,
	documentDate time.Time,
	usedTaskDate bool,
	periodEnd time.Time,
	decision policy.Decision,
) (int, []string) {
	score := 100
	reasons := []string{"时间范围匹配"}
	if usedTaskDate {
		score += 20
		reasons = append(reasons, "使用文档任务日期")
	} else {
		reasons = append(reasons, "使用最近修改日期")
	}

	typeScore, typeReason := documentTypeRank(document.DocumentType)
	score += typeScore
	reasons = append(reasons, typeReason)

	if queryAssociated(query, root, document) {
		score += 20
		reasons = append(reasons, "标题或项目与任务描述关联")
	}
	accomplishmentCount := keywordMatchCount(document.ContentExcerpt)
	if accomplishmentCount > 0 {
		if accomplishmentCount > 3 {
			accomplishmentCount = 3
		}
		score += accomplishmentCount * 10
		reasons = append(reasons, "包含完成、推进或成果表述")
	}

	daysAgo := int(periodEnd.Sub(documentDate).Hours() / 24)
	switch {
	case daysAgo <= 30:
		score += 15
		reasons = append(reasons, "最近一个月有活动")
	case daysAgo <= 90:
		score += 10
		reasons = append(reasons, "最近三个月有活动")
	case daysAgo <= 180:
		score += 5
		reasons = append(reasons, "最近半年有活动")
	}
	if decision.AIAccess == policy.AIAccessMetadata {
		reasons = append(reasons, "权限限制为元数据")
	}
	return score, reasons
}

func documentTypeRank(documentType string) (int, string) {
	switch documentType {
	case indexer.DocumentTypeWorkRecord:
		return 45, "工作记录优先"
	case "journal_weekly":
		return 40, "周报优先"
	case "journal_daily":
		return 35, "日报优先"
	case "markdown":
		return 20, "Markdown 元数据匹配"
	case indexer.DocumentTypeText:
		return 15, "TXT 元数据匹配"
	default:
		return 10, "文件元数据匹配"
	}
}

func queryAssociated(
	query string,
	root sources.SourceRoot,
	document indexer.IndexedDocument,
) bool {
	query = strings.ToLower(query)
	values := []string{document.Title}
	if root.ScopeName != nil {
		values = append(values, *root.ScopeName)
	}
	if document.ProjectID != nil {
		values = append(values, *document.ProjectID)
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if utf8.RuneCountInString(value) >= 2 &&
			strings.Contains(query, value) {
			return true
		}
	}

	terms := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	haystack := strings.ToLower(
		document.Title + " " + sourceProject(root),
	)
	for _, term := range terms {
		if utf8.RuneCountInString(term) >= 2 &&
			strings.Contains(haystack, term) {
			return true
		}
	}
	return false
}

func keywordMatchCount(value string) int {
	value = strings.ToLower(value)
	count := 0
	for _, keyword := range accomplishmentKeywords {
		if strings.Contains(value, keyword) {
			count++
		}
	}
	return count
}

func sortCandidates(candidates []candidate) {
	sort.Slice(candidates, func(left, right int) bool {
		switch {
		case candidates[left].score != candidates[right].score:
			return candidates[left].score > candidates[right].score
		case candidates[left].date != candidates[right].date:
			return candidates[left].date > candidates[right].date
		case candidates[left].root.ID != candidates[right].root.ID:
			return candidates[left].root.ID < candidates[right].root.ID
		case candidates[left].document.RelativePath !=
			candidates[right].document.RelativePath:
			return candidates[left].document.RelativePath <
				candidates[right].document.RelativePath
		default:
			return candidates[left].document.ID <
				candidates[right].document.ID
		}
	})
}

func estimateRequestTokens(request DiscoverRequest) int {
	return 64 +
		estimateTextTokens(request.TaskType) +
		estimateTextTokens(request.Query) +
		estimateTextTokens(request.PeriodStart) +
		estimateTextTokens(request.PeriodEnd)
}

func estimateEvidenceTokens(evidence EvidenceItem) int {
	return 40 +
		estimateTextTokens(evidence.ID) +
		estimateTextTokens(evidence.SourceType) +
		estimateTextTokens(evidence.Title) +
		estimateTextTokens(evidence.Date) +
		estimateTextTokens(evidence.Project) +
		estimateTextTokens(evidence.Excerpt) +
		estimateTextTokens(evidence.Reason)
}

func estimateTextTokens(value string) int {
	asciiRunes := 0
	nonASCIIRunes := 0
	for _, character := range value {
		if character <= unicode.MaxASCII {
			asciiRunes++
		} else {
			nonASCIIRunes++
		}
	}
	return (asciiRunes+3)/4 + nonASCIIRunes
}

func trimRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func evidenceIdentifier(
	document indexer.IndexedDocument,
	access policy.AIAccess,
	excerpt string,
) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{
		"evidence-v1",
		document.ID,
		document.ContentHash,
		document.ModifiedAt,
		string(access),
		excerpt,
	}, "\x00")))
	return "evidence_" + hex.EncodeToString(hash[:12])
}

func calculateSnapshotHash(
	manifest ContextManifest,
	snapshots []documentSnapshot,
) string {
	sorted := append([]documentSnapshot(nil), snapshots...)
	sort.Slice(sorted, func(left, right int) bool {
		return sorted[left].DocumentID < sorted[right].DocumentID
	})
	builder := strings.Builder{}
	builder.WriteString(manifest.TaskType)
	builder.WriteByte(0)
	builder.WriteString(manifest.Query)
	builder.WriteByte(0)
	builder.WriteString(manifest.PeriodStart)
	builder.WriteByte(0)
	builder.WriteString(manifest.PeriodEnd)
	for _, snapshot := range sorted {
		builder.WriteByte(0)
		builder.WriteString(snapshot.DocumentID)
		builder.WriteByte(0)
		builder.WriteString(snapshot.SourceRootID)
		builder.WriteByte(0)
		builder.WriteString(snapshot.RelativePath)
		builder.WriteByte(0)
		builder.WriteString(snapshot.ContentHash)
		builder.WriteByte(0)
		builder.WriteString(snapshot.ModifiedAt)
		builder.WriteByte(0)
		builder.WriteString(fmt.Sprintf("%d", snapshot.FileSize))
		builder.WriteByte(0)
		builder.WriteString(string(snapshot.AIAccessEffective))
	}
	hash := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func applyDocumentPolicy(
	base policy.Decision,
	documentAccess *policy.AIAccess,
	filePath string,
) (policy.Decision, error) {
	access := policy.Access{
		LocalAccess: base.LocalAccess,
		AIAccess:    base.AIAccess,
	}
	return policy.Evaluate(policy.Evaluation{
		Defaults: access,
		Source:   access,
		Document: &policy.Override{AIAccess: documentAccess},
		Path:     filePath,
	})
}

func deniedReason(decision policy.Decision) string {
	if decision.HasReason(policy.ReasonSensitivePathMarker) {
		return string(policy.ReasonSensitivePathMarker)
	}
	return string(policy.ReasonAIAccessDenied)
}

func pathReason(err error) string {
	if reason, ok := safepath.Reason(err); ok {
		return string(reason)
	}
	return ReasonReadError
}

func exclusionDescription(reason string) string {
	switch policy.ReasonCode(reason) {
	case policy.ReasonRootDisabled:
		return "资料源已停用。"
	case policy.ReasonRootOffline:
		return "资料源当前离线、缺失或不可用。"
	case policy.ReasonLocalAccessDenied:
		return "当前策略不允许 Knot 在本地读取。"
	case policy.ReasonSensitivePathMarker:
		return "敏感路径标记禁止将元数据或正文用于 AI。"
	case policy.ReasonAIAccessDenied:
		return "当前策略禁止将元数据或正文用于 AI。"
	case policy.ReasonPathOutsideRoot:
		return "相对路径无法在登记资料源边界内解析。"
	case policy.ReasonSymlinkEscape:
		return "链接目标越出登记资料源边界。"
	default:
		return "候选当前不可用。"
	}
}

func unavailableSource(
	root sources.SourceRoot,
	reason policy.ReasonCode,
) UnavailableSource {
	return UnavailableSource{
		SourceRootID: root.ID,
		Name:         root.Name,
		Kind:         root.Kind,
		Availability: root.Availability,
		ReasonCode:   string(reason),
		Reason:       exclusionDescription(string(reason)),
	}
}

func addDocumentExcluded(
	manifest *ContextManifest,
	limit int,
	document indexer.IndexedDocument,
	reasonCode string,
	reason string,
) {
	addExcluded(manifest, limit, ExcludedItem{
		SourceRootID: document.SourceRootID,
		DocumentID:   document.ID,
		RelativePath: document.RelativePath,
		Title:        document.Title,
		ReasonCode:   reasonCode,
		Reason:       reason,
	})
}

func addExcluded(
	manifest *ContextManifest,
	limit int,
	excluded ExcludedItem,
) {
	manifest.ExcludedCount++
	if len(manifest.Excluded) < limit {
		manifest.Excluded = append(manifest.Excluded, excluded)
		return
	}
	manifest.OmittedExcludedCount++
}

func excludedSortKey(excluded ExcludedItem) string {
	return strings.Join([]string{
		excluded.SourceRootID,
		excluded.DocumentID,
		excluded.RelativePath,
		excluded.ReasonCode,
	}, "\x00")
}

func sourceProject(root sources.SourceRoot) string {
	if root.ScopeType == sources.ScopeProject && root.ScopeName != nil {
		return *root.ScopeName
	}
	return ""
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func sortedReasonSet(reasons map[string]struct{}) []string {
	result := make([]string, 0, len(reasons))
	for reason := range reasons {
		result = append(result, reason)
	}
	sort.Strings(result)
	return result
}

func newIdentifier(prefix string) (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(value), nil
}
