package preview

import (
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

func TestStripDoclingImagePlaceholdersKeepsPresentationTextReadable(t *testing.T) {
	markdown := strings.Join([]string{
		"# 第 6 页",
		"",
		"<!-- image -->",
		"",
		"",
		"结语",
		"",
		"<!-- image kind=picture -->",
		"",
		"谢谢观看",
	}, "\n")

	got := stripDoclingImagePlaceholders(markdown)
	if strings.Contains(got, "<!-- image") {
		t.Fatalf("图片占位符未清理：\n%s", got)
	}
	if got != "# 第 6 页\n\n结语\n\n谢谢观看" {
		t.Fatalf("正文或段落间距异常：\n%s", got)
	}
}

func TestBuildPresentationSlidesGroupsTextAndImagesByPage(t *testing.T) {
	parsed := &parser.ParsedDocument{
		Blocks: []parser.Block{
			{Type: parser.BlockTypeTitle, Text: "开场", Source: parser.SourceLocation{Page: 1}},
			{Type: parser.BlockTypePicture, AssetRefs: []string{"asset-1"}, Source: parser.SourceLocation{Page: 1}},
			{Type: parser.BlockTypeParagraph, Markdown: "正文", Source: parser.SourceLocation{Page: 2}},
		},
		Assets: []parser.Asset{{ID: "asset-1", ObjectKey: "asset.png", Caption: "关键示意图", Width: 800, Height: 600}},
	}
	svc := &service{assetSignKey: "test-secret"}

	slides := svc.buildPresentationSlides(parsed, "doc-1")
	if len(slides) != 2 {
		t.Fatalf("slides 数量 = %d, want 2", len(slides))
	}
	if slides[0].Page != 1 || slides[0].Content != "### 开场" || len(slides[0].Images) != 1 {
		t.Fatalf("第一页结构异常: %#v", slides[0])
	}
	if !strings.Contains(slides[0].Images[0].URL, "/api/v1/documents/doc-1/assets/asset-1") || slides[0].Images[0].Alt != "关键示意图" {
		t.Fatalf("第一页图片异常: %#v", slides[0].Images[0])
	}
	if slides[1].Page != 2 || slides[1].Content != "正文" || slides[1].Images == nil || len(slides[1].Images) != 0 {
		t.Fatalf("第二页结构异常: %#v", slides[1])
	}
}
