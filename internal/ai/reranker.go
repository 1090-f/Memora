package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/1090-f/Memora/internal/contracts"
)

// goOpenAIReranker 使用 go-openai 实现 Reranker。
type goOpenAIReranker struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// rerankRequest 表示 Reranker API 请求。
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResponse 表示 Reranker API 响应。
type rerankResponse struct {
	Results []rerankResult `json:"results"`
}

// rerankResult 表示单个 rerank 结果。
type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// rerankerItem 表示 reranker 返回的结果项。
type rerankerItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// Rerank 实现 contracts.Reranker.Rerank。
func (r *goOpenAIReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]contracts.RerankItem, error) {
	// 构建请求
	reqBody := rerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      topK,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rerank request: %w", err)
	}

	// 创建 HTTP 请求
	url := strings.TrimRight(r.baseURL, "/") + "/rerank"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create rerank request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	// 发送请求
	client := r.client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send rerank request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read rerank response: %w", err)
	}
	if len(body) > 4*1024*1024 {
		return nil, fmt.Errorf("rerank response exceeds size limit")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank API error: status=%d", resp.StatusCode)
	}

	// 解析响应
	var rerankResp rerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, fmt.Errorf("unmarshal rerank response: %w", err)
	}

	// 转换结果
	results := make([]contracts.RerankItem, len(rerankResp.Results))
	for i, item := range rerankResp.Results {
		results[i] = contracts.RerankItem{
			Index: item.Index,
			Score: item.RelevanceScore,
		}
	}

	return results, nil
}
