package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func isSensitivePath(value string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(value)))
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

func isNoAIAccess(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer("_", "", "-", "", " ", "").Replace(normalized)
	switch normalized {
	case "noai", "none", "disabled", "deny", "denied", "forbidden", "private":
		return true
	default:
		return false
	}
}

func isAIRestrictedFolderPath(folderPath string) bool {
	folderPath = strings.TrimSpace(folderPath)
	if folderPath == "" {
		return false
	}
	if isSensitivePath(folderPath) {
		return true
	}
	info, err := parseWorkRecord(filepath.Join(folderPath, workRecordFileName))
	return err == nil && isNoAIAccess(info.AIAccess)
}

func validateTaskFolder(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("任务目录不能为空")
	}

	folderPath := filepath.Clean(value)
	if isSensitivePath(folderPath) {
		return "", fmt.Errorf("敏感路径不允许由 Knot 处理")
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
