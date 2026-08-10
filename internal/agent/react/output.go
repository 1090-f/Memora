package react

import (
	"sort"
	"strings"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
)

type citationCollector struct {
	mu    sync.Mutex
	items []contracts.Citation
	seen  map[string]struct{}
}

func newCitationCollector() *citationCollector {
	return &citationCollector{seen: make(map[string]struct{})}
}

func (c *citationCollector) Add(items []contracts.Citation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		key := strings.Join([]string{string(item.SourceType), string(item.DocumentID), string(item.ChunkID), item.URL}, "|")
		if _, exists := c.seen[key]; exists {
			continue
		}
		c.seen[key] = struct{}{}
		c.items = append(c.items, item)
	}
}

func (c *citationCollector) Get() []contracts.Citation {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := append([]contracts.Citation(nil), c.items...)
	sort.SliceStable(result, func(i, j int) bool {
		return string(result[i].DocumentID)+string(result[i].ChunkID)+result[i].URL < string(result[j].DocumentID)+string(result[j].ChunkID)+result[j].URL
	})
	return result
}

func (c *citationCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
	c.seen = make(map[string]struct{})
}

func normalizeAnswer(value string) string { return strings.TrimSpace(value) }

func mergeUsage(total, current contracts.TokenUsage) contracts.TokenUsage {
	total.InputTokens += current.InputTokens
	total.OutputTokens += current.OutputTokens
	total.TotalTokens += current.TotalTokens
	return total
}
