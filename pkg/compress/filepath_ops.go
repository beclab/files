package compress

import (
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
)

func CollectFiles(root string) []string {
	var results []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			klog.Errorf("Walk error: %v", err)
			return nil
		}
		//if !d.IsDir() {
		results = append(results, path)
		//}
		return nil
	})
	if err != nil {
		klog.Errorf("Walk error: %v", err)
	}
	return results
}

// CalculateCommonParentDir 计算路径列表的公共父目录
func CalculateCommonParentDir(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	// 规范化路径：确保目录以分隔符结尾
	var normalized []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// 获取绝对路径并清理
		absPath, err := filepath.Abs(p)
		if err != nil {
			normalized = append(normalized, p)
			continue
		}

		cleaned := filepath.Clean(absPath)

		// 目录强制以分隔符结尾
		if strings.HasSuffix(p, "/") || isDirectory(cleaned) {
			normalized = append(normalized, cleaned+"/")
		} else {
			normalized = append(normalized, cleaned)
		}
	}

	if len(normalized) == 0 {
		return ""
	}

	// 单路径特殊处理
	if len(normalized) == 1 {
		p := normalized[0]
		if strings.HasSuffix(p, "/") {
			return filepath.Dir(strings.TrimSuffix(p, "/"))
		}
		// 文件路径返回其所在目录
		return filepath.Dir(p) + "/"
	}

	// 查找多路径的最长公共前缀
	common := normalized[0]
	for _, p := range normalized[1:] {
		common = commonPathPrefix(common, p)
		if common == "" {
			break
		}
	}

	// 确保公共前缀是目录（以分隔符结尾）
	if !strings.HasSuffix(common, "/") {
		common = filepath.Dir(common) + "/"
	}

	return common
}

// 辅助函数：检查路径是否为目录
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// 辅助函数：计算两个路径的公共前缀
func commonPathPrefix(a, b string) string {
	aComponents := strings.Split(a, "/")
	bComponents := strings.Split(b, "/")

	var common []string
	minLen := min(len(aComponents), len(bComponents))
	for i := 0; i < minLen; i++ {
		if aComponents[i] != bComponents[i] {
			break
		}
		common = append(common, aComponents[i])
	}

	return strings.Join(common, "/") + "/"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateRelativePaths 生成相对路径列表
func GenerateRelativePaths(commonParent string, paths []string) []string {
	relPaths := make([]string, len(paths))
	for i, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			relPaths[i] = ""
			continue
		}

		// 规范化路径
		p = filepath.Clean(p)

		// 计算相对路径
		rel, err := filepath.Rel(commonParent, p)
		if err != nil {
			relPaths[i] = p // 失败时使用原路径
		} else {
			// 确保目录结尾正确
			if isDirectory(p) && !strings.HasSuffix(rel, "/") {
				rel += "/"
			}
			relPaths[i] = strings.TrimPrefix(rel, "/")
		}
	}
	return relPaths
}
