package tools

import (
	"errors"
	"sort"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
)

// Registry 是工具的注册表，集中管理所有可用的工具。
// 通过互斥锁保证并发安全，供 Executor 按名称查找工具。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建并返回一个空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register 将工具注册进注册表。
// 工具名为空会报错；同名工具重复注册也会报错。
func (r *Registry) Register(tool Tool) error {
	spec := tool.Spec()
	if spec.Name == "" {
		return errors.New("tool name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[spec.Name]; ok {
		return errors.New("tool already registered")
	}
	r.tools[spec.Name] = tool
	return nil
}

// Has 判断指定名称的工具是否已注册。
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// Get 返回指定名称工具的规格描述；工具不存在时返回第二个参数 false。
func (r *Registry) Get(name string) (contracts.ToolSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return contracts.ToolSpec{}, false
	}
	return tool.Spec(), true
}

// Names 返回全部已注册工具的名称，按字典序排序。
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

// Specs 返回全部已注册工具的规格描述，按名称排序。
func (r *Registry) Specs() []contracts.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]contracts.ToolSpec, 0, len(r.tools))
	for _, tool := range r.tools {
		specs = append(specs, tool.Spec())
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// Tool 返回指定名称的工具实例及其是否存在。
func (r *Registry) Tool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}
