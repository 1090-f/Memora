package chunking

import "testing"

func TestCompareChunksIdentical(t *testing.T) {
	chunks := []ParsedChunk{{Content: "第一块", BlockIDs: []string{"b1"}}, {Content: "第二块", BlockIDs: []string{"b2"}}}
	report := CompareChunks(chunks, append([]ParsedChunk(nil), chunks...), StrategyStructured, "v1")
	if report.ExactContentMatches != 2 || report.BoundaryDifferenceRate != 0 || report.SourceDifferenceRate != 0 {
		t.Fatalf("相同分块差异报告异常: %+v", report)
	}
}

func TestCompareChunksReportsBoundaryAndSourceChanges(t *testing.T) {
	legacy := []ParsedChunk{{Content: "甲", BlockIDs: []string{"b1"}}, {Content: "乙", BlockIDs: []string{"b2"}}}
	candidate := []ParsedChunk{{Content: "甲乙", BlockIDs: []string{"b1", "b2"}}}
	report := CompareChunks(legacy, candidate, StrategyParagraph, "v2")
	if report.BoundaryDifferenceRate != 1 || report.SourceDifferenceRate <= 0 || report.CandidateChunkCount != 1 {
		t.Fatalf("未报告边界/来源变化: %+v", report)
	}
}
