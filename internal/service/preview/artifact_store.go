package preview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/pkg/objectstore"
	"github.com/klauspost/compress/zstd"
)

const (
	artifactSchemaVersion = "1.0"
	workbookSchemaVersion = "1.0"
	maxManifestBytes      = 1 << 20
)

var (
	ErrArtifactMissing = errors.New("Preview Artifact 不存在")
	ErrArtifactCorrupt = errors.New("Preview Artifact 损坏")
)

type ObjectStore interface {
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	StatObject(ctx context.Context, objectKey string) (*objectstore.ObjectInfo, error)
	RemoveObject(ctx context.Context, objectKey string) error
}

type ArtifactStore struct {
	store            ObjectStore
	maxWorkbookBytes int64
}

func NewArtifactStore(store ObjectStore, maxWorkbookBytes int64) *ArtifactStore {
	if maxWorkbookBytes <= 0 {
		maxWorkbookBytes = 64 << 20
	}
	return &ArtifactStore{store: store, maxWorkbookBytes: maxWorkbookBytes}
}

func artifactPrefix(preview *entity.DocumentPreview) string {
	return path.Join("derived", preview.UserID, preview.DocumentID,
		fmt.Sprintf("content-%d", preview.ContentVersion), "preview-"+preview.RenderHash) + "/"
}

func artifactObjectName(typ Type) string {
	if typ == TypeTable {
		return "sheet-data.json.zst"
	}
	return "rendered.pdf"
}

func (s *ArtifactStore) Save(ctx context.Context, preview *entity.DocumentPreview, sourceSHA, strategyVersion string, result *RenderResult) (*ArtifactManifest, error) {
	if preview == nil || result == nil || result.Reader == nil {
		return nil, fmt.Errorf("预览产物参数不完整")
	}
	defer func() { _ = result.Reader.Close() }()
	prefix := artifactPrefix(preview)
	objectKey := path.Join(prefix, artifactObjectName(Type(preview.PreviewType)))
	hash := sha256.New()
	if err := s.store.PutObject(ctx, objectKey, io.TeeReader(result.Reader, hash), result.Size, result.MediaType); err != nil {
		return nil, fmt.Errorf("保存预览对象失败: %w", err)
	}
	manifest := &ArtifactManifest{
		ArtifactSchemaVersion: artifactSchemaVersion,
		DocumentID:            preview.DocumentID, ContentVersion: preview.ContentVersion,
		SourceSHA256: sourceSHA, PreviewType: Type(preview.PreviewType), RenderHash: preview.RenderHash,
		Renderer: preview.Renderer, RendererVersion: preview.RendererVersion, StrategyVersion: strategyVersion,
		Object:    ArtifactObject{Key: objectKey, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: result.Size, MediaType: result.MediaType},
		CreatedAt: time.Now().UTC(),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		_ = s.store.RemoveObject(ctx, objectKey)
		return nil, err
	}
	manifestKey := path.Join(prefix, "manifest.json")
	if err := s.store.PutObject(ctx, manifestKey, bytes.NewReader(manifestBytes), int64(len(manifestBytes)), "application/json"); err != nil {
		_ = s.store.RemoveObject(ctx, objectKey)
		return nil, fmt.Errorf("保存预览 manifest 失败: %w", err)
	}
	return manifest, nil
}

func (s *ArtifactStore) LoadManifest(ctx context.Context, preview *entity.DocumentPreview) (*ArtifactManifest, error) {
	if preview == nil || preview.ManifestKey == nil || strings.TrimSpace(*preview.ManifestKey) == "" {
		return nil, ErrArtifactMissing
	}
	reader, err := s.store.OpenObject(ctx, *preview.ManifestKey)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, ErrArtifactMissing
		}
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	body, err := readLimited(reader, maxManifestBytes)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, ErrArtifactCorrupt
	}
	if manifest.ArtifactSchemaVersion != artifactSchemaVersion || manifest.DocumentID != preview.DocumentID ||
		manifest.ContentVersion != preview.ContentVersion || manifest.RenderHash != preview.RenderHash ||
		manifest.PreviewType != Type(preview.PreviewType) || manifest.Object.Key == "" {
		return nil, ErrArtifactCorrupt
	}
	info, err := s.store.StatObject(ctx, manifest.Object.Key)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, ErrArtifactMissing
		}
		return nil, err
	}
	if info.Size != manifest.Object.Size || manifest.Object.Size < 0 {
		return nil, ErrArtifactCorrupt
	}
	return &manifest, nil
}

func (s *ArtifactStore) Open(ctx context.Context, preview *entity.DocumentPreview) (*File, error) {
	manifest, err := s.LoadManifest(ctx, preview)
	if err != nil {
		return nil, err
	}
	if manifest.PreviewType == TypePDF {
		validationReader, err := s.store.OpenObject(ctx, manifest.Object.Key)
		if err != nil {
			return nil, err
		}
		if err := validateArtifactStream(validationReader, manifest.Object, true); err != nil {
			_ = validationReader.Close()
			return nil, ErrArtifactCorrupt
		}
		_ = validationReader.Close()
	}
	reader, err := s.store.OpenObject(ctx, manifest.Object.Key)
	if err != nil {
		return nil, err
	}
	return &File{Reader: reader, FileName: artifactObjectName(manifest.PreviewType), ContentType: manifest.Object.MediaType, Size: manifest.Object.Size}, nil
}

func (s *ArtifactStore) LoadWorkbook(ctx context.Context, preview *entity.DocumentPreview) (*Workbook, error) {
	manifest, err := s.LoadManifest(ctx, preview)
	if err != nil {
		return nil, err
	}
	if manifest.PreviewType != TypeTable {
		return nil, ErrArtifactCorrupt
	}
	reader, err := s.store.OpenObject(ctx, manifest.Object.Key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	compressed, err := readLimited(reader, s.maxWorkbookBytes)
	if err != nil {
		return nil, ErrArtifactCorrupt
	}
	if int64(len(compressed)) != manifest.Object.Size || !strings.EqualFold(hashBytes(compressed), manifest.Object.SHA256) {
		return nil, ErrArtifactCorrupt
	}
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(uint64(s.maxWorkbookBytes)))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	body, err := decoder.DecodeAll(compressed, nil)
	if err != nil || int64(len(body)) > s.maxWorkbookBytes {
		return nil, ErrArtifactCorrupt
	}
	var workbook Workbook
	if err := json.Unmarshal(body, &workbook); err != nil || workbook.SchemaVersion != workbookSchemaVersion {
		return nil, ErrArtifactCorrupt
	}
	return &workbook, nil
}

func validateArtifactStream(reader io.Reader, object ArtifactObject, requirePDF bool) error {
	hash := sha256.New()
	buffer := make([]byte, 32<<10)
	var total int64
	var head, tail []byte
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			_, _ = hash.Write(chunk)
			total += int64(n)
			if len(head) < 5 {
				needed := 5 - len(head)
				if needed > len(chunk) {
					needed = len(chunk)
				}
				head = append(head, chunk[:needed]...)
			}
			tail = append(tail, chunk...)
			if len(tail) > 2048 {
				tail = append([]byte(nil), tail[len(tail)-2048:]...)
			}
			if total > object.Size {
				return ErrArtifactCorrupt
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if total != object.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), object.SHA256) {
		return ErrArtifactCorrupt
	}
	if requirePDF && (!bytes.Equal(head, []byte("%PDF-")) || !bytes.Contains(tail, []byte("%%EOF"))) {
		return ErrArtifactCorrupt
	}
	return nil
}

func hashBytes(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func EncodeWorkbook(workbook *Workbook) ([]byte, error) {
	if workbook == nil {
		return nil, fmt.Errorf("workbook 不能为空")
	}
	workbook.SchemaVersion = workbookSchemaVersion
	body, err := json.Marshal(workbook)
	if err != nil {
		return nil, err
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	defer encoder.Close()
	return encoder.EncodeAll(body, nil), nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("对象超过读取上限 %d", limit)
	}
	return body, nil
}
