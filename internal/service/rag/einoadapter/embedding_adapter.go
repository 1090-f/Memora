package einoadapter

import (
	"context"
	"fmt"
	"math"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/components/embedding"
)

// ContractsEmbeddingAdapter 将成员二提供的 contracts.EmbeddingModel 包装为
// Eino embedding.Embedder，负责 float32/float64 转换、NaN/Inf 校验和维度校验。
// Service 只依赖 contracts.ModelFactory 获取模型，本适配器不接触模型密钥。
type ContractsEmbeddingAdapter struct {
	model contracts.EmbeddingModel
}

// NewContractsEmbeddingAdapter 构造适配器。model 为空时返回错误，避免后续静默失败。
func NewContractsEmbeddingAdapter(model contracts.EmbeddingModel) (*ContractsEmbeddingAdapter, error) {
	if model == nil {
		return nil, fmt.Errorf("contracts.EmbeddingModel 不能为空")
	}
	return &ContractsEmbeddingAdapter{model: model}, nil
}

// EmbedStrings 实现 Eino embedding.Embedder。
// Eino 使用 [][]float64，contracts 使用 [][]float32，这里完成安全转换。
func (a *ContractsEmbeddingAdapter) EmbedStrings(ctx context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	// 空输入直接返回空结果，避免无谓地调用底层模型。
	if len(texts) == 0 {
		return nil, nil
	}
	vectors, err := a.model.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	// 数量必须与输入一一对应，防止后续按索引取向量时错位或越界。
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding 返回数量 %d 与输入数量 %d 不一致", len(vectors), len(texts))
	}
	expected := a.model.Dimension()
	out := make([][]float64, len(vectors))
	for i, vec := range vectors {
		// 模型声明了固定维度时校验一致性，避免维度漂移污染向量库。
		if expected > 0 && len(vec) != expected {
			return nil, fmt.Errorf("embedding 维度 %d 与模型维度 %d 不一致", len(vec), expected)
		}
		converted := make([]float64, len(vec))
		for j, v := range vec {
			// NaN/Inf 无法参与余弦相似度计算，提前拒绝防止脏数据入库。
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("embedding 包含非法数值(NaN/Inf)")
			}
			converted[j] = float64(v)
		}
		out[i] = converted
	}
	return out, nil
}
