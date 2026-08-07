package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/schema"
)

// Registry 是进程内显式构造的工具注册表。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: make(map[string]Tool)} }

func (r *Registry) Register(value Tool) error {
	if value == nil {
		return fmt.Errorf("tool is nil")
	}
	spec := value.Spec()
	if spec.Name == "" {
		return fmt.Errorf("tool name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[spec.Name]; exists {
		return fmt.Errorf("tool %q already registered", spec.Name)
	}
	r.tools[spec.Name] = value
	return nil
}

func (r *Registry) Has(name string) bool { _, ok := r.find(name); return ok }

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *Registry) Specs() []contracts.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]contracts.ToolSpec, 0, len(r.tools))
	for _, value := range r.tools {
		specs = append(specs, value.Spec())
	}
	return specs
}

func (r *Registry) EinoTools(ctx context.Context, allowed []string) ([]schema.ToolInfo, error) {
	result := make([]schema.ToolInfo, 0, len(allowed))
	for _, name := range allowed {
		value, ok := r.find(name)
		if !ok {
			return nil, fmt.Errorf("tool %q is not registered", name)
		}
		item, err := value.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("tool %q info: %w", name, err)
		}
		result = append(result, *item)
	}
	return result, nil
}

func (r *Registry) find(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.tools[name]
	return value, ok
}

func decodeResult(text string) (contracts.ToolResult, error) {
	var result contracts.ToolResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return contracts.ToolResult{}, err
	}
	return result, nil
}
