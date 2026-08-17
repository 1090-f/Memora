package loader

import (
	"context"
	"net"
	"net/url"
	"testing"
)

func TestValidateURLPortWhitelist(t *testing.T) {
	loader := NewSafeWebLoader(SafeWebConfig{})
	ctx := context.Background()

	allowed := []string{
		"https://example.com/article",
		"http://example.com:80/page",
		"https://example.com:443/page",
	}
	for _, raw := range allowed {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析 %q 失败: %v", raw, err)
		}
		if err := loader.validateURL(ctx, target); err != nil {
			t.Errorf("端口应允许 %q: %v", raw, err)
		}
	}

	forbidden := []string{
		"http://example.com:8080/page",
		"https://example.com:8443/x",
		"http://example.com:22/ssh",
	}
	for _, raw := range forbidden {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("解析 %q 失败: %v", raw, err)
		}
		if err := loader.validateURL(ctx, target); err == nil {
			t.Errorf("端口应被拒绝 %q", raw)
		}
	}
}

func TestIsForbiddenIPIPv6(t *testing.T) {
	cases := []struct {
		ip        string
		forbidden bool
	}{
		{"8.8.8.8", false},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"fe80::1", true},
		{"fd12:3456::1", true},          // IPv6 ULA
		{"fc00::1", true},               // IPv6 ULA 边界
		{"fec0::1", true},               // 站点本地（已废弃）
		{"2606:4700:4700::1111", false}, // 公网 IPv6
		{"2001:4860:4860::8888", false}, // 公网 IPv6
	}
	for _, item := range cases {
		ip := net.ParseIP(item.ip)
		if ip == nil {
			t.Fatalf("无法解析测试 IP %q", item.ip)
		}
		if got := isForbiddenIP(ip); got != item.forbidden {
			t.Errorf("isForbiddenIP(%s) = %v, want %v", item.ip, got, item.forbidden)
		}
	}
}
