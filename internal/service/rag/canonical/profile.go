package canonical

import (
	"fmt"
	"math"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

// Profile derives deterministic routing features from a canonical document.
func Profile(doc *CanonicalDocument, parsed *parser.ParsedDocument, counter TokenCounter) (DocumentProfile, error) {
	if doc == nil || parsed == nil {
		return DocumentProfile{}, fmt.Errorf("DocumentProfile 需要 CanonicalDocument 和 ParsedDocument")
	}
	profile := DocumentProfile{
		SourceFormat: parsed.Source.Format, PageCount: parsed.Document.PageCount,
		DocumentBytes: len(doc.Markdown), NodeCount: len(doc.Nodes),
		WarningCount: len(parsed.Warnings),
	}
	if counter != nil {
		tokens, err := counter.Count(doc.Markdown)
		if err != nil {
			return DocumentProfile{}, err
		}
		profile.DocumentTokens = tokens
	}

	paragraphTokens := make([]float64, 0)
	headingScoped := 0
	for _, node := range doc.Nodes {
		if len(node.HeadingPath) > 0 {
			headingScoped++
			profile.HasReliableHeadingPath = true
			if len(node.HeadingPath) > profile.HeadingDepth {
				profile.HeadingDepth = len(node.HeadingPath)
			}
		}
		switch node.Kind {
		case NodeKindHeading:
			profile.HeadingCount++
		case NodeKindParagraph, NodeKindListItem, NodeKindFootnote, NodeKindUnknown:
			profile.ParagraphCount++
			if counter != nil {
				value, err := counter.Count(node.Text)
				if err != nil {
					return DocumentProfile{}, err
				}
				paragraphTokens = append(paragraphTokens, float64(value))
			}
		case NodeKindTable:
			profile.TableRatio++
		case NodeKindPicture:
			profile.PictureRatio++
		case NodeKindCode, NodeKindFormula:
			profile.CodeRatio++
		}
	}
	denominator := float64(max(1, len(doc.Nodes)))
	profile.HeadingCoverage = float64(headingScoped) / denominator
	profile.TableRatio /= denominator
	profile.PictureRatio /= denominator
	profile.CodeRatio /= denominator
	if len(paragraphTokens) > 0 {
		for _, value := range paragraphTokens {
			profile.AverageParagraphTokens += value
		}
		profile.AverageParagraphTokens /= float64(len(paragraphTokens))
		for _, value := range paragraphTokens {
			delta := value - profile.AverageParagraphTokens
			profile.ParagraphTokenVariance += delta * delta
		}
		profile.ParagraphTokenVariance /= float64(len(paragraphTokens))
		if math.IsNaN(profile.ParagraphTokenVariance) {
			profile.ParagraphTokenVariance = 0
		}
	}
	return profile, nil
}
