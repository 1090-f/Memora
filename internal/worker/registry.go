package worker

import (
	"fmt"
	"sync"
)

// Registry 维护任务类型到处理器的映射关系，提供线程安全的注册与查询。
type Registry struct {
	mutex    sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry 创建一个新的任务处理器注册表。
func NewRegistry() *Registry { return &Registry{handlers: make(map[string]Handler)} }

// Register 注册一个任务类型及其对应的处理器。
func (r *Registry) Register(jobType string, handler Handler) error {
	if jobType == "" || handler == nil {
		return fmt.Errorf("job type and handler are required")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, exists := r.handlers[jobType]; exists {
		return fmt.Errorf("worker handler %q is already registered", jobType)
	}
	r.handlers[jobType] = handler
	return nil
}

// Handler 根据任务类型查找对应的处理器。
func (r *Registry) Handler(jobType string) (Handler, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	handler, exists := r.handlers[jobType]
	return handler, exists
}

// Count 返回已注册的处理器数量。
func (r *Registry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.handlers)
}
