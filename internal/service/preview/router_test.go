package preview

import (
	"testing"

	"github.com/1090-f/Memora/internal/model/entity"
)

func TestClassifyUsesSourceAndFileNameBeforeMIME(t *testing.T) {
	fileName := "季度报表.XLSX"
	mimeType := "application/octet-stream"
	tests := []struct {
		name string
		doc  *entity.Document
		want documentKind
	}{
		{name: "manual markdown", doc: &entity.Document{SourceType: "manual", ContentFormat: "markdown"}, want: kindMarkdown},
		{name: "url", doc: &entity.Document{SourceType: "url"}, want: kindMarkdown},
		{name: "file extension", doc: &entity.Document{SourceType: "file", OriginalFileName: &fileName, MIMEType: &mimeType}, want: kindXLSX},
		{name: "mime fallback", doc: &entity.Document{SourceType: "file", Title: "scan", MIMEType: stringPointer("image/png")}, want: kindImage},
		{name: "unknown", doc: &entity.Document{SourceType: "file", Title: "archive.bin", MIMEType: &mimeType}, want: kindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.doc); got != tt.want {
				t.Fatalf("classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }
