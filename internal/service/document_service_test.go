package service

import (
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/service/rag/parser"
	"github.com/1090-f/Memora/pkg/asseturl"
)

func TestDocumentIndexMode(t *testing.T) {
	activeVersion := 1
	embeddingModelID := "embedding-model"

	tests := []struct {
		name string
		doc  *entity.Document
		want string
	}{
		{name: "nil document", want: string(contracts.DocumentIndexNone)},
		{name: "no active index", doc: &entity.Document{}, want: string(contracts.DocumentIndexNone)},
		{name: "keyword index", doc: &entity.Document{ActiveIndexVersion: &activeVersion}, want: string(contracts.DocumentIndexKeyword)},
		{name: "hybrid index", doc: &entity.Document{ActiveIndexVersion: &activeVersion, EmbeddingModelID: &embeddingModelID}, want: string(contracts.DocumentIndexHybrid)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := documentIndexMode(tt.doc); got != tt.want {
				t.Fatalf("documentIndexMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteMarkdownImageRefs(t *testing.T) {
	assets := []parser.Asset{
		{ID: "asset-aaaa", SourceRef: "img/logo.png", ObjectKey: "o1", MIMEType: "image/png"},
		{ID: "asset-bbbb", SourceRef: "https://example.com/pic.jpg", ObjectKey: "o2"},
		{ID: "asset-cccc", SourceRef: "missing.png", ObjectKey: "", Omitted: false},
	}
	content := strings.Join([]string{
		"# 标题",
		"![Logo](img/logo.png)",
		"![网络图](https://example.com/pic.jpg)",
		"![缺图](missing.png)",
		"![绝对路径](C:\\Users\\foo\\Desktop\\a.jpg)",
	}, "\n")

	svc := &documentService{assetSignKey: "test-secret"}
	got := svc.rewriteMarkdownImageRefs(content, assets, "doc-1")

	wantContains := []string{
		"![Logo](/api/v1/documents/doc-1/assets/asset-aaaa?exp=",
		"![网络图](/api/v1/documents/doc-1/assets/asset-bbbb?exp=",
		"![缺图](missing.png)",
		"![绝对路径](C:\\Users\\foo\\Desktop\\a.jpg)",
	}
	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("重写结果缺少 %q：\n%s", want, got)
		}
	}
}

func TestRewriteDoclingImagePlaceholders(t *testing.T) {
	assets := []parser.Asset{
		{ID: "asset-aaaa", ObjectKey: "o1"},
		{ID: "asset-omitted", ObjectKey: "", Omitted: true},
		{ID: "asset-bbbb", ObjectKey: "o2"},
	}
	blocks := []parser.Block{
		{Type: parser.BlockTypeHeading},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-aaaa"}},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-omitted"}},
		{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-bbbb"}},
	}
	markdown := strings.Join([]string{
		"# 标题",
		"<!-- image -->",
		"中间文字",
		"<!-- image -->",
		"<!-- image -->",
	}, "\n")

	svc := &documentService{assetSignKey: "test-secret"}
	got := svc.rewriteDoclingImagePlaceholders(markdown, blocks, assets, "doc-1")

	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[1], "/api/v1/documents/doc-1/assets/asset-aaaa?exp=") {
		t.Errorf("第一个占位符应替换为 asset-aaaa 签名 URL: %q", lines[1])
	}
	if lines[3] != "<!-- image -->" {
		t.Errorf("omitted 图片的占位符应保持原样: %q", lines[3])
	}
	if !strings.Contains(lines[4], "/api/v1/documents/doc-1/assets/asset-bbbb?exp=") {
		t.Errorf("第三个占位符应替换为 asset-bbbb 签名 URL: %q", lines[4])
	}
}

func TestAssetURLSignAndVerify(t *testing.T) {
	exp, sig, err := asseturl.Sign("secret", "doc-1", "asset-1", time.Hour)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if !asseturl.Verify("secret", "doc-1", "asset-1", exp, sig) {
		t.Error("合法签名应通过校验")
	}
	if asseturl.Verify("wrong", "doc-1", "asset-1", exp, sig) {
		t.Error("错误密钥应校验失败")
	}
	if asseturl.Verify("secret", "doc-2", "asset-1", exp, sig) {
		t.Error("篡改文档 ID 应校验失败")
	}
	expiredExp, expiredSig, _ := asseturl.Sign("secret", "doc-1", "asset-1", 2*time.Second)
	time.Sleep(3500 * time.Millisecond)
	if asseturl.Verify("secret", "doc-1", "asset-1", expiredExp, expiredSig) {
		t.Error("过期签名应校验失败")
	}
}
