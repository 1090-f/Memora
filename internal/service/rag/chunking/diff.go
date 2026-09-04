package chunking

import (
	"sort"
	"strings"
)

// ChunkDiffReport 是旧 ParsedDocument 路径与 Canonical 候选路径的确定性影子比较。
// BoundaryDifferenceRate/SourceDifferenceRate 取值均为 [0,1]，0 表示完全一致。
type ChunkDiffReport struct {
	LegacyChunkCount       int     `json:"legacy_chunk_count"`
	CandidateChunkCount    int     `json:"candidate_chunk_count"`
	ExactContentMatches    int     `json:"exact_content_matches"`
	BoundaryDifferenceRate float64 `json:"boundary_difference_rate"`
	SourceDifferenceRate   float64 `json:"source_difference_rate"`
	CandidateStrategy      string  `json:"candidate_strategy"`
	CandidateVersion       string  `json:"candidate_version"`
}

// CompareChunks 生成不影响生产 Chunk 的影子差异报告。
// 边界率比较按顺序累计的 Chunk 内容 byte 边界；来源率比较对齐位置的对象引用集合。
func CompareChunks(legacy, candidate []ParsedChunk, strategy, version string) ChunkDiffReport {
	report := ChunkDiffReport{
		LegacyChunkCount:    len(legacy),
		CandidateChunkCount: len(candidate),
		CandidateStrategy:   strategy,
		CandidateVersion:    version,
	}
	paired := diffMin(len(legacy), len(candidate))
	for i := 0; i < paired; i++ {
		if strings.TrimSpace(legacy[i].Content) == strings.TrimSpace(candidate[i].Content) {
			report.ExactContentMatches++
		}
	}
	report.BoundaryDifferenceRate = setDifferenceRate(chunkBoundaries(legacy), chunkBoundaries(candidate))
	report.SourceDifferenceRate = sourceDifferenceRate(legacy, candidate)
	return report
}

func chunkBoundaries(chunks []ParsedChunk) map[int]bool {
	out := make(map[int]bool)
	offset := 0
	for i, chunk := range chunks {
		offset += len(strings.TrimSpace(chunk.Content))
		if i < len(chunks)-1 {
			out[offset] = true
		}
	}
	return out
}

func setDifferenceRate(a, b map[int]bool) float64 {
	union, same := make(map[int]bool, len(a)+len(b)), 0
	for value := range a {
		union[value] = true
	}
	for value := range b {
		union[value] = true
	}
	for value := range union {
		if a[value] && b[value] {
			same++
		}
	}
	if len(union) == 0 {
		if len(a) == len(b) {
			return 0
		}
		return 1
	}
	return 1 - float64(same)/float64(len(union))
}

func sourceDifferenceRate(legacy, candidate []ParsedChunk) float64 {
	total := diffMax(len(legacy), len(candidate))
	if total == 0 {
		return 0
	}
	different := total - diffMin(len(legacy), len(candidate))
	for i := 0; i < diffMin(len(legacy), len(candidate)); i++ {
		if sourceKey(legacy[i]) != sourceKey(candidate[i]) {
			different++
		}
	}
	return float64(different) / float64(total)
}

func sourceKey(chunk ParsedChunk) string {
	values := make([]string, 0, len(chunk.BlockIDs)+len(chunk.TableRefs)+len(chunk.AssetRefs))
	for _, value := range chunk.BlockIDs {
		values = append(values, "b:"+value)
	}
	for _, value := range chunk.TableRefs {
		values = append(values, "t:"+value)
	}
	for _, value := range chunk.AssetRefs {
		values = append(values, "a:"+value)
	}
	sort.Strings(values)
	return strings.Join(values, "|")
}

func diffMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func diffMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
