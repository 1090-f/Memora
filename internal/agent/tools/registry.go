package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

// RegisterOrUpdate 注册或更新工具。如果工具已存在则更新，不存在则注册。
// 用于动态刷新 MCP 工具列表时保证幂等性。
func (r *Registry) RegisterOrUpdate(value Tool) error {
	if value == nil {
		return fmt.Errorf("tool is nil")
	}
	spec := value.Spec()
	if spec.Name == "" {
		return fmt.Errorf("tool name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[spec.Name] = value
	return nil
}

// Unregister 移除指定名称的工具。用于清理已禁用的 MCP 工具。
func (r *Registry) Unregister(name string) {
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// UnregisterByType 移除指定类型的所有工具。用于批量清理某一类工具（如所有 MCP 工具）。
func (r *Registry) UnregisterByType(toolType contracts.ToolType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, tool := range r.tools {
		if tool.Spec().Type == toolType {
			delete(r.tools, name)
		}
	}
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
// Tools 返回当前注册表中的全部工具快照。
func (r *Registry) Tools() []Tool {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, 0, len(r.tools))
	for _, value := range r.tools {
		result = append(result, value)
	}
	return result
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
	value, ok := r.tools[name]
	if ok {
		r.mu.RUnlock()
		return value, ok
	}
	// 降级：尝试短名匹配（处理 LLM 省略 serverID:: 前缀的情况）
	// 仅当匹配到唯一结果时才返回，避免不同 Server 的同名工具冲突
	if idx := strings.LastIndex(name, "::"); idx >= 0 {
		shortName := name[idx+2:]
		var matched Tool
		matchCount := 0
		for fullName, tool := range r.tools {
			if strings.HasSuffix(fullName, "::"+shortName) || fullName == shortName {
				matched = tool
				matchCount++
			}
		}
		r.mu.RUnlock()
		if matchCount == 1 {
			return matched, true
		}
	}
	r.mu.RUnlock()
	return nil, false
}

func decodeResult(text string) (contracts.ToolResult, error) {
	var result contracts.ToolResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return contracts.ToolResult{}, err
	}
	return result, nil
}
