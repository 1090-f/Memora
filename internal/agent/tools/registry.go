package tools

import (
	"errors"
	"sort"
	"sync"

	"github.com/1090-f/Memora/internal/contracts"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

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

func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

func (r *Registry) Get(name string) (contracts.ToolSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return contracts.ToolSpec{}, false
	}
	return tool.Spec(), true
}

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

func (r *Registry) Tool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}
