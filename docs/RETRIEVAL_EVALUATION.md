# 离线检索评估

Memora 的离线评估使用版本化金标集调用与线上相同的 `RetrievalService`，输出一次可追溯的 Experiment。它用于比较分块、索引、召回、RRF、阈值和 Reranker 配置，不把向量相似度或 Reranker 分数误当成系统准确率。

## 1. 评估对象

评估分为三层：

1. 候选召回：关注正确证据是否进入候选集，主要看 Recall@K 和 HitRate@K。
2. 最终上下文：关注 Reranker 与阈值过滤后的排序和噪声，主要看 Precision@K、MRR、nDCG@K 和 SourceSpan Hit Rate。
3. 最终回答：Correctness、Faithfulness 和 Citation 指标尚未接入，本命令不声称评估生成质量。

当前生产检索最多返回 20 条结果，因此评估 K 也不能超过 20。

## 2. 金标数据集

从 `internal/service/rag/evaluation/testdata/gold.example.json` 复制模板。数据集使用严格 JSON，未知字段会直接报错。

```json
{
  "schema_version": "retrieval-gold-v1",
  "name": "退款政策核心回归集",
  "version": "2026-09-08-v1",
  "metadata": { "owner": "knowledge-team" },
  "cases": [
    {
      "id": "refund-001",
      "question": "退款需要多长时间？",
      "expected_answerable": true,
      "reference_answer": "审核通过后 3—5 个工作日到账",
      "tags": ["退款", "直接事实"],
      "difficulty": "easy",
      "relevant_sources": [
        {
          "document_id": "真实 document_id",
          "start_byte": 1280,
          "end_byte": 1460,
          "relevance": 3,
          "document_version": "v3",
          "document_fingerprint": "可选内容摘要"
        }
      ]
    },
    {
      "id": "refund-unanswerable-001",
      "question": "尚未写入知识库的退款渠道是什么？",
      "expected_answerable": false,
      "relevant_sources": [],
      "tags": ["不可回答"]
    }
  ]
}
```

约束：

- 可回答样例必须提供至少一个 `relevant_sources`。
- 每个来源必须提供 `document_id`，或提供可移植的 `document_title`；标题比较会忽略大小写和 `.md` 后缀。
- `source_text` 可用于对检索结果正文做确定性精确包含匹配，适合数据库 ID 暂时不可用的已导入 Markdown 语料。
- `start_byte/end_byte` 都为 0 时只判断文档命中；提供区间时按 Canonical UTF-8 byte offset 判断相交。
- `relevance` 为 1～3；省略或填 0 时按 1 处理，用于兼容旧数据。
- `reference_answer` 为后续答案评估预留，目前不参与检索指标计算。
- 真实金标集不要提交敏感问题、正文或凭据。

建议分别维护核心金标集、困难/回归集和脱敏真实问题集，并冻结版本。文档内容变化后，应同步更新文档版本、指纹和 SourceSpan。

只校验文件而不连接数据库：

```powershell
go run ./cmd/eval-retrieval --dataset ./evaluation/gorm/gold.v1.json --validate-only
```

## 3. 运行评估

数据库、知识库、文档索引和所需模型配置必须已经可用。命令只初始化 PostgreSQL、模型工厂和生产检索链路，不会启动 HTTP、Redis、MinIO、解析器或后台 Worker。

```powershell
go run ./cmd/eval-retrieval `
  --dataset ./evaluation/refund-gold.json `
  --user-id <user-id> `
  --kb-id <knowledge-base-id> `
  --mode hybrid `
  --ks 1,3,5,10 `
  --top-k 10 `
  --name "hybrid-baseline" `
  --git-commit <commit-sha> `
  --output ./evaluation/results/hybrid-baseline.json
```

省略 `--output` 时将 JSON 写到标准输出。省略 `--search-config` 时使用知识库数据库中保存的检索配置。

要评估候选配置，准备一个与 `contracts.SearchConfig` 对应的严格 JSON：

```json
{
  "keyword_top_k": 30,
  "vector_top_k": 30,
  "rrf_k": 60,
  "rrf_top_k": 20,
  "reranker_top_k": 8,
  "reranker_threshold": 0.45,
  "reranker_model_id": "",
  "minimum_effective_results": 1,
  "min_vector_score": 0.3,
  "ambiguous_score": 0.45
}
```

然后运行：

```powershell
go run ./cmd/eval-retrieval `
  --dataset ./evaluation/refund-gold.json `
  --user-id <user-id> `
  --kb-id <knowledge-base-id> `
  --search-config ./evaluation/candidate-search-config.json `
  --baseline ./evaluation/results/hybrid-baseline.json `
  --name "reranker-threshold-045" `
  --output ./evaluation/results/reranker-threshold-045.json
```

提供 `--baseline` 后，输出的 `comparison` 会包含 Recall、HitRate、Precision、MRR、nDCG、SourceSpan 和不可回答误接受率的差值。正向指标差值越大越好；`accepted_context_false_positive_delta` 越小越好。

## 4. 指标语义

- `recall_at_k`：所有正确来源中，Top-K 覆盖了多少；按可回答问题做 macro average。
- `hit_rate_at_k`：Top-K 至少命中一个正确来源的问题比例。
- `precision_at_k`：Top-K 中相关结果数除以 K；返回不足 K 条时，缺失位置按不相关处理。
- `mrr`：第一个相关结果倒数排名的平均值。
- `ndcg_at_k`：考虑排序位置和 1～3 级相关性的归一化折损累计增益。
- `source_span_hit_rate`：有精确 SourceSpan 金标的问题中，最大 K 内命中正确区间的比例。
- `source_text_hit_rate`：使用证据原文标注的问题中，最大 K 内同时命中文档标题和证据原文的比例。
- `accepted_context_false_positive_rate`：不可回答问题经过生产阈值、融合和重排后仍返回最终上下文的比例。
- `unanswerable_false_positive_rate`：为兼容旧报告保留，与上一字段当前含义相同。

`report.cases` 保存逐题 Hit、Recall、Precision、nDCG、首个命中排名和失败状态；调参时应先检查失败问题，不能只看总体平均值。

## 5. 回归策略

第一版不要凭空设行业阈值。先在冻结的金标集上生成可信 baseline，再采用以下门禁：

1. 核心回归问题不得从成功变为失败。
2. Recall@候选 K、nDCG@最终 K 不得超过团队约定的容忍下降。
3. 不可回答误接受率不得上升。
4. 分别检查不同 tags 和难度，不用总平均掩盖类别退化。

PR 可以运行固定的小型核心集；完整数据集和将来的 LLM Judge 更适合夜间或发布前运行。真实评估依赖数据库、索引和模型，因此当前不会自动加入默认 GitHub Actions。
