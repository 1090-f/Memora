package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service"
	previewservice "github.com/1090-f/Memora/internal/service/preview"
	"github.com/1090-f/Memora/internal/service/rag/asset"
	"github.com/1090-f/Memora/internal/service/rag/chunking"
	"github.com/1090-f/Memora/internal/service/rag/loader"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/internal/service/rag/pipeline"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/1090-f/Memora/pkg/objectstore"
)

const defaultChunkConfig = `{"splitter":"structure-aware","chunk_size_tokens":1000,"overlap_tokens":100,"min_tokens":100,"repeat_table_header":true}`

func buildDocumentProcessService(cfg *config.Config, store *objectstore.Client, tasks repository.ImportTaskRepository, docs repository.DocumentRepository, kbs repository.KnowledgeBaseRepository, chunks repository.DocumentChunkRepository, vectors repository.VectorRepository, embeddings service.DocumentEmbeddingResolver, previewScheduler previewservice.Scheduler) (service.DocumentProcessService, error) {
	parseOptions := documentParseOptions(cfg)
	chunkOptions := chunking.ChunkOptions{MaxTokens: cfg.Chunking.MaxTokens, MinTokens: cfg.Chunking.MinTokens, OverlapTokens: cfg.Chunking.OverlapTokens, RepeatTableHead: cfg.Chunking.RepeatTableHead, StrategyVersion: cfg.Chunking.StrategyVersion}
	pipelineConfig := pipeline.DocumentPipelineConfig{
		Store: &parserObjectStore{inner: store}, Chunks: chunks, Vectors: vectors,
		ChunkConfig: defaultChunkConfig, ChunkOptions: chunkOptions, Tokenizer: chunking.NewHeuristicTokenizer(),
		ParseOptions: parseOptions, ParserConfig: parser.PythonParserConfig{BaseURL: cfg.DocumentParser.BaseURL, Timeout: cfg.DocumentParser.Timeout, MaxResponseBytes: cfg.DocumentParser.MaxResponseBytes},
		ValidateLimits: parser.DefaultValidateLimits(), AssetEnricher: asset.NewNoopEnricher(),
		ChunkStrategy: cfg.Chunking.Strategy, UseCanonicalChunker: cfg.Chunking.UseCanonicalChunker,
		EnableCanonicalChunkDiff: cfg.Chunking.EnableCanonicalChunkDiff,
		WebLoader:                loader.NewSafeWebLoader(loader.SafeWebConfig{Timeout: cfg.URLImport.Timeout, MaxBytes: cfg.URLImport.MaxResponseBytes, MaxRedirects: cfg.URLImport.MaxRedirects}),
	}
	if cfg.AssetEnrichment.Mode != "" && cfg.AssetEnrichment.Mode != "none" {
		return nil, fmt.Errorf("不支持的 asset_enrichment.mode %q", cfg.AssetEnrichment.Mode)
	}
	documentPipeline, err := pipeline.NewDocumentPipeline(pipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("构造文档加工流水线失败: %w", err)
	}
	return service.NewDocumentProcessService(tasks, docs, kbs, chunks, vectors, documentPipeline, embeddings, store, previewScheduler), nil
}

func documentParseOptions(cfg *config.Config) parser.ParseOptions {
	return parser.ParseOptions{SchemaVersion: parser.SchemaVersion, OCRLanguages: cfg.DocumentParser.OCRLanguages, DoOCR: cfg.DocumentParser.DoOCR, DoImageOCR: cfg.DocumentParser.DoImageOCR, TableStructure: cfg.DocumentParser.TableStructure, ExtractPictures: cfg.DocumentParser.ExtractPictures, IncludeBBoxes: cfg.DocumentParser.IncludeBBoxes}
}

type parserObjectStore struct{ inner *objectstore.Client }

func (p *parserObjectStore) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	return p.inner.OpenObject(ctx, key)
}
func (p *parserObjectStore) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return p.inner.PutObject(ctx, key, reader, size, contentType)
}
func (p *parserObjectStore) StatObject(ctx context.Context, key string) (*parser.ObjectInfo, error) {
	info, err := p.inner.StatObject(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrObjectNotFound) {
			return nil, parser.ErrObjectNotFound
		}
		return nil, err
	}
	return &parser.ObjectInfo{Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}
func (p *parserObjectStore) RemoveObject(ctx context.Context, key string) error {
	return p.inner.RemoveObject(ctx, key)
}
func (p *parserObjectStore) Bucket() string { return p.inner.Bucket() }
