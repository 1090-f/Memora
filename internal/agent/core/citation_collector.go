package core

import (
	"encoding/json"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
)

// CitationCollector 收集并按内容去重工具和检索服务返回的引用。
type CitationCollector interface {
	Add(citations []contracts.Citation)
	Get() []contracts.Citation
	Reset()
}

type citationCollector struct {
	mu    sync.RWMutex
	items []contracts.Citation
	seen  map[string]struct{}
}

// NewCitationCollector 创建线程安全的引用收集器。
func NewCitationCollector() CitationCollector {
	return &citationCollector{seen: make(map[string]struct{})}
}

func (c *citationCollector) Add(items []contracts.Citation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		key, err := json.Marshal(item)
		if err != nil {
			continue
		}
		if _, exists := c.seen[string(key)]; exists {
			continue
		}
		c.seen[string(key)] = struct{}{}
		c.items = append(c.items, item)
	}
}

func (c *citationCollector) Get() []contracts.Citation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]contracts.Citation(nil), c.items...)
}

func (c *citationCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
	c.seen = make(map[string]struct{})
}
