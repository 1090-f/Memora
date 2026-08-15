# 任务包 07：混合检索交付记录

- 新增启动期编译的 Eino Retrieval Graph：校验、Retriever、RRF、Reranker、知识判断、Citation。
- `ParadeDBKeywordRetriever` 与 `PgVectorRetriever` 通过 Repository 参数化查询，强制用户、知识库、成功文档、软删除、活动索引和 ready 向量过滤。
- hybrid 使用受 Context 约束的并发分支；RRF 按 `1/(k+rank)` 合并 Chunk ID。
- Reranker 返回索引执行越界、重复、缺失校验；失败按冻结策略降级 RRF，并执行稳定排序。
- 无有效依据正常返回 `knowledge_status=insufficient`，不伪造引用或默认转 HTTP 422。
- 生产 `RetrievalService` 从归属校验后的搜索配置和模型配置装配请求，API 与工具共享 contracts。

运行态仍需真实 PostgreSQL、pgvector、Embedding 与 Reranker 环境验收。
