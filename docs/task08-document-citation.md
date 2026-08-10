# 任务包 08：DocumentReader 与 CitationService 交付记录

- 新增实现 `contracts.DocumentService` 的受限 DocumentReader。
- Repository 在同一查询中校验 user、knowledge base、document、`succeeded`、软删除和活动索引版本。
- 读取支持 Section、Cursor、MaxTokens；MaxTokens 最大 6000。
- Cursor 使用 HMAC 签名，并绑定用户、知识库、文档、Section、索引版本、Chunk 与字符偏移；活动索引切换后旧游标返回版本冲突。
- `CitationService` 统一 Retrieval 和 DocumentReader 的可信文档/Chunk/位置/更新时间映射。

未新增永久测试文件；真实跨用户与游标篡改运行态验收仍需数据库环境。
