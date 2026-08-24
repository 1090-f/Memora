package parser

import (
	"bytes"
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
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// 资产上传并发与重试策略。
const (
	// maxConcurrentAssetUploads 是 Artifact 资产并发上传上限。
	maxConcurrentAssetUploads = 4
	// assetUploadMaxAttempts 是单资产上传最大尝试次数（含首次）。
	assetUploadMaxAttempts = 3
	// assetUploadRetryDelay 是单资产上传重试的基础退避间隔（线性递增）。
	assetUploadRetryDelay = 1 * time.Second
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
//  1. 预清洗资产（超限/类型不支持的降级为 Omitted + warning）；
//  2. 解码并保存 assets（data_base64 → object_key），并发上传 + 单资产重试；
//  3. 保存压缩后的 parsed-document.json.zst；
//  4. 最后保存 manifest.json。
//
// 保存成功后 doc.Assets 中的 data_base64 已被替换为 object_key，
// 同一份内存对象可继续用于后续分块，无需重新读取。
func (s *ArtifactStore) Save(ctx context.Context, prefix string, doc *ParsedDocument, parseConfigHash string) (*ArtifactRef, error) {
	// 0. 预清洗：单图超限、总量超预算或 MIME 类型不支持的资产降级为 Omitted + warning，
	//    不阻断整篇文档导入；解码前按 base64 长度预检，超大图不进入内存解码。
	s.sanitizeAssets(doc)
	if err := ValidateParsedDocument(doc, "", s.limits); err != nil {
		return nil, fmt.Errorf("保存 Artifact 前校验失败: %w", err)
	}

	// 1. 资产：解码 → 上传 → 替换引用（sanitize 与校验通过后每张图均合法且不超限）。
	//    并发上传（限流 semaphore）保持 manifest 顺序确定；单资产失败重试，最终失败
	//    时整体 Save 失败（与顺序实现语义一致：manifest 未写入，Artifact 不生效）。
	assetInfos := make([]AssetInfo, len(doc.Assets))
	results := make([]error, len(doc.Assets))
	sem := make(chan struct{}, maxConcurrentAssetUploads)
	var wg sync.WaitGroup
	for i := range doc.Assets {
		asset := &doc.Assets[i]
		if asset.Omitted || asset.DataBase64 == "" {
			if asset.Omitted {
				assetInfos[i] = AssetInfo{ID: asset.ID}
			}
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, asset *Asset) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := base64.StdEncoding.DecodeString(asset.DataBase64)
			if err != nil {
				results[idx] = fmt.Errorf("Asset %q base64 解码失败: %w", asset.ID, err)
				return
			}
			digest := sha256.Sum256(data)
			assetSHA := hex.EncodeToString(digest[:])
			objectKey := path.Join(prefix, ArtifactAssetsDir, asset.ID+extensionForMIME(asset.MIMEType))
			if err := putAssetWithRetry(ctx, s.store, objectKey, data, asset.MIMEType); err != nil {
				results[idx] = fmt.Errorf("保存 Asset %q 失败: %w", asset.ID, err)
				return
			}
			asset.SHA256 = assetSHA
			asset.ObjectKey = objectKey
			asset.DataBase64 = ""
			assetInfos[idx] = AssetInfo{ID: asset.ID, ObjectKey: objectKey, SHA256: assetSHA, Size: int64(len(data))}
		}(i, asset)
	}
	wg.Wait()
	for _, resultErr := range results {
		if resultErr != nil {
			return nil, resultErr
		}
	}

	// 2. 文档：序列化 → zstd 压缩 → 上传。
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("序列化 ParsedDocument 失败: %w", err)
	}
	docSHA := sha256.Sum256(docBytes)
	compressed := zstdCompress(docBytes)
	if err := s.store.PutObject(ctx, path.Join(prefix, ArtifactDocumentFile), bytes.NewReader(compressed), int64(len(compressed)), "application/octet-stream"); err != nil {
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
	if err := s.store.PutObject(ctx, path.Join(prefix, ArtifactManifestFile), bytes.NewReader(manifestBytes), int64(len(manifestBytes)), "application/json"); err != nil {
		return nil, fmt.Errorf("保存 manifest 失败: %w", err)
	}
	return &ArtifactRef{Prefix: prefix, Manifest: manifest}, nil
}

// sanitizeAssets 预清洗资产：超过单图大小限制、总量预算或 MIME 类型不受支持的资产
// 标记为 Omitted 并追加 warning，避免个别图片问题阻断整篇文档导入。
// 解码前先用 DecodedLen 预检长度，超大 base64 不进入内存解码。
func (s *ArtifactStore) sanitizeAssets(doc *ParsedDocument) {
	omit := func(asset *Asset, reason string) {
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("Asset %q %s，已跳过", asset.ID, reason))
		asset.Omitted = true
		asset.DataBase64 = ""
	}
	for i := range doc.Assets {
		asset := &doc.Assets[i]
		if asset.Omitted || asset.DataBase64 == "" {
			continue
		}
		decodedLen := base64.StdEncoding.DecodedLen(len(asset.DataBase64))
		if s.limits.MaxAssetBytes > 0 && int64(decodedLen) > s.limits.MaxAssetBytes {
			omit(asset, fmt.Sprintf("大小 %d 字节超过单图限制 %d", decodedLen, s.limits.MaxAssetBytes))
			continue
		}
		if !isAllowedAssetMIME(asset.MIMEType) {
			omit(asset, fmt.Sprintf("类型 %q 不受支持", asset.MIMEType))
		}
	}

	// 总量预算：超出的资产降级为 Omitted，优先丢弃未被任何 Block 引用的资产，
	// 仍超预算时按列表顺序丢弃，保证总量校验可通过。
	if s.limits.MaxTotalAssetBytes > 0 {
		referenced := make(map[string]struct{})
		for _, block := range doc.Blocks {
			for _, ref := range block.AssetRefs {
				referenced[ref] = struct{}{}
			}
		}
		var total int64
		var over []*Asset
		for i := range doc.Assets {
			asset := &doc.Assets[i]
			if asset.Omitted || asset.DataBase64 == "" {
				continue
			}
			l := int64(base64.StdEncoding.DecodedLen(len(asset.DataBase64)))
			if total+l > s.limits.MaxTotalAssetBytes {
				over = append(over, asset)
				continue
			}
			total += l
		}
		for _, asset := range over {
			if _, used := referenced[asset.ID]; used {
				continue
			}
			omit(asset, "超出资产总量预算")
		}
		for _, asset := range over {
			if asset.Omitted {
				continue
			}
			omit(asset, "超出资产总量预算")
		}
	}

	// 截断超限 warning，保证 Artifact 重新加载时仍能通过 MaxWarnings 校验。
	if s.limits.MaxWarnings > 0 && len(doc.Warnings) > s.limits.MaxWarnings {
		doc.Warnings = doc.Warnings[:s.limits.MaxWarnings]
	}
}

// isAllowedAssetMIME 判断资产 MIME 是否允许入库：
// 仅接受位图类图片（image/*），SVG 可内嵌脚本（stored XSS 风险）与非图片类型一律拒绝。
func isAllowedAssetMIME(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	return strings.HasPrefix(m, "image/") && m != "image/svg+xml"
}

// putAssetWithRetry 上传单个资产，失败时线性退避重试；上下文取消时立即返回。
func putAssetWithRetry(ctx context.Context, store ObjectStore, objectKey string, data []byte, mimeType string) error {
	var lastErr error
	for attempt := 1; attempt <= assetUploadMaxAttempts; attempt++ {
		if err := store.PutObject(ctx, objectKey, bytes.NewReader(data), int64(len(data)), mimeType); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt >= assetUploadMaxAttempts || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(assetUploadRetryDelay * time.Duration(attempt)):
		}
	}
	return lastErr
}

// Resolve 查找兼容 Artifact：
// manifest 存在且版本/源哈希/解析器一致才算命中；否则返回 ErrArtifactNotFound/ErrArtifactCorrupt。
func (s *ArtifactStore) Resolve(ctx context.Context, prefix, expectedSourceSHA256 string) (*ArtifactRef, error) {
	return s.ResolveWithIdentity(ctx, prefix, expectedSourceSHA256, ParseRuntimeIdentity{})
}

// ResolveWithIdentity 除完整性外校验当前 Parser/Adapter 兼容身份。
// 版本不匹配表示缓存已过期，返回 ErrArtifactNotFound 触发重新解析，而不是将旧产物视为损坏。
func (s *ArtifactStore) ResolveWithIdentity(ctx context.Context, prefix, expectedSourceSHA256 string, identity ParseRuntimeIdentity) (*ArtifactRef, error) {
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
	if len(identity.ParserVersions) > 0 {
		expectedVersion, ok := identity.ParserVersions[manifest.ParserName]
		if !ok || expectedVersion == "" || manifest.ParserVersion != expectedVersion ||
			manifest.AdapterVersion != identity.AdapterVersion {
			return nil, ErrArtifactNotFound
		}
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
