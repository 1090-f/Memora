// Package testutil 提供 RAG 集成测试的共享工具：临时数据库、迁移应用与种子数据。
// 集成测试通过环境变量 MEMORA_TEST_DATABASE_URL 启用（指向可创建数据库的 PostgreSQL 管理连接），
// 未设置时相关测试自动跳过，不影响常规 go test ./...。
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// EnvTestDatabaseURL 是集成测试使用的 PostgreSQL 管理连接串环境变量。
const EnvTestDatabaseURL = "MEMORA_TEST_DATABASE_URL"

// OpenRAGTestDB 创建独立临时数据库、按序应用 scripts/migrations 全部 up 迁移并返回 GORM 连接。
// 未设置 EnvTestDatabaseURL 时跳过测试；测试结束后强制删除临时数据库。
func OpenRAGTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv(EnvTestDatabaseURL))
	if adminURL == "" {
		t.Skipf("未设置 %s，跳过真实 PostgreSQL/ParadeDB 集成测试", EnvTestDatabaseURL)
	}
	ctx := context.Background()

	admin, err := pgconn.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("连接管理数据库失败: %v", err)
	}
	dbName := "memora_test_" + randomHex(6)
	if _, err := execAll(ctx, admin, fmt.Sprintf(`CREATE DATABASE %q`, dbName)); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("创建临时数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if _, err := execAll(context.Background(), admin, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, dbName)); err != nil {
			t.Logf("清理临时数据库 %s 失败: %v", dbName, err)
		}
		_ = admin.Close(context.Background())
	})

	dsn, err := swapDatabase(adminURL, dbName)
	if err != nil {
		t.Fatalf("构造临时数据库 DSN 失败: %v", err)
	}
	conn, err := pgconn.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("连接临时数据库失败: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	migrations, err := migrationFiles()
	if err != nil {
		t.Fatalf("定位迁移文件失败: %v", err)
	}
	for _, file := range migrations {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取迁移文件失败: %v", err)
		}
		if _, err := execAll(ctx, conn, string(sqlBytes)); err != nil {
			t.Fatalf("应用迁移 %s 失败: %v", filepath.Base(file), err)
		}
	}

	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开 GORM 连接失败: %v", err)
	}
	return db
}

// execAll 使用简单查询协议执行整段 SQL（支持多语句迁移文件）。
func execAll(ctx context.Context, conn *pgconn.PgConn, sqlText string) ([]*pgconn.Result, error) {
	reader := conn.Exec(ctx, sqlText)
	return reader.ReadAll()
}

// swapDatabase 把 DSN 的库名替换为目标库名。
func swapDatabase(dsn, dbName string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

// migrationFiles 从当前目录向上查找 scripts/migrations 目录并返回排序后的 up 迁移文件。
func migrationFiles() ([]string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for range 10 {
		candidate := filepath.Join(dir, "scripts", "migrations")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return filepath.Glob(filepath.Join(candidate, "*.up.sql"))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("找不到 scripts/migrations 目录")
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func mustExec(t *testing.T, db *gorm.DB, sqlText string, args ...any) {
	t.Helper()
	if err := db.Exec(sqlText, args...).Error; err != nil {
		t.Fatalf("执行种子 SQL 失败: %v\nSQL: %s", err, sqlText)
	}
}

// SeedUser 插入最小用户行。
func SeedUser(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO users (id, username, email, password_hash, nickname) VALUES (?, ?, ?, 'hash', '测试用户')`,
		id, "user_"+id, "user_"+id+"@example.com")
}

// SeedModelConfig 插入指定类型的模型配置；vectorDimension<=0 时写入 NULL。
func SeedModelConfig(t *testing.T, db *gorm.DB, id, userID, modelType string, vectorDimension int) {
	t.Helper()
	var dimension any
	if vectorDimension > 0 {
		dimension = vectorDimension
	}
	mustExec(t, db, `INSERT INTO ai_model_configs (id, user_id, model_type, provider, name, base_url, vector_dimension, is_default)
		VALUES (?, ?, ?, 'openai', ?, 'https://example.com/v1', ?, false)`,
		id, userID, modelType, modelType+"_"+id, dimension)
}

// SeedKnowledgeBase 插入最小知识库行。
func SeedKnowledgeBase(t *testing.T, db *gorm.DB, id, userID, name string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO knowledge_bases (id, user_id, name) VALUES (?, ?, ?)`, id, userID, name)
}

// SeedSearchConfig 插入检索配置行（其余字段使用迁移默认值）。
func SeedSearchConfig(t *testing.T, db *gorm.DB, id, kbID string, minVectorScore float64) {
	t.Helper()
	mustExec(t, db, `INSERT INTO search_configs (id, knowledge_base_id, min_vector_score) VALUES (?, ?, ?)`, id, kbID, minVectorScore)
}

// SeedDocument 插入文档行；activeVersion<=0 时 active_index_version 为 NULL。
func SeedDocument(t *testing.T, db *gorm.DB, id, userID, kbID, title, status string, activeVersion int, embeddingModelID *string) {
	t.Helper()
	var active any
	if activeVersion > 0 {
		active = activeVersion
	}
	mustExec(t, db, `INSERT INTO documents (id, user_id, knowledge_base_id, title, source_type, processing_status, active_index_version, embedding_model_id)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?)`,
		id, userID, kbID, title, status, active, embeddingModelID)
}

// SeedChunk 插入文档分块行（chunk_config_hash 固定为测试值，与分段配置无关）。
func SeedChunk(t *testing.T, db *gorm.DB, id, userID, kbID, docID string, chunkNo int, content string, indexVersion int) {
	t.Helper()
	mustExec(t, db, `INSERT INTO document_chunks (id, user_id, knowledge_base_id, document_id, chunk_no, content, char_count, token_count, content_version, chunk_version, index_version, chunk_config_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, 'test-chunk-config')`,
		id, userID, kbID, docID, chunkNo, content, len([]rune(content)), len([]rune(content))/2+1, indexVersion)
}

// SeedVector 插入文档向量行（状态默认为 ready）。
func SeedVector(t *testing.T, db *gorm.DB, id, userID, kbID, docID, chunkID string, indexVersion int, modelID string, vec []float32) {
	t.Helper()
	mustExec(t, db, `INSERT INTO document_vectors (id, user_id, knowledge_base_id, document_id, chunk_id, index_version, embedding_model_id, embedding_dim, embedding, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS vector), 'ready')`,
		id, userID, kbID, docID, chunkID, indexVersion, modelID, len(vec), vectorLiteral(vec))
}

// SoftDeleteDocument 软删除指定文档（置 deleted_at）。
func SoftDeleteDocument(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	mustExec(t, db, `UPDATE documents SET deleted_at = now() WHERE id = ?`, id)
}

// vectorLiteral 把向量格式化为 pgvector 字面量字符串，如 [1,0,0]。
func vectorLiteral(vec []float32) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// NewID 返回一个新的 UUID 字符串，便于测试生成主键。
func NewID() string { return uuid.NewString() }
