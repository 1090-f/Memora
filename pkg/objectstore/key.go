package objectstore

import (
	"path"
	"strings"
	"time"
)

// ObjectKeyPrefix 定义文档对象的统一前缀。
const ObjectKeyPrefix = "documents"

// BuildObjectKey 生成统一的 MinIO 对象 key：
// documents/{YYYYMMDD}/{user_id}/{kb_id}/{task_id}/{safe_file_name}。
// 日期前缀（UTC）便于在 MinIO 控制台按天浏览导入记录；
// 文件名会被净化，防止路径穿越与任意 key 注入。
func BuildObjectKey(userID, kbID, taskID, fileName string) string {
	date := time.Now().UTC().Format("20060102")
	return path.Join(ObjectKeyPrefix, date, userID, kbID, taskID, sanitizeFileName(fileName))
}

// sanitizeFileName 净化文件名：仅保留字母数字、中文与常见文件字符，替换路径分隔符与危险字符。
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_', r == ' ', r >= 0x4E00 && r <= 0x9FFF:
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	safe := builder.String()
	safe = strings.TrimSpace(safe)
	if safe == "" || safe == "." || safe == ".." {
		return "file"
	}
	return safe
}
