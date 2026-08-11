package parser

import (
	"regexp"
	"strings"
)

// ImageRef 是 Markdown 图片引用（用于导入扫描与附件匹配）。
type ImageRef struct {
	Alt string `json:"alt"`
	Ref string `json:"ref"`
}

// 行内图片语法（支持可选 title："..."、'...'、(...)）。
var (
	markdownInlineImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+["'(][^)]*["')])?\)`)
	markdownRefImageRe    = regexp.MustCompile(`!\[([^\]]*)\]\[([^\]]*)\]`)
	markdownRefDefRe      = regexp.MustCompile(`^\s*\[([^\]]+)\]:\s*(\S+)(?:\s+.*)?$`)
)

// ScanMarkdownImageRefs 按出现顺序提取 Markdown 全部图片引用：
// 支持行内式 ![alt](path "title")、引用式 ![alt][ref] + [ref]: path。
// 引用式找不到定义时保留原引用（ref 为 "[ref]" 形式，交由调用方判定）。
func ScanMarkdownImageRefs(content string) []ImageRef {
	definitions := collectReferenceDefinitions(content)
	var refs []ImageRef
	for _, line := range strings.Split(content, "\n") {
		refs = append(refs, scanLineImageRefs(line, definitions)...)
	}
	return refs
}

// scanLineImageRefs 扫描单行中的图片引用（行内式 + 引用式）。
func scanLineImageRefs(line string, definitions map[string]string) []ImageRef {
	var refs []ImageRef
	// 行内式：![alt](ref "title")
	for _, match := range markdownInlineImageRe.FindAllStringSubmatch(line, -1) {
		refs = append(refs, ImageRef{Alt: strings.TrimSpace(match[1]), Ref: strings.TrimSpace(match[2])})
	}
	// 引用式：![alt][ref]，ref 指向 [ref]: path 定义。
	for _, match := range markdownRefImageRe.FindAllStringSubmatch(line, -1) {
		ref := strings.TrimSpace(match[2])
		if path, ok := definitions[strings.ToLower(ref)]; ok {
			refs = append(refs, ImageRef{Alt: strings.TrimSpace(match[1]), Ref: path})
		} else {
			refs = append(refs, ImageRef{Alt: strings.TrimSpace(match[1]), Ref: "[" + ref + "]"})
		}
	}
	return refs
}

// collectReferenceDefinitions 收集引用式图片定义：[ref]: path（全文档扫描）。
func collectReferenceDefinitions(content string) map[string]string {
	definitions := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		if match := markdownRefDefRe.FindStringSubmatch(line); len(match) == 3 {
			definitions[strings.ToLower(strings.TrimSpace(match[1]))] = strings.TrimSpace(match[2])
		}
	}
	return definitions
}
