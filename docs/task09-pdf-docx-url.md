# 任务包 09：PDF、DOCX 与 URL 导入交付记录

- PDF/DOCX 复用已落地的 Docling Python Parser 与统一 ParsedDocument/结构分块链。
- 新增 Eino `SafeWebLoader`：仅 HTTP/HTTPS，拒绝 localhost、回环、私网、链路本地、组播、未指定地址和云元数据主机。
- 初始请求和每次重定向均重新校验；自定义 DialContext 再次解析并校验 IP，降低 DNS rebinding 风险。
- 限制重定向次数、连接/响应头/总超时、解压后响应体大小和 Content-Type。
- HTML 提取可见文本；URL 来源随后进入与文件相同的解析、Artifact、清洗、分段和索引 Graph。
- HTTP API 只创建 pending 任务，网络抓取在 Worker 执行。

未对公网 URL、压缩炸弹、损坏 PDF/DOCX 做本机运行态验收。
