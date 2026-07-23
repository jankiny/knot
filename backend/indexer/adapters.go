package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"

	"knot-backend/extractors"
	"knot-backend/sources"
)

const adapterVersion = 1

type adapterError struct {
	code string
	err  error
}

func (err *adapterError) Error() string {
	return err.err.Error()
}

func (err *adapterError) Unwrap() error {
	return err.err
}

type workRecordAdapter struct {
	reader WorkRecordReader
}

func NewWorkRecordAdapter(reader WorkRecordReader) Adapter {
	return &workRecordAdapter{reader: reader}
}

func (adapter *workRecordAdapter) Name() string {
	return "work_record"
}

func (adapter *workRecordAdapter) Version() int {
	return adapterVersion
}

func (adapter *workRecordAdapter) Matches(relativePath string) bool {
	return strings.EqualFold(path.Base(relativePath), "工作记录.md")
}

func (adapter *workRecordAdapter) ReadsContentForMetadata() bool {
	return true
}

func (adapter *workRecordAdapter) Extract(
	ctx context.Context,
	input AdapterInput,
) (Extraction, error) {
	if err := ctx.Err(); err != nil {
		return Extraction{}, err
	}
	if adapter.reader == nil {
		return Extraction{}, &adapterError{
			code: ReasonParseError,
			err:  fmt.Errorf("work record reader is unavailable"),
		}
	}

	record, err := adapter.reader(input.AbsolutePath)
	if err != nil {
		return Extraction{}, &adapterError{
			code: ReasonParseError,
			err:  fmt.Errorf("read work record: %w", err),
		}
	}
	return Extraction{
		DocumentType:     DocumentTypeWorkRecord,
		Title:            fallbackTitle(record.Title, input.RelativePath),
		TaskDate:         extractors.NormalizeDate(record.TaskDate),
		ContentHash:      contentHash(record.RawContent),
		ContentExcerpt:   truncateRunes(record.Content, input.MaxExcerptRunes),
		DeclaredAIAccess: strings.TrimSpace(record.AIAccess),
	}, nil
}

type markdownAdapter struct{}

func NewMarkdownAdapter() Adapter {
	return markdownAdapter{}
}

func (markdownAdapter) Name() string {
	return "markdown"
}

func (markdownAdapter) Version() int {
	return adapterVersion
}

func (markdownAdapter) Matches(relativePath string) bool {
	extension := strings.ToLower(path.Ext(relativePath))
	return extension == ".md" || extension == ".markdown"
}

func (markdownAdapter) ReadsContentForMetadata() bool {
	return false
}

func (markdownAdapter) Extract(
	ctx context.Context,
	input AdapterInput,
) (Extraction, error) {
	data, err := readBoundedFile(ctx, input.AbsolutePath)
	if err != nil {
		return Extraction{}, err
	}
	result := extractors.ExtractMarkdown(
		data,
		input.RelativePath,
		extractors.MarkdownOptions{
			Journal:         input.SourceRoot.Kind == sources.KindJournal,
			MaxExcerptRunes: input.MaxExcerptRunes,
		},
	)
	return Extraction{
		DocumentType:   result.DocumentType,
		Title:          result.Title,
		TaskDate:       result.TaskDate,
		ContentHash:    contentHash(data),
		ContentExcerpt: result.Excerpt,
	}, nil
}

type textAdapter struct{}

func NewTextAdapter() Adapter {
	return textAdapter{}
}

func (textAdapter) Name() string {
	return "text"
}

func (textAdapter) Version() int {
	return adapterVersion
}

func (textAdapter) Matches(relativePath string) bool {
	return strings.EqualFold(path.Ext(relativePath), ".txt")
}

func (textAdapter) ReadsContentForMetadata() bool {
	return false
}

func (textAdapter) Extract(
	ctx context.Context,
	input AdapterInput,
) (Extraction, error) {
	data, err := readBoundedFile(ctx, input.AbsolutePath)
	if err != nil {
		return Extraction{}, err
	}
	return Extraction{
		DocumentType: DocumentTypeText,
		Title:        fallbackTitle("", input.RelativePath),
		ContentHash:  contentHash(data),
	}, nil
}

type fileAdapter struct{}

func NewFileAdapter() Adapter {
	return fileAdapter{}
}

func (fileAdapter) Name() string {
	return "file"
}

func (fileAdapter) Version() int {
	return adapterVersion
}

func (fileAdapter) Matches(string) bool {
	return true
}

func (fileAdapter) ReadsContentForMetadata() bool {
	return false
}

func (fileAdapter) Extract(
	ctx context.Context,
	input AdapterInput,
) (Extraction, error) {
	data, err := readBoundedFile(ctx, input.AbsolutePath)
	if err != nil {
		return Extraction{}, err
	}
	return Extraction{
		DocumentType: DocumentTypeFile,
		Title:        fallbackTitle("", input.RelativePath),
		ContentHash:  contentHash(data),
	}, nil
}

func readBoundedFile(ctx context.Context, filePath string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &adapterError{
			code: ReasonReadError,
			err:  fmt.Errorf("read indexed file: %w", err),
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func fallbackTitle(title, relativePath string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}
	name := path.Base(relativePath)
	extension := path.Ext(name)
	title = strings.TrimSpace(strings.TrimSuffix(name, extension))
	if title == "" {
		return name
	}
	return title
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
