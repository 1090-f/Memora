package preview

import (
	"path"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
)

type documentKind string

const (
	kindText     documentKind = "text"
	kindMarkdown documentKind = "markdown"
	kindPDF      documentKind = "pdf"
	kindDOCX     documentKind = "docx"
	kindPPTX     documentKind = "pptx"
	kindXLSX     documentKind = "xlsx"
	kindImage    documentKind = "image"
	kindUnknown  documentKind = "unknown"
)

func classify(doc *entity.Document) documentKind {
	if doc == nil {
		return kindUnknown
	}
	if doc.SourceType == string(contracts.DocumentSourceManual) {
		if strings.EqualFold(doc.ContentFormat, "markdown") {
			return kindMarkdown
		}
		return kindText
	}
	if doc.SourceType == string(contracts.DocumentSourceURL) {
		return kindMarkdown
	}
	name := doc.Title
	if doc.OriginalFileName != nil && strings.TrimSpace(*doc.OriginalFileName) != "" {
		name = *doc.OriginalFileName
	}
	switch strings.ToLower(path.Ext(strings.TrimSpace(name))) {
	case ".txt":
		return kindText
	case ".md", ".markdown":
		return kindMarkdown
	case ".pdf":
		return kindPDF
	case ".docx":
		return kindDOCX
	case ".pptx":
		return kindPPTX
	case ".xlsx":
		return kindXLSX
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif", ".gif", ".webp":
		return kindImage
	}
	if doc.MIMEType != nil {
		mimeType := strings.ToLower(strings.TrimSpace(*doc.MIMEType))
		switch {
		case mimeType == "application/pdf":
			return kindPDF
		case strings.HasPrefix(mimeType, "image/"):
			return kindImage
		case mimeType == "text/markdown", mimeType == "text/x-markdown":
			return kindMarkdown
		case mimeType == "text/plain":
			return kindText
		case strings.Contains(mimeType, "wordprocessingml"):
			return kindDOCX
		case strings.Contains(mimeType, "presentationml"):
			return kindPPTX
		case strings.Contains(mimeType, "spreadsheetml"):
			return kindXLSX
		}
	}
	return kindUnknown
}

func documentURL(documentID, suffix string) string {
	return "/api/v1/documents/" + documentID + suffix
}
