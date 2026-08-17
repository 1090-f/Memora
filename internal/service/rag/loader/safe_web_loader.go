// Package loader 提供受限外部来源的 Eino document.Loader 实现。
package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/net/html"
)

const (
	defaultWebTimeout   = 30 * time.Second
	defaultWebMaxBytes  = int64(10 * 1024 * 1024)
	defaultWebRedirects = 5
)

// SafeWebConfig 定义 URL 导入的 SSRF 与资源上限。
type SafeWebConfig struct {
	Timeout      time.Duration
	MaxBytes     int64
	MaxRedirects int
	UserAgent    string
}

// SafeWebLoader 是拒绝本机/私网/元数据地址并限制重定向和响应体的 Eino Loader。
type SafeWebLoader struct {
	client    *http.Client
	resolver  *net.Resolver
	maxBytes  int64
	userAgent string
}

func NewSafeWebLoader(cfg SafeWebConfig) *SafeWebLoader {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultWebTimeout
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultWebMaxBytes
	}
	if cfg.MaxRedirects <= 0 {
		cfg.MaxRedirects = defaultWebRedirects
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Memora-Document-Importer/1.0"
	}
	resolver := net.DefaultResolver
	loader := &SafeWebLoader{resolver: resolver, maxBytes: cfg.MaxBytes, userAgent: cfg.UserAgent}
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
		DisableCompression:    false,
	}
	loader.client = &http.Client{Transport: transport, Timeout: cfg.Timeout}
	loader.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= cfg.MaxRedirects {
			return fmt.Errorf("URL 重定向超过上限 %d", cfg.MaxRedirects)
		}
		return loader.validateURL(req.Context(), req.URL)
	}
	return loader
}

func (l *SafeWebLoader) Load(ctx context.Context, src document.Source, _ ...document.LoaderOption) ([]*schema.Document, error) {
	target, err := url.Parse(strings.TrimSpace(src.URI))
	if err != nil {
		return nil, fmt.Errorf("URL 格式无效: %w", err)
	}
	if err := l.validateURL(ctx, target); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建 URL 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", l.userAgent)
	req.Header.Set("Accept", "text/html,text/plain,text/markdown;q=0.9")
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("抓取 URL 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("URL 返回非成功状态 %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if contentType != "text/html" && contentType != "text/plain" && contentType != "text/markdown" && contentType != "text/x-markdown" {
		return nil, fmt.Errorf("URL Content-Type %q 不受支持", contentType)
	}
	limited := io.LimitReader(resp.Body, l.maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("读取 URL 响应失败: %w", err)
	}
	if int64(len(body)) > l.maxBytes {
		return nil, fmt.Errorf("URL 响应超过大小上限 %d", l.maxBytes)
	}
	content, title := string(body), ""
	if contentType == "text/html" {
		content, title, err = extractHTML(body)
		if err != nil {
			return nil, fmt.Errorf("解析 HTML 失败: %w", err)
		}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("URL 未产生可用正文")
	}
	digest := sha256.Sum256([]byte(content))
	fetchedAt := time.Now().UTC()
	return []*schema.Document{{
		ID: hex.EncodeToString(digest[:]), Content: content,
		MetaData: map[string]any{
			"source_url": src.URI, "final_url": resp.Request.URL.String(), "title": title,
			"content_type": contentType, "fetched_at": fetchedAt.Format(time.RFC3339Nano),
			"source_hash": hex.EncodeToString(digest[:]), "size": len(body),
		},
	}}, nil
}

func (l *SafeWebLoader) validateURL(ctx context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return fmt.Errorf("仅允许不含用户信息的 HTTP/HTTPS URL")
	}
	// 端口白名单：仅允许标准 80/443，阻断公网任意端口探测。
	if port := target.Port(); port != "" && port != "80" && port != "443" {
		return fmt.Errorf("仅允许 80/443 端口，当前端口 %q", port)
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

func isForbiddenIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip.IsInterfaceLocalMulticast() {
		return true
	}
	// IPv6 特殊地址：Go 的 IsPrivate 仅覆盖 IPv4 RFC1918，
	// ULA（fc00::/7）与站点本地（fec0::/10）需显式拒绝。
	if ip4 := ip.To4(); ip4 == nil && len(ip) == net.IPv6len {
		if ip[0]&0xfe == 0xfc || ip[0] == 0xfe && ip[1]&0xc0 == 0xc0 {
			return true
		}
	}
	return false
}

func extractHTML(body []byte) (string, string, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	var title string
	var output strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, hidden bool) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "script" || tag == "style" || tag == "noscript" || tag == "svg" {
				hidden = true
			}
			if tag == "br" || tag == "p" || tag == "div" || tag == "li" || tag == "h1" || tag == "h2" || tag == "h3" || tag == "tr" {
				output.WriteByte('\n')
			}
		}
		if !hidden && node.Type == html.TextNode {
			text := strings.Join(strings.Fields(node.Data), " ")
			if text != "" {
				output.WriteString(text)
				output.WriteByte(' ')
			}
			if node.Parent != nil && strings.EqualFold(node.Parent.Data, "title") && title == "" {
				title = text
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
	}
	walk(root, false)
	lines := strings.Split(output.String(), "\n")
	cleaned := lines[:0]
	for _, line := range lines {
		if value := strings.TrimSpace(line); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return strings.Join(cleaned, "\n\n"), strings.TrimSpace(title), nil
}
