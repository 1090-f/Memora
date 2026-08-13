package loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

const (
	defaultImageTimeout  = 30 * time.Second
	defaultImageMaxBytes = int64(32 * 1024 * 1024)
)

// MarkdownAssetLoader 解析 Markdown 图片引用：
//   - http(s) URL：SSRF 防护下载，仅接受 image/* Content-Type，限制大小；
//   - 相对路径：从随文档上传的附件映射读取（zip 导入时填充）。
//
// 实现 parser.AssetLoader。
type MarkdownAssetLoader struct {
	store       parser.ObjectStore
	attachments map[string]string
	client      *http.Client
	resolver    *net.Resolver
	maxBytes    int64
	userAgent   string
}

// NewMarkdownAssetLoader 构造 Markdown 图片资源加载器。
// attachments 是 zip 附件映射（相对路径 → MinIO object key）；可为 nil。
func NewMarkdownAssetLoader(store parser.ObjectStore, attachments map[string]string) *MarkdownAssetLoader {
	resolver := net.DefaultResolver
	loader := &MarkdownAssetLoader{
		store:       store,
		attachments: attachments,
		resolver:    resolver,
		maxBytes:    defaultImageMaxBytes,
		userAgent:   "Memora-Document-Importer/1.0",
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("解析目标地址失败: %w", err)
			}
			ips, err := resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("解析目标主机失败: %w", err)
			}
			for _, candidate := range ips {
				if isForbiddenIP(candidate.IP) {
					return nil, fmt.Errorf("目标地址不允许访问")
				}
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("目标主机没有可用地址")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	loader.client = &http.Client{Transport: transport, Timeout: defaultImageTimeout}
	loader.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("URL 重定向超过上限 5")
		}
		return loader.validateURL(req.Context(), req.URL)
	}
	return loader
}

// Open 实现 parser.AssetLoader。
func (l *MarkdownAssetLoader) Open(ctx context.Context, ref string) (io.ReadCloser, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, "", errors.New("图片引用为空")
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return l.openURL(ctx, ref)
	}
	key, ok := l.attachments[archiveRefPath(ref)]
	if !ok {
		// 兼容绝对路径引用（如 C:\...）：按文件名匹配 zip 内附件。
		key, ok = l.matchByBaseName(ref)
	}
	if !ok {
		return nil, "", fmt.Errorf("附件 %q 未随文档上传", ref)
	}
	if l.store == nil {
		return nil, "", errors.New("附件存储未配置")
	}
	reader, err := l.store.OpenObject(ctx, key)
	if err != nil {
		return nil, "", fmt.Errorf("读取附件 %q 失败: %w", ref, err)
	}
	return reader, "", nil
}

// matchByBaseName 按文件名（不含目录，兼容 / 与 \ 分隔符）匹配附件；
// 只有文件名唯一时才回退，避免同名图片随机串到错误目录。
func (l *MarkdownAssetLoader) matchByBaseName(ref string) (string, bool) {
	want := strings.ToLower(fileBase(ref))
	if want == "" || want == "." || want == ".." {
		return "", false
	}
	var matchedKey string
	matches := 0
	for attachmentPath, key := range l.attachments {
		if strings.ToLower(fileBase(attachmentPath)) == want {
			matchedKey = key
			matches++
		}
	}
	if matches != 1 {
		return "", false
	}
	return matchedKey, true
}

func archiveRefPath(ref string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(ref), "\\", "/")
	return path.Clean(normalized)
}

// fileBase 返回路径最后一段，兼容 Windows（\）与 Unix（/）分隔符。
func fileBase(p string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}

// openURL 下载网络图片并强制 SSRF/大小/类型限制。
func (l *MarkdownAssetLoader) openURL(ctx context.Context, rawURL string) (io.ReadCloser, string, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("URL 格式无效: %w", err)
	}
	if err := l.validateURL(ctx, target); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建图片请求失败: %w", err)
	}
	req.Header.Set("User-Agent", l.userAgent)
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载图片失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("图片 URL 返回非成功状态 %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("URL Content-Type %q 不是图片", contentType)
	}
	if resp.ContentLength > l.maxBytes {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("图片超过大小上限 %d 字节", l.maxBytes)
	}
	return &limitedReadCloser{Reader: io.LimitReader(resp.Body, l.maxBytes+1), Closer: resp.Body}, contentType, nil
}

func (l *MarkdownAssetLoader) validateURL(ctx context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return fmt.Errorf("仅允许不含用户信息的 HTTP/HTTPS URL")
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.Contains(host, "metadata.google.internal") || strings.Contains(host, "metadata.azure.internal") {
		return fmt.Errorf("目标主机不允许访问")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return fmt.Errorf("目标地址不允许访问")
		}
		return nil
	}
	ips, err := l.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("解析目标主机失败: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("目标主机没有可用地址")
	}
	for _, address := range ips {
		if isForbiddenIP(address.IP) {
			return fmt.Errorf("目标地址不允许访问")
		}
	}
	return nil
}

// limitedReadCloser 限制读取大小（配合调用方的 LimitReader 超限检测）。
type limitedReadCloser struct {
	io.Reader
	io.Closer
}
