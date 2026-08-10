package parser

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// 确定性 Artifact key 前缀：
//
//	derived/{user_id}/{document_id}/content-{content_version}/parse-{parse_config_hash}/
//
// 前缀可由 document/content version 与 parse config 确定性计算，
// 不依赖扫描 bucket 发现 Artifact。
func ArtifactKeyPrefix(userID, documentID string, contentVersion int, parseConfigHash string) string {
	return path.Join("derived", userID, documentID,
		fmt.Sprintf("content-%d", contentVersion),
		fmt.Sprintf("parse-%s", parseConfigHash)) + "/"
}

// Artifact 文件相对前缀的固定名称。
const (
	ArtifactManifestFile     = "manifest.json"
	ArtifactDocumentFile     = "parsed-document.json.zst"
	ArtifactAssetsDir        = "assets"
	ArtifactManifestFileSize = 1 << 20 // 1MB 读取上限
)

// ErrArtifactNotFound 表示不存在兼容的 Parsed Artifact。
var ErrArtifactNotFound = errors.New("Parsed Artifact 不存在")

// ErrArtifactCorrupt 表示 Artifact 存在但不完整/损坏，必须重新解析。
var ErrArtifactCorrupt = errors.New("Parsed Artifact 损坏")

// ArtifactRef 指向一个已确认完整的 Parsed Artifact。
type ArtifactRef struct {
	Prefix   string
	Manifest ArtifactManifest
}

// ArtifactStore 负责 Parsed Artifact 的保存、查找与复用。
// 保存顺序：assets → parsed-document → manifest（manifest 最后写入，
// 只有 manifest 存在且全部哈希一致，Artifact 才算完整）。
type ArtifactStore struct {
	store  ObjectStore
	limits ValidateLimits
}

// NewArtifactStore 构造 Artifact 存储。
func NewArtifactStore(store ObjectStore, limits ValidateLimits) *ArtifactStore {
	return &ArtifactStore{store: store, limits: limits}
}

// Save 保存 ParsedDocument 为 Artifact：
//  1. 解码并保存 assets（data_base64 → object_key）；
//  2. 保存压缩后的 parsed-document.json.zst；
//  3. 最后保存 manifest.json。
//
// 保存成功后 doc.Assets 中的 data_base64 已被替换为 object_key，
// 同一份内存对象可继续用于后续分块，无需重新读取。
func (s *ArtifactStore) Save(ctx context.Context, prefix string, doc *ParsedDocument, parseConfigHash string) (*ArtifactRef, error) {
	if err := ValidateParsedDocument(doc, "", s.limits); err != nil {
		return nil, fmt.Errorf("保存 Artifact 前校验失败: %w", err)
	}

	// 1. 资产：解码 → 校验单图限制 → 上传 → 替换引用。
	assetInfos := make([]AssetInfo, 0, len(doc.Assets))
	for i := range doc.Assets {
		asset := &doc.Assets[i]
		if asset.Omitted || asset.DataBase64 == "" {
			if asset.Omitted {
				assetInfos = append(assetInfos, AssetInfo{ID: asset.ID})
			}
			continue
		}
		data, err := base64.StdEncoding.DecodeString(asset.DataBase64)
		if err != nil {
			return nil, fmt.Errorf("Asset %q base64 解码失败: %w", asset.ID, err)
		}
		if s.limits.MaxAssets > 0 && int64(len(data)) > 32*1024*1024 {
			return nil, fmt.Errorf("Asset %q 超过单图大小限制 32MB（实际 %d 字节）", asset.ID, len(data))
		}
		digest := sha256.Sum256(data)
		assetSHA := hex.EncodeToString(digest[:])
		objectKey := path.Join(prefix, ArtifactAssetsDir, asset.ID+extensionForMIME(asset.MIMEType))
		if err := s.store.PutObject(ctx, objectKey, strings.NewReader(string(data)), int64(len(data)), asset.MIMEType); err != nil {
			return nil, fmt.Errorf("保存 Asset %q 失败: %w", asset.ID, err)
		}
		asset.SHA256 = assetSHA
		asset.ObjectKey = objectKey
		asset.DataBase64 = ""
		assetInfos = append(assetInfos, AssetInfo{ID: asset.ID, ObjectKey: objectKey, SHA256: assetSHA, Size: int64(len(data))})
	}

	// 2. 文档：序列化 → zstd 压缩 → 上传。
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("序列化 ParsedDocument 失败: %w", err)
	}
	docSHA := sha256.Sum256(docBytes)
	compressed := zstdCompress(docBytes)
	if err := s.store.PutObject(ctx, path.Join(prefix, ArtifactDocumentFile), strings.NewReader(string(compressed)), int64(len(compressed)), "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("保存 ParsedDocument 失败: %w", err)
	}

	// 3. manifest 最后写入。
	manifest := ArtifactManifest{
		ArtifactSchemaVersion:       ArtifactSchemaVersion,
		ParsedDocumentSchemaVersion: doc.SchemaVersion,
		SourceSHA256:                doc.Source.SHA256,
		ParserName:                  doc.Parser.Name,
		ParserVersion:               doc.Parser.Version,
		AdapterVersion:              doc.Parser.AdapterVersion,
		ParseConfigHash:             parseConfigHash,
		ParsedDocumentSHA256:        hex.EncodeToString(docSHA[:]),
		Assets:                      assetInfos,
		CreatedAt:                   time.Now().UTC().Format(time.RFC3339),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("序列化 manifest 失败: %w", err)
	}
	if err := s.store.PutObject(ctx, path.Join(prefix, ArtifactManifestFile), strings.NewReader(string(manifestBytes)), int64(len(manifestBytes)), "application/json"); err != nil {
		return nil, fmt.Errorf("保存 manifest 失败: %w", err)
	}
	return &ArtifactRef{Prefix: prefix, Manifest: manifest}, nil
}

// Resolve 查找兼容 Artifact：
// manifest 存在且版本/源哈希/解析器一致才算命中；否则返回 ErrArtifactNotFound/ErrArtifactCorrupt。
func (s *ArtifactStore) Resolve(ctx context.Context, prefix, expectedSourceSHA256 string) (*ArtifactRef, error) {
	manifestKey := path.Join(prefix, ArtifactManifestFile)
	if _, err := s.store.StatObject(ctx, manifestKey); err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("查询 Artifact manifest 失败: %w", err)
	}
	body, err := s.readObject(ctx, manifestKey, ArtifactManifestFileSize)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, ErrArtifactCorrupt
	}
	if manifest.ArtifactSchemaVersion != ArtifactSchemaVersion {
		return nil, fmt.Errorf("%w: manifest 版本 %q 不支持", ErrArtifactCorrupt, manifest.ArtifactSchemaVersion)
	}
	if err := CheckSchemaVersion(manifest.ParsedDocumentSchemaVersion); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArtifactCorrupt, err)
	}
	if expectedSourceSHA256 != "" && !strings.EqualFold(manifest.SourceSHA256, expectedSourceSHA256) {
		return nil, fmt.Errorf("%w: 源哈希不一致", ErrArtifactCorrupt)
	}
	return &ArtifactRef{Prefix: prefix, Manifest: manifest}, nil
}

// Load 加载 ParsedDocument 并校验完整性：
// 文档哈希、全部资产对象存在性。任一失败返回 ErrArtifactCorrupt。
func (s *ArtifactStore) Load(ctx context.Context, ref *ArtifactRef) (*ParsedDocument, error) {
	body, err := s.readObject(ctx, path.Join(ref.Prefix, ArtifactDocumentFile), s.limits.MaxTotalAssetBytes+64*1024*1024)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	docBytes, err := zstdDecompress(body)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	digest := sha256.Sum256(docBytes)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), ref.Manifest.ParsedDocumentSHA256) {
		return nil, fmt.Errorf("%w: ParsedDocument 哈希校验失败", ErrArtifactCorrupt)
	}
	var doc ParsedDocument
	if err := json.Unmarshal(docBytes, &doc); err != nil {
		return nil, ErrArtifactCorrupt
	}

	// 资产对象必须全部存在，缺失/损坏拒绝复用。
	byID := make(map[string]int, len(doc.Assets))
	for i, asset := range doc.Assets {
		byID[asset.ID] = i
	}
	for _, info := range ref.Manifest.Assets {
		if info.ObjectKey == "" {
			continue
		}
		stat, err := s.store.StatObject(ctx, info.ObjectKey)
		if err != nil {
			if errors.Is(err, ErrObjectNotFound) {
				return nil, fmt.Errorf("%w: 资产 %q 缺失", ErrArtifactCorrupt, info.ID)
			}
			return nil, fmt.Errorf("%w: 查询资产 %q 失败: %v", ErrArtifactCorrupt, info.ID, err)
		}
		if idx, ok := byID[info.ID]; ok && stat.Size >= 0 {
			doc.Assets[idx].ObjectKey = info.ObjectKey
		}
	}
	if err := ValidateParsedDocument(&doc, ref.Manifest.SourceSHA256, s.limits); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrArtifactCorrupt, err)
	}
	return &doc, nil
}

// readObject 读取完整对象并限制大小。
func (s *ArtifactStore) readObject(ctx context.Context, key string, limit int64) ([]byte, error) {
	reader, err := s.store.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(io.LimitReader(reader, limit+1))
}

// extensionForMIME 返回资产文件扩展名。
func extensionForMIME(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

// zstdCompress 压缩数据（纯内存）。
func zstdCompress(data []byte) []byte {
	encoder, _ := zstd.NewWriter(nil)
	defer encoder.Close()
	return encoder.EncodeAll(data, nil)
}

// zstdDecompress 解压数据（纯内存）。
func zstdDecompress(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(data, nil)
}
