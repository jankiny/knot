// Package policy calculates effective local and AI access without performing
// filesystem reads or trusting callers to widen a more restrictive layer.
package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

type LocalAccess string

const (
	LocalAccessNone      LocalAccess = "none"
	LocalAccessRead      LocalAccess = "read"
	LocalAccessReadWrite LocalAccess = "read_write"
)

type AIAccess string

const (
	AIAccessNone     AIAccess = "none"
	AIAccessMetadata AIAccess = "metadata"
	AIAccessContent  AIAccess = "content"
)

type ReasonCode string

const (
	ReasonRootDisabled        ReasonCode = "root_disabled"
	ReasonRootOffline         ReasonCode = "root_offline"
	ReasonLocalAccessDenied   ReasonCode = "local_access_denied"
	ReasonAIAccessDenied      ReasonCode = "ai_access_denied"
	ReasonMetadataOnly        ReasonCode = "metadata_only"
	ReasonSensitivePathMarker ReasonCode = "sensitive_path_marker"
	ReasonPathOutsideRoot     ReasonCode = "path_outside_root"
	ReasonSymlinkEscape       ReasonCode = "symlink_escape"
	ReasonUnsupportedType     ReasonCode = "unsupported_type"
	ReasonSizeLimit           ReasonCode = "size_limit"
)

var sensitivePathMarkers = []string{
	"noai",
	"private",
	"隐私",
	"身份证",
	"银行卡",
	"手机号",
	"学号信息",
	"人员名单",
	"家庭资料",
	"合同原件",
	"个人信息",
}

type Access struct {
	LocalAccess LocalAccess
	AIAccess    AIAccess
}

type Override struct {
	LocalAccess *LocalAccess
	AIAccess    *AIAccess
}

// Evaluation represents access declarations from least specific to most
// specific. Every layer is a cap: a more permissive value can never widen an
// access level already restricted by another layer.
type Evaluation struct {
	Defaults Access
	Source   Access
	Project  *Override
	Document *Override
	System   *Override

	Path         string
	RootDisabled bool
	RootOffline  bool
}

type Decision struct {
	LocalAccess LocalAccess  `json:"local_access"`
	AIAccess    AIAccess     `json:"ai_access"`
	Reasons     []ReasonCode `json:"reasons"`
}

func (value LocalAccess) Valid() bool {
	switch value {
	case LocalAccessNone, LocalAccessRead, LocalAccessReadWrite:
		return true
	default:
		return false
	}
}

func (value AIAccess) Valid() bool {
	switch value {
	case AIAccessNone, AIAccessMetadata, AIAccessContent:
		return true
	default:
		return false
	}
}

func (decision Decision) AllowsLocalRead() bool {
	return decision.LocalAccess == LocalAccessRead ||
		decision.LocalAccess == LocalAccessReadWrite
}

func (decision Decision) AllowsLocalWrite() bool {
	return decision.LocalAccess == LocalAccessReadWrite
}

func (decision Decision) AllowsAIMetadata() bool {
	return decision.AIAccess == AIAccessMetadata ||
		decision.AIAccess == AIAccessContent
}

func (decision Decision) AllowsAIContent() bool {
	return decision.AIAccess == AIAccessContent
}

func (decision Decision) HasReason(code ReasonCode) bool {
	for _, reason := range decision.Reasons {
		if reason == code {
			return true
		}
	}
	return false
}

func Evaluate(input Evaluation) (Decision, error) {
	if err := validateAccess("defaults", input.Defaults); err != nil {
		return Decision{}, err
	}
	if err := validateAccess("source", input.Source); err != nil {
		return Decision{}, err
	}
	if err := validateOverride("project", input.Project); err != nil {
		return Decision{}, err
	}
	if err := validateOverride("document", input.Document); err != nil {
		return Decision{}, err
	}
	if err := validateOverride("system", input.System); err != nil {
		return Decision{}, err
	}

	decision := Decision{
		LocalAccess: input.Defaults.LocalAccess,
		AIAccess:    input.Defaults.AIAccess,
		Reasons:     make([]ReasonCode, 0, 4),
	}
	applyAccess(&decision, input.Source)
	applyOverride(&decision, input.Project)
	applyOverride(&decision, input.Document)

	if HasSensitivePathMarker(input.Path) {
		decision.AIAccess = AIAccessNone
		decision.addReason(ReasonSensitivePathMarker)
	}

	applyOverride(&decision, input.System)

	if input.RootDisabled {
		decision.LocalAccess = LocalAccessNone
		decision.AIAccess = AIAccessNone
		decision.addReason(ReasonRootDisabled)
	}
	if input.RootOffline {
		decision.LocalAccess = LocalAccessNone
		decision.AIAccess = AIAccessNone
		decision.addReason(ReasonRootOffline)
	}

	// AI cannot expose content that Knot is not allowed to read locally.
	if decision.LocalAccess == LocalAccessNone {
		decision.AIAccess = AIAccessNone
		decision.addReason(ReasonLocalAccessDenied)
	}
	switch decision.AIAccess {
	case AIAccessNone:
		decision.addReason(ReasonAIAccessDenied)
	case AIAccessMetadata:
		decision.addReason(ReasonMetadataOnly)
	}

	return decision, nil
}

func HasSensitivePathMarker(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, `\`, "/")
	normalized = filepath.ToSlash(normalized)
	if normalized == "" {
		return false
	}
	for _, marker := range sensitivePathMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// ParseLegacyAIAccess maps historical work-record spellings onto the stable
// enum. Empty values have no explicit override.
func ParseLegacyAIAccess(value string) (AIAccess, bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer("_", "", "-", "", " ", "").Replace(normalized)
	if normalized == "" {
		return "", false, nil
	}
	switch normalized {
	case "noai", "none", "disabled", "deny", "denied", "forbidden", "private":
		return AIAccessNone, true, nil
	case "metadata", "metadataonly", "meta":
		return AIAccessMetadata, true, nil
	case "content", "full", "allow", "allowed", "enabled":
		return AIAccessContent, true, nil
	default:
		return "", true, fmt.Errorf("unsupported legacy ai_access %q", value)
	}
}

func validateAccess(name string, access Access) error {
	if !access.LocalAccess.Valid() {
		return fmt.Errorf("%s local_access %q is not supported", name, access.LocalAccess)
	}
	if !access.AIAccess.Valid() {
		return fmt.Errorf("%s ai_access %q is not supported", name, access.AIAccess)
	}
	return nil
}

func validateOverride(name string, override *Override) error {
	if override == nil {
		return nil
	}
	if override.LocalAccess != nil && !override.LocalAccess.Valid() {
		return fmt.Errorf("%s local_access %q is not supported", name, *override.LocalAccess)
	}
	if override.AIAccess != nil && !override.AIAccess.Valid() {
		return fmt.Errorf("%s ai_access %q is not supported", name, *override.AIAccess)
	}
	return nil
}

func applyAccess(decision *Decision, access Access) {
	decision.LocalAccess = stricterLocalAccess(decision.LocalAccess, access.LocalAccess)
	decision.AIAccess = stricterAIAccess(decision.AIAccess, access.AIAccess)
}

func applyOverride(decision *Decision, override *Override) {
	if override == nil {
		return
	}
	if override.LocalAccess != nil {
		decision.LocalAccess = stricterLocalAccess(decision.LocalAccess, *override.LocalAccess)
	}
	if override.AIAccess != nil {
		decision.AIAccess = stricterAIAccess(decision.AIAccess, *override.AIAccess)
	}
}

func stricterLocalAccess(left, right LocalAccess) LocalAccess {
	if localAccessRank(left) <= localAccessRank(right) {
		return left
	}
	return right
}

func stricterAIAccess(left, right AIAccess) AIAccess {
	if aiAccessRank(left) <= aiAccessRank(right) {
		return left
	}
	return right
}

func localAccessRank(value LocalAccess) int {
	switch value {
	case LocalAccessNone:
		return 0
	case LocalAccessRead:
		return 1
	case LocalAccessReadWrite:
		return 2
	default:
		return -1
	}
}

func aiAccessRank(value AIAccess) int {
	switch value {
	case AIAccessNone:
		return 0
	case AIAccessMetadata:
		return 1
	case AIAccessContent:
		return 2
	default:
		return -1
	}
}

func (decision *Decision) addReason(code ReasonCode) {
	if !decision.HasReason(code) {
		decision.Reasons = append(decision.Reasons, code)
	}
}
