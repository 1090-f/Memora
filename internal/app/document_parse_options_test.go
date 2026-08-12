package app

import (
	"testing"

	"github.com/1090-f/Memora/pkg/config"
)

// TestDocumentParseOptionsPassesDoImageOCR 验证 DoImageOCR 配置正确传递到解析选项：
// do_ocr（整页/整图 OCR）与 do_image_ocr（文档内图片二级 OCR）相互独立。
func TestDocumentParseOptionsPassesDoImageOCR(t *testing.T) {
	cfg := &config.Config{}
	cfg.DocumentParser.DoOCR = true
	cfg.DocumentParser.DoImageOCR = true
	opts := documentParseOptions(cfg)
	if !opts.DoImageOCR {
		t.Error("DoImageOCR=true 时解析选项应携带 do_image_ocr=true")
	}
	if !opts.DoOCR {
		t.Error("DoOCR 不应受 DoImageOCR 影响")
	}

	cfg.DocumentParser.DoImageOCR = false
	opts = documentParseOptions(cfg)
	if opts.DoImageOCR {
		t.Error("DoImageOCR=false 时解析选项应携带 do_image_ocr=false")
	}
	if !opts.DoOCR {
		t.Error("DoOCR 应保持独立配置")
	}

	cfg.DocumentParser.DoOCR = false
	cfg.DocumentParser.DoImageOCR = true
	opts = documentParseOptions(cfg)
	if !opts.DoImageOCR {
		t.Error("仅启用 DoImageOCR 时也应正确传递")
	}
	if opts.DoOCR {
		t.Error("DoOCR=false 时解析选项应携带 do_ocr=false")
	}
}
