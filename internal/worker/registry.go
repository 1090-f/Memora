package worker

import (
	"fmt"
	"sync"
)

type Registry struct {
	mutex    sync.RWMutex
	handlers map[string]Handler
}

func NewRegistry() *Registry { return &Registry{handlers: make(map[string]Handler)} }

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

func (r *Registry) Handler(jobType string) (Handler, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	handler, exists := r.handlers[jobType]
	return handler, exists
}

func (r *Registry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.handlers)
}
