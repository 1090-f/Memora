package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/1090-f/Memora/internal/apperror"
	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
)

const (
	defaultDocumentReadTokens = 2000
	maxDocumentReaderTokens   = 6000
	maxDocumentReadChunks     = 100
)

type documentCursor struct {
	UserID          string `json:"u"`
	KnowledgeBaseID string `json:"k"`
	DocumentID      string `json:"d"`
	Section         string `json:"s,omitempty"`
	IndexVersion    int    `json:"v"`
	ChunkNo         int    `json:"c"`
	RuneOffset      int    `json:"o"`
}

// documentReader 实现跨模块 contracts.DocumentService。
type documentReader struct {
	chunks    repository.DocumentChunkRepository
	citations CitationService
	counter   *tokenCounter
	cursorKey []byte
}

// NewDocumentReader 创建带 HMAC 签名游标的受限正文读取服务。
func NewDocumentReader(chunks repository.DocumentChunkRepository, citations CitationService, cursorSecret string) (contracts.DocumentService, error) {
	if chunks == nil || citations == nil {
		return nil, fmt.Errorf("文档读取服务缺少 ChunkRepository 或 CitationService")
	}
	if cursorSecret == "" {
		return nil, fmt.Errorf("文档读取游标密钥不能为空")
	}
	key := sha256.Sum256([]byte(cursorSecret))
	return &documentReader{chunks: chunks, citations: citations, counter: NewTokenCounter(), cursorKey: key[:]}, nil
}

func (s *documentReader) Read(ctx context.Context, request contracts.DocumentReadRequest) (contracts.DocumentReadResult, error) {
	userID, kbID, docID := string(request.UserID), string(request.KnowledgeBaseID), string(request.DocumentID)
	section := strings.TrimSpace(request.Section)
	if userID == "" || kbID == "" || docID == "" || len(section) > 500 {
		return contracts.DocumentReadResult{}, apperrors.ErrInvalidArgument
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultDocumentReadTokens
	}
	if maxTokens > maxDocumentReaderTokens {
		return contracts.DocumentReadResult{}, apperrors.ErrInvalidArgument
	}

	fromChunk, runeOffset, cursorVersion := 0, 0, 0
	if request.Cursor != "" {
		cursor, err := s.decodeCursor(request.Cursor)
		if err != nil || cursor.UserID != userID || cursor.KnowledgeBaseID != kbID || cursor.DocumentID != docID || cursor.Section != section {
			return contracts.DocumentReadResult{}, apperrors.New(contracts.ErrInvalidArgument, fmt.Errorf("无效或不匹配的文档游标"))
		}
		fromChunk, runeOffset, cursorVersion = cursor.ChunkNo, cursor.RuneOffset, cursor.IndexVersion
	}

	chunks, err := s.chunks.ReadActive(ctx, userID, kbID, docID, section, fromChunk, maxDocumentReadChunks)
	if errors.Is(err, repository.ErrDocumentNotFound) {
		return contracts.DocumentReadResult{}, apperrors.ErrNotFound
	}
	if err != nil {
		return contracts.DocumentReadResult{}, apperrors.New(contracts.ErrInternal, err)
	}
	if len(chunks) == 0 {
		return contracts.DocumentReadResult{DocumentID: request.DocumentID}, nil
	}
	if cursorVersion != 0 && cursorVersion != chunks[0].IndexVersion {
		return contracts.DocumentReadResult{}, apperrors.New(contracts.ErrIndexVersionConflict, fmt.Errorf("文档活动索引已更新"))
	}

	var content strings.Builder
	remaining := maxTokens
	next := documentCursor{UserID: userID, KnowledgeBaseID: kbID, DocumentID: docID, Section: section, IndexVersion: chunks[0].IndexVersion}
	truncated := false
	firstChunk := chunks[0]
	for _, chunk := range chunks {
		if remaining <= 0 {
			next.ChunkNo, next.RuneOffset = chunk.ChunkNo, 0
			truncated = true
			break
		}
		runes := []rune(chunk.Content)
		start := 0
		if chunk.ChunkNo == fromChunk {
			start = runeOffset
		}
		if start < 0 || start > len(runes) {
			return contracts.DocumentReadResult{}, apperrors.ErrInvalidArgument
		}
		part := string(runes[start:])
		count := s.counter.Count(part)
		if count <= remaining {
			if content.Len() > 0 {
				content.WriteString("\n\n")
			}
			content.WriteString(part)
			remaining -= count
			next.ChunkNo, next.RuneOffset = chunk.ChunkNo+1, 0
			continue
		}
		end := start + maxRunePrefix(runes[start:], remaining, s.counter)
		if end == start {
			truncated = true
			break
		}
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(string(runes[start:end]))
		next.ChunkNo, next.RuneOffset = chunk.ChunkNo, end
		truncated = true
		break
	}
	if !truncated && len(chunks) == maxDocumentReadChunks {
		truncated = true
	}

	nextCursor := ""
	if truncated {
		nextCursor, err = s.encodeCursor(next)
		if err != nil {
			return contracts.DocumentReadResult{}, apperrors.New(contracts.ErrInternal, err)
		}
	}
	location := make(map[string]any)
	_ = json.Unmarshal(firstChunk.SourceLocation, &location)
	return contracts.DocumentReadResult{
		DocumentID: request.DocumentID, Title: firstChunk.DocumentTitle, Content: content.String(),
		NextCursor: nextCursor, Truncated: truncated,
		Citation: s.citations.BuildKnowledge(kbID, docID, firstChunk.DocumentTitle, firstChunk.ChunkID,
			content.String(), location, firstChunk.DocumentUpdatedAt),
	}, nil
}

func maxRunePrefix(runes []rune, maxTokens int, counter *tokenCounter) int {
	if maxTokens <= 0 || len(runes) == 0 {
		return 0
	}
	low, high := 1, len(runes)
	best := 0
	for low <= high {
		mid := low + (high-low)/2
		if counter.Count(string(runes[:mid])) <= maxTokens {
			best, low = mid, mid+1
		} else {
			high = mid - 1
		}
	}
	return best
}

func (s *documentReader) encodeCursor(cursor documentCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *documentReader) decodeCursor(value string) (documentCursor, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return documentCursor{}, fmt.Errorf("游标格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return documentCursor{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return documentCursor{}, err
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return documentCursor{}, fmt.Errorf("游标签名无效")
	}
	var cursor documentCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return documentCursor{}, err
	}
	if cursor.IndexVersion <= 0 || cursor.ChunkNo < 0 || cursor.RuneOffset < 0 {
		return documentCursor{}, fmt.Errorf("游标值无效")
	}
	return cursor, nil
}
