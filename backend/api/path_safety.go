package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knot-backend/policy"
	"knot-backend/safepath"
)

func isSensitivePath(value string) bool {
	return policy.HasSensitivePathMarker(value)
}

func isNoAIAccess(value string) bool {
	access, present, err := policy.ParseLegacyAIAccess(value)
	return present && (err != nil || access == policy.AIAccessNone)
}

func isAIRestrictedFolderPath(folderPath string) bool {
	folderPath = strings.TrimSpace(folderPath)
	if folderPath == "" {
		return false
	}

	var documentOverride *policy.Override
	info, err := parseWorkRecord(filepath.Join(folderPath, workRecordFileName))
	if err == nil && strings.TrimSpace(info.AIAccess) != "" {
		aiAccess, present, parseErr := policy.ParseLegacyAIAccess(info.AIAccess)
		if parseErr != nil {
			return true
		}
		if present {
			documentOverride = &policy.Override{AIAccess: &aiAccess}
		}
	}

	decision, err := policy.Evaluate(policy.Evaluation{
		Defaults: policy.Access{
			LocalAccess: policy.LocalAccessRead,
			AIAccess:    policy.AIAccessContent,
		},
		Source: policy.Access{
			LocalAccess: policy.LocalAccessRead,
			AIAccess:    policy.AIAccessContent,
		},
		Document: documentOverride,
		Path:     folderPath,
	})
	return err != nil || !decision.AllowsAIContent()
}

func validateTaskFolder(value string) (string, error) {
	return validateTaskFolderForLocal(
		context.Background(),
		nil,
		value,
		false,
	)
}

func validateTaskFolderForLocal(
	ctx context.Context,
	resolver *safepath.Resolver,
	value string,
	requireWrite bool,
) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("任务目录不能为空")
	}

	folderPath := filepath.Clean(value)
	if isSensitivePath(folderPath) {
		resolved, err := resolveRegisteredLocalPath(
			ctx,
			resolver,
			folderPath,
			requireWrite,
		)
		if err != nil {
			return "", err
		}
		folderPath = resolved
	}

	info, err := os.Stat(folderPath)
	if err != nil {
		return "", fmt.Errorf("任务目录不存在: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("任务路径不是目录")
	}

	recordInfo, err := os.Stat(filepath.Join(folderPath, workRecordFileName))
	if err != nil {
		return "", fmt.Errorf("目录中缺少 %s", workRecordFileName)
	}
	if recordInfo.IsDir() {
		return "", fmt.Errorf("%s 不是有效文件", workRecordFileName)
	}

	return folderPath, nil
}

func resolveRegisteredLocalPath(
	ctx context.Context,
	resolver *safepath.Resolver,
	value string,
	requireWrite bool,
) (string, error) {
	if resolver == nil {
		return "", fmt.Errorf("敏感路径必须先登记为资料源")
	}
	result, err := resolver.ResolveRegisteredAbsolute(ctx, value)
	if err != nil {
		return "", fmt.Errorf("敏感路径未通过资料源安全校验: %w", err)
	}
	if requireWrite && !result.Policy.AllowsLocalWrite() {
		return "", fmt.Errorf("资料源不允许本地写入")
	}
	if !result.Policy.AllowsLocalRead() {
		return "", fmt.Errorf("资料源不允许本地读取")
	}
	return result.AbsolutePath, nil
}

func localPathAuthorizer(
	ctx context.Context,
	resolver *safepath.Resolver,
	requireWrite bool,
) func(string) bool {
	cache := make(map[string]bool)
	return func(value string) bool {
		key := normalizePathKey(value)
		if allowed, exists := cache[key]; exists {
			return allowed
		}
		_, err := resolveRegisteredLocalPath(ctx, resolver, value, requireWrite)
		allowed := err == nil
		cache[key] = allowed
		return allowed
	}
}
