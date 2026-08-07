package mcp

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL 校验 HTTP 传输的 URL 安全性，防止 SSRF 攻击。
// allowHTTP 为 true 时允许 localhost 的 HTTP（仅 debug 模式）。
func ValidateURL(rawURL string, allowHTTP bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "file" || scheme == "unix" {
		return errors.New("protocol not allowed: only http/https supported")
	}
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported protocol: %s", scheme)
	}
	if scheme == "http" {
		if !allowHTTP {
			return errors.New("http not allowed, use https")
		}
		host := parsed.Hostname()
		if !isLocalhost(host) {
			return errors.New("http only allowed for localhost in debug mode")
		}
		return nil
	}
	host := parsed.Hostname()
	if host == "" {
		return errors.New("url host is required")
	}
	if IsPrivateIP(host) {
		return fmt.Errorf("private or reserved ip address not allowed: %s", host)
	}
	return nil
}

// IsPrivateIP 判断 IP 是否为内网或保留地址。
func IsPrivateIP(host string) bool {
	// 解析主机名为 IP
	ip := net.ParseIP(host)
	if ip == nil {
		// 域名情况下尝试解析
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			// 无法解析时放行（运行时连接会再次校验）
			return false
		}
		ip = ips[0]
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	// 云元数据地址 169.254.169.254 属于 LinkLocalUnicast，已被覆盖
	return false
}

func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// shellMetacharacters 是禁止在 stdio command/args 中出现的 shell 元字符。
var shellMetacharacters = []string{";", "&", "|", "$", "`", ">", "<", "\n", "\r", "(", ")", "{", "}", "\\"}

// ValidateCommand 校验 stdio 命令是否在白名单内且不含注入字符。
func ValidateCommand(command string, whitelist []string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("command is required for stdio transport")
	}
	if containsShellMetachar(command) {
		return fmt.Errorf("command contains forbidden shell metacharacters: %s", command)
	}
	if !isCommandWhitelisted(command, whitelist) {
		return fmt.Errorf("command %q is not in the whitelist", command)
	}
	return nil
}

// ValidateArgs 校验 stdio 命令参数，禁止 shell 元字符注入。
func ValidateArgs(args []string) error {
	for _, arg := range args {
		if containsShellMetachar(arg) {
			return fmt.Errorf("argument contains forbidden shell metacharacters: %s", arg)
		}
	}
	return nil
}

// ValidateEnv 校验环境变量数量和值的安全性。
func ValidateEnv(env map[string]string) error {
	if len(env) > 20 {
		return errors.New("env entries exceed maximum of 20")
	}
	for k, v := range env {
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("env value for %q contains newline characters", k)
		}
	}
	return nil
}

// ValidateHeaders 校验 HTTP headers 数量和格式。
func ValidateHeaders(headers map[string]string) error {
	if len(headers) > 20 {
		return errors.New("headers exceed maximum of 20")
	}
	for k, v := range headers {
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("header value for %q contains newline characters", k)
		}
	}
	return nil
}

func containsShellMetachar(s string) bool {
	for _, mc := range shellMetacharacters {
		if strings.Contains(s, mc) {
			return true
		}
	}
	return false
}

func isCommandWhitelisted(command string, whitelist []string) bool {
	for _, allowed := range whitelist {
		if strings.TrimSpace(allowed) == command {
			return true
		}
	}
	return false
}
