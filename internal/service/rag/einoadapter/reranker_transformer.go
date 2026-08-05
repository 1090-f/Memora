package einoadapter

import (
	"context"
	"fmt"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// ContractsRerankerTransformer 将成员二提供的 contracts.Reranker 包装为
// Eino document.Transformer。查询文本从文档 MetaData 的 MetaQuery 键读取
// （由检索 Graph 的入口节点写入），以保持组件无状态、可在初始化时单例编译。
type ContractsRerankerTransformer struct {
	reranker contracts.Reranker
}

// NewContractsRerankerTransformer 构造重排 Transformer。reranker 为空时返回错误。
func NewContractsRerankerTransformer(reranker contracts.Reranker) (*ContractsRerankerTransformer, error) {
	if reranker == nil {
		return nil, fmt.Errorf("contracts.Reranker 不能为空")
	}
	return &ContractsRerankerTransformer{reranker: reranker}, nil
}

// Transform 实现 Eino document.Transformer，按 query 对输入文档重排并重写排序分数。
// 返回的文档顺序即最终相关性顺序，每份文档保留原有 MetaData 并写入 reranker_score。
func (t *ContractsRerankerTransformer) Transform(ctx context.Context, docs []*schema.Document, _ ...document.TransformerOption) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}
	query := GetMetaString(docs[0].MetaData, MetaQuery)
	if query == "" {
		return nil, fmt.Errorf("reranker 缺少 query metadata，无法重排")
	}

	contents := make([]string, len(docs))
	for i, d := range docs {
		contents[i] = d.Content
	}
	items, err := t.reranker.Rerank(ctx, query, contents, len(docs))
	if err != nil {
		return nil, err
	}

	seen := make(map[int]struct{}, len(items))
	reordered := make([]*schema.Document, 0, len(items))
	for _, item := range items {
		if item.Index < 0 || item.Index >= len(docs) {
			return nil, fmt.Errorf("reranker 返回越界 index %d", item.Index)
		}
		if _, dup := seen[item.Index]; dup {
			return nil, fmt.Errorf("reranker 返回重复 index %d", item.Index)
		}
		seen[item.Index] = struct{}{}
		d := docs[item.Index]
		if d.MetaData == nil {
			d.MetaData = make(map[string]any)
		}
		d.MetaData[MetaRerankerScore] = item.Score
		reordered = append(reordered, d)
	}
	return reordered, nil
}
