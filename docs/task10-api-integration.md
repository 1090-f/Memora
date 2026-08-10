# 任务包 10：API 与集成交付记录

补齐接口：

- `POST /api/v1/knowledge-bases/:kb_id/imports/url`
- `GET /api/v1/documents/:document_id/index-versions`
- `POST /api/v1/knowledge-bases/:kb_id/search`
- `POST /api/v1/knowledge-bases/:kb_id/search/test`

同时将真实 RetrievalService、模型工厂、检索 Graph、CitationService、DocumentReader 和文档 Embedding 解析器在 `internal/app` 统一组装；模型 API Key 由应用加密服务写入，模型客户端使用配置的模型名和显式超时。文档索引与查询向量均通过同一 ModelFactory；重新索引与失败重试会创建真实 Worker 任务，不再只返回静态成功。

导入任务在首次创建文档后立即持久化 `document_id` 关联；流水线中途失败再重试会复用同一文档和同一索引版本槽位，不重复创建业务文档。

最终运行态联调依赖 PostgreSQL/pgvector、Redis、MinIO、Docling、Embedding 与 Reranker，当前文档不将静态编译等同于端到端验收。
