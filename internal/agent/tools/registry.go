package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/cloudwego/eino/schema"
)

// Registry 是进程内显式构造的工具注册表，保证工具发现不依赖全局状态。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建空工具注册表。
func NewRegistry() *Registry { return &Registry{tools: make(map[string]Tool)} }

// Register 注册一个名称唯一的工具。
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

// Has 检查工具是否已注册。
func (r *Registry) Has(name string) bool { _, ok := r.find(name); return ok }

// Names 返回稳定排序的工具名称快照。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Specs 返回稳定排序的工具规格快照。
func (r *Registry) Specs() []contracts.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]contracts.ToolSpec, 0, len(r.tools))
	for _, value := range r.tools {
		specs = append(specs, value.Spec())
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// EinoTools 返回白名单工具的模型可见信息。
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
		if item == nil {
			return nil, fmt.Errorf("tool %q info is nil", name)
		}
		result = append(result, *item)
	}
	return result, nil
}

func (r *Registry) find(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
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
